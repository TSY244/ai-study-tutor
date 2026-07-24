// service.go 是薄弱点与进度查询的应用服务；事务边界在本层。
package mastery

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
)

// CourseExister 是课程存在性校验端口（course.Repository 实现）。
type CourseExister interface {
	Exists(ctx context.Context, q database.Querier, id uuid.UUID) (bool, error)
}

// WeaknessService 提供薄弱点查询与生命周期操作。
type WeaknessService struct {
	pool   *pgxpool.Pool
	repo   Repository
	scores ScoreReader
	idem   *idempotency.Service
}

func NewWeaknessService(pool *pgxpool.Pool, scores ScoreReader) *WeaknessService {
	return &WeaknessService{
		pool:   pool,
		repo:   Repository{},
		scores: scores,
		idem:   idempotency.NewService(pool),
	}
}

// List 按 first_seen_at ASC, id ASC 游标分页返回薄弱点（可过滤）。
func (s *WeaknessService) List(ctx context.Context, f WeaknessFilter, limit int) ([]Weakness, error) {
	return s.repo.ListWeaknesses(ctx, s.pool, f, limit)
}

// Get 读取薄弱点详情；不存在返回 404。
func (s *WeaknessService) Get(ctx context.Context, id uuid.UUID) (*Weakness, error) {
	w, err := s.repo.GetWeaknessByID(ctx, s.pool, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return nil, err
	}
	return w, nil
}

// SetMastered 是 resolve/reopen 的共用事务：乐观锁更新 → 追加课程（与课时，
// 若有）范围掌握度快照（人工调整薄弱点后追加快照，03 §6.3）→ 幂等完成。
// mastered=true 对应 resolve（severity 归 1），false 对应 reopen。
// 返回的 DTO 与幂等记录中的响应体一致（新鲜响应与回放同形）。
func (s *WeaknessService) SetMastered(ctx context.Context, rec *idempotency.Record,
	id uuid.UUID, expectedVersion int64, mastered bool) (int, *WeaknessDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	var w *Weakness
	if mastered {
		w, err = s.repo.ResolveWeakness(ctx, tx, id, expectedVersion)
	} else {
		w, err = s.repo.ReopenWeakness(ctx, tx, id, expectedVersion)
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return 0, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		case errors.Is(err, ErrVersionConflict):
			return 0, nil, s.weaknessVersionConflict(ctx, id)
		default:
			return 0, nil, err
		}
	}
	// 追加快照（source_attempt_id 为空，人工调整不参与重复消费去重）。
	apply := &ApplyService{repo: s.repo, scores: s.scores}
	if w.LessonID != nil {
		if err := apply.appendScopeSnapshot(ctx, tx, "lesson", w.CourseID, w.LessonID, nil); err != nil {
			return 0, nil, err
		}
	}
	if err := apply.appendScopeSnapshot(ctx, tx, "course", w.CourseID, nil, nil); err != nil {
		return 0, nil, err
	}
	dto := ToWeaknessDTO(w)
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusOK, dto,
		nil, nil, nil, nil); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return http.StatusOK, &dto, nil
}

// weaknessVersionConflict 构造带 current_version 扩展的 409 version_conflict。
func (s *WeaknessService) weaknessVersionConflict(ctx context.Context, id uuid.UUID) error {
	p := httpx.NewProblemError(http.StatusConflict, "version_conflict",
		"薄弱点版本冲突", "服务端已有更新的薄弱点状态，请刷新后重试。")
	if w, err := s.repo.GetWeaknessByID(ctx, s.pool, id); err == nil {
		p.Problem.Extensions = map[string]any{"current_version": w.Version}
	}
	return p
}

// ProgressService 提供课程/课时进度聚合。
type ProgressService struct {
	pool    *pgxpool.Pool
	repo    Repository
	scores  ScoreReader
	courses CourseExister
	lessons LessonCourseReader
}

func NewProgressService(pool *pgxpool.Pool, scores ScoreReader, courses CourseExister, lessons LessonCourseReader) *ProgressService {
	return &ProgressService{
		pool:    pool,
		repo:    Repository{},
		scores:  scores,
		courses: courses,
		lessons: lessons,
	}
}

// GetProgress 聚合指定范围的进度：当前掌握度（最新快照，0-100 → 0..1）、
// 已批阅作答数、未掌握薄弱点数与趋势点（追加式快照，前端不重算公式）。
func (s *ProgressService) GetProgress(ctx context.Context, scopeType string, scopeID uuid.UUID) (*ProgressDTO, error) {
	var courseID uuid.UUID
	var lessonID *uuid.UUID
	if scopeType == "lesson" {
		cid, found, err := s.lessons.GetCourseID(ctx, s.pool, scopeID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		courseID = cid
		lessonID = &scopeID
	} else {
		exists, err := s.courses.Exists(ctx, s.pool, scopeID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		courseID = scopeID
	}
	completed, err := s.scores.CountGraded(ctx, s.pool, courseID, lessonID)
	if err != nil {
		return nil, err
	}
	active, err := s.repo.CountActive(ctx, s.pool, courseID, lessonID)
	if err != nil {
		return nil, err
	}
	dto := &ProgressDTO{
		ScopeType:         scopeType,
		ScopeID:           scopeID.String(),
		CompletedAttempts: int32(completed),
		ActiveWeaknesses:  int32(active),
		Trend:             []ProgressPointDTO{},
	}
	latest, err := s.repo.LatestSnapshot(ctx, s.pool, scopeType, courseID, lessonID)
	switch {
	case errors.Is(err, ErrNotFound):
		// 无快照：mastery/formula_version 保持 null
	case err != nil:
		return nil, err
	default:
		v := latest.Value / 100
		dto.Mastery = &v
		dto.FormulaVersion = &latest.FormulaVersion
	}
	trend, err := s.repo.ListSnapshots(ctx, s.pool, scopeType, courseID, lessonID, 50)
	if err != nil {
		return nil, err
	}
	for _, snap := range trend {
		dto.Trend = append(dto.Trend, ToProgressPointDTO(snap))
	}
	return dto, nil
}
