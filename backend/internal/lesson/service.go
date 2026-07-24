package lesson

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// DocumentsDTO 对齐契约 DocumentAvailability Schema。
type DocumentsDTO struct {
	RawText        bool `json:"raw_text"`
	StudentNotes   bool `json:"student_notes"`
	TeacherOutline bool `json:"teacher_outline"`
}

// LessonDTO 对齐契约 Lesson Schema（additionalProperties:false）。
type LessonDTO struct {
	ID          string       `json:"id"`
	CourseID    string       `json:"course_id"`
	Ordinal     int32        `json:"ordinal"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	ActiveJobID *string      `json:"active_job_id"`
	Documents   DocumentsDTO `json:"documents"`
	Mastery     *float64     `json:"mastery"`
	Version     int64        `json:"version"`
	CreatedAt   string       `json:"created_at"`
	UpdatedAt   string       `json:"updated_at"`
}

// Service 是课时业务编排：拥有事务边界（pool.Begin），写事务入口
// 先过 gate.AssertBusinessOpen。
type Service struct {
	pool        *pgxpool.Pool
	repo        Repository
	courses     course.Repository
	jobs        *job.Repository
	masteryRepo mastery.Repository
}

// NewService 构造课时 Service。
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, jobs: &job.Repository{}}
}

// mapReadErr 把读取路径的哨兵错误转换为 ProblemError（404）。模块哨兵
// 必须在 service 层转换：幂等执行器内的错误统一走 httpx.WriteError，
// 而 httpx 不能反向依赖本包。
func mapReadErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, course.ErrNotFound):
		return httpx.NewProblemError(http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
	default:
		return err
	}
}

// mapWriteErr 在 mapReadErr 之上追加乐观锁与 ordinal 唯一冲突的转换。
func mapWriteErr(err error) error {
	switch {
	case errors.Is(err, ErrVersionConflict):
		return httpx.NewProblemError(http.StatusConflict, "version_conflict",
			"版本冲突", "资源已被修改，请刷新后重试。")
	case errors.Is(err, ErrDuplicateOrdinal):
		p := httpserver.NewProblem(http.StatusUnprocessableEntity, "validation_error",
			"请求参数无效", "请修正标记的字段后重试。")
		p.Errors = []httpserver.FieldError{
			{Field: "ordinal", Code: "duplicate", Message: "课程内已存在相同序号的课时。"},
		}
		return &httpx.ProblemError{Problem: p}
	default:
		return mapReadErr(err)
	}
}

// CreateInput 是创建课时的入参（字段校验已在 handler 完成）。
type CreateInput struct {
	Ordinal     int32
	Title       string
	Description *string
}

// UpdateInput 是 PATCH 入参：指针字段为 nil 表示不更新；
// Version 为客户端持有的当前版本（乐观锁，必填）。
type UpdateInput struct {
	Ordinal     *int32
	Title       *string
	Description *string
	Version     int64
}

// baseDTO 映射基础字段；汇总字段（active_job_id/documents/mastery）
// 由 enrich 填充。新建课时无任何子数据，直接用零值汇总。
func baseDTO(l Lesson) LessonDTO {
	desc := ""
	if l.Description != nil {
		desc = *l.Description
	}
	return LessonDTO{
		ID:          l.ID.String(),
		CourseID:    l.CourseID.String(),
		Ordinal:     l.Ordinal,
		Title:       l.Title,
		Description: desc,
		Status:      l.Status,
		Version:     l.Version,
		CreatedAt:   httpx.Time(l.CreatedAt),
		UpdatedAt:   httpx.Time(l.UpdatedAt),
	}
}

// enrich 填充 DTO 的三个汇总字段：活动生成任务、active 文档可用性、
// 最新掌握度（value 0-100 → 0..1，无快照为 null）。
func (s *Service) enrich(ctx context.Context, q database.Querier, dto *LessonDTO,
	l Lesson, avail DocumentAvailability) error {
	dto.Documents = DocumentsDTO{
		RawText:        avail.RawText,
		StudentNotes:   avail.StudentNotes,
		TeacherOutline: avail.TeacherOutline,
	}
	j, err := s.jobs.GetActiveGenerationJob(ctx, q, l.ID)
	if err != nil {
		return err
	}
	if j != nil {
		id := j.ID.String()
		dto.ActiveJobID = &id
	}
	snap, err := s.masteryRepo.LatestSnapshot(ctx, q, "lesson", l.CourseID, &l.ID)
	switch {
	case errors.Is(err, mastery.ErrNotFound):
		// 无快照：mastery 保持 null
	case err != nil:
		return err
	default:
		v := snap.Value / 100
		dto.Mastery = &v
	}
	return nil
}

// List 按 ordinal ASC, id ASC 游标分页；课程不存在返回 course.ErrNotFound。
func (s *Service) List(ctx context.Context, courseID uuid.UUID,
	afterOrdinal *int32, afterID *uuid.UUID, limit int) ([]LessonDTO, bool, error) {
	if _, err := s.courses.GetByID(ctx, s.pool, courseID); err != nil {
		return nil, false, mapReadErr(err)
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.repo.ListByCourse(ctx, s.pool, courseID, afterOrdinal, afterID, limit+1)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, l := range rows {
		ids = append(ids, l.ID)
	}
	avail, err := s.repo.DocumentAvailabilityByLesson(ctx, s.pool, ids)
	if err != nil {
		return nil, false, err
	}
	out := make([]LessonDTO, 0, len(rows))
	for _, l := range rows {
		dto := baseDTO(l)
		if err := s.enrich(ctx, s.pool, &dto, l, avail[l.ID]); err != nil {
			return nil, false, err
		}
		out = append(out, dto)
	}
	return out, hasMore, nil
}

// Create 在事务中创建课时；complete 非 nil 时在提交前于同一事务调用
// （幂等完成记录与业务写入同事务提交）。课程不存在返回
// course.ErrNotFound；ordinal 冲突返回 ErrDuplicateOrdinal。
func (s *Service) Create(ctx context.Context, courseID uuid.UUID, in CreateInput,
	complete func(ctx context.Context, tx database.Querier, dto LessonDTO) error) (LessonDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return LessonDTO{}, err
	}
	if _, err := s.courses.GetByID(ctx, tx, courseID); err != nil {
		return LessonDTO{}, mapReadErr(err)
	}
	l := Lesson{CourseID: courseID, Ordinal: in.Ordinal, Title: in.Title, Description: in.Description}
	if err := s.repo.Create(ctx, tx, &l); err != nil {
		return LessonDTO{}, mapWriteErr(err)
	}
	dto := baseDTO(l)
	if complete != nil {
		if err := complete(ctx, tx, dto); err != nil {
			return LessonDTO{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return LessonDTO{}, err
	}
	return dto, nil
}

// Get 读取课时详情（含汇总字段）；不存在返回 404 ProblemError。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (LessonDTO, error) {
	l, err := s.repo.GetByID(ctx, s.pool, id)
	if err != nil {
		return LessonDTO{}, mapReadErr(err)
	}
	avail, err := s.repo.DocumentAvailabilityByLesson(ctx, s.pool, []uuid.UUID{id})
	if err != nil {
		return LessonDTO{}, err
	}
	dto := baseDTO(*l)
	if err := s.enrich(ctx, s.pool, &dto, *l, avail[id]); err != nil {
		return LessonDTO{}, err
	}
	return dto, nil
}

// Update 乐观锁更新：版本不匹配返回 409 version_conflict；
// ordinal 唯一冲突返回 422 validation_error；允许不带任何可更新字段。
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (LessonDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LessonDTO{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return LessonDTO{}, err
	}
	l, err := s.repo.GetByID(ctx, tx, id)
	if err != nil {
		return LessonDTO{}, mapReadErr(err)
	}
	if in.Ordinal != nil {
		l.Ordinal = *in.Ordinal
	}
	if in.Title != nil {
		l.Title = *in.Title
	}
	if in.Description != nil {
		l.Description = in.Description
	}
	l.Version = in.Version
	if err := s.repo.Update(ctx, tx, l); err != nil {
		return LessonDTO{}, mapWriteErr(err)
	}
	avail, err := s.repo.DocumentAvailabilityByLesson(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return LessonDTO{}, err
	}
	dto := baseDTO(*l)
	if err := s.enrich(ctx, tx, &dto, *l, avail[id]); err != nil {
		return LessonDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LessonDTO{}, err
	}
	return dto, nil
}

// Delete 物理删除课时及其子数据。有活动 lesson_generation 任务或
// 被任何试卷引用（保护作答历史）时拒绝（409 resource_busy）。
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return err
	}
	if _, err := s.repo.GetByID(ctx, tx, id); err != nil {
		return mapReadErr(err)
	}
	j, err := s.jobs.GetActiveGenerationJob(ctx, tx, id)
	if err != nil {
		return err
	}
	if j != nil {
		return job.ResourceBusyError(j.ID)
	}
	referenced, err := s.repo.HasQuizReference(ctx, tx, id)
	if err != nil {
		return err
	}
	if referenced {
		return httpx.NewProblemError(409, "resource_busy", "资源繁忙", "课时已被试卷引用，不能删除。")
	}
	if err := s.repo.DeleteCascade(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
