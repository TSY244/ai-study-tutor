package material

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
)

// DTO 是素材的 API 表示（契约 Material；source_url/original_name
// 按 kind 可空：URL 素材输出 source_url，上传输出 original_name，
// text 两者均为 null）。
type DTO struct {
	ID           uuid.UUID `json:"id"`
	LessonID     uuid.UUID `json:"lesson_id"`
	Kind         string    `json:"kind"`
	Status       string    `json:"status"`
	SourceURL    *string   `json:"source_url"`
	OriginalName *string   `json:"original_name"`
	CreatedAt    string    `json:"created_at"`
}

// Accepted 是三种创建端点的 202 响应体（契约 MaterialJobAccepted）。
type Accepted struct {
	Material DTO     `json:"material"`
	Job      job.DTO `json:"job"`
}

// TextInput 是创建文本素材的输入。
type TextInput struct {
	LessonID uuid.UUID
	Title    *string
	Text     string
}

// URLInput 是创建 URL 素材的输入（URL 尚未规范化/SSRF 校验）。
type URLInput struct {
	LessonID          uuid.UUID
	URL               string
	PreferredLanguage *string
}

// UploadInput 是创建上传素材的输入。StagedPath 必须位于数据目录 tmp/ 内，
// 由 filestore.Publish 两阶段发布接管（rename 后不再是调用方的文件）。
type UploadInput struct {
	LessonID          uuid.UUID
	StagedPath        string
	OriginalName      string // 已 filepath.Base 清洗，仅用于展示
	MediaType         string // 嗅探结果（http.DetectContentType）
	PreferredLanguage string
	TranscriptionMode string
}

// ListInput 是素材游标分页的输入。
type ListInput struct {
	LessonID       uuid.UUID
	AfterCreatedAt *time.Time
	AfterID        *uuid.UUID
	Limit          int
}

// Service 拥有素材用例的事务边界。pool 用于业务事务；files 用于上传的
// 两阶段发布；urlAllowlist 是 URL 素材的 SSRF 主机白名单。
type Service struct {
	pool         *pgxpool.Pool
	files        *filestore.Store
	urlAllowlist []string
	repo         Repository
	lessons      lesson.Repository
	jobs         *job.Repository

	// HostResolver 解析素材 URL 主机（SSRF 校验用）；nil 时使用系统 DNS。
	// 测试注入桩以避免外网依赖。
	HostResolver func(ctx context.Context, host string) ([]net.IP, error)
	// SkipURLDNSValidation lets an API deployment without egress defer DNS and
	// redirect-chain validation to the Worker. Scheme and host allowlist checks
	// still run here, and the Worker must always revalidate before fetching.
	SkipURLDNSValidation bool
}

// NewService 创建素材 Service。
func NewService(pool *pgxpool.Pool, files *filestore.Store, urlAllowlist []string) *Service {
	return &Service{pool: pool, files: files, urlAllowlist: urlAllowlist, jobs: &job.Repository{}}
}

// mapLessonErr 把课时读取错误转换为 404 ProblemError（httpx 不能反向依赖
// lesson 包，映射在各模块 service 完成）。
func mapLessonErr(err error) error {
	if errors.Is(err, lesson.ErrNotFound) {
		return httpx.NewProblemError(http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
	}
	return err
}

// activeJobConflictErr 把 lesson_generation 族唯一冲突转换为带
// active_job_id 扩展的 409 resource_busy。原事务已因唯一冲突被
// PostgreSQL 中止，必须经连接池重新查询；查不到活动任务时回退为原始
// 错误（httpx.WriteError 映射为无扩展的 409 resource_busy）。
func (s *Service) activeJobConflictErr(ctx context.Context, lessonID uuid.UUID, err error) error {
	if !errors.Is(err, job.ErrActiveGenerationJobExists) {
		return err
	}
	active, gerr := s.jobs.GetActiveGenerationJob(ctx, s.pool, lessonID)
	if gerr == nil && active != nil {
		return job.ResourceBusyError(active.ID)
	}
	return err
}

// toDTO 映射领域素材到 API 表示。
func (m *Material) toDTO() DTO {
	return DTO{
		ID:           m.ID,
		LessonID:     m.LessonID,
		Kind:         m.Kind,
		Status:       m.Status,
		SourceURL:    m.SourceURL,
		OriginalName: m.OriginalName,
		CreatedAt:    httpx.Time(m.CreatedAt),
	}
}

// CreateTextMaterial 在单事务内创建文本素材与 lesson_import Job，
// 并完成幂等记录（fn 语义见 httpx.IdemExecutor）。
func (s *Service) CreateTextMaterial(ctx context.Context, idemSvc *idempotency.Service,
	rec *idempotency.Record, in TextInput) (*Accepted, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return nil, err
	}
	les, err := s.lessons.GetByID(ctx, tx, in.LessonID)
	if err != nil {
		return nil, mapLessonErr(err)
	}
	metadata := map[string]any{}
	if in.Title != nil {
		metadata["title"] = *in.Title
	}
	m := &Material{
		LessonID:   in.LessonID,
		Kind:       KindText,
		InlineText: &in.Text,
		Metadata:   mustJSON(metadata),
	}
	if err := s.repo.Create(ctx, tx, m); err != nil {
		return nil, err
	}
	payload := basePayload(m)
	if in.Title != nil {
		payload["title"] = *in.Title
	}
	body, err := s.finishCreation(ctx, tx, idemSvc, rec, les, m, payload)
	if err != nil {
		return nil, s.activeJobConflictErr(ctx, in.LessonID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return body, nil
}

// CreateURLMaterial 规范化并 SSRF 校验 URL（不下载内容），然后在单事务内
// 创建 URL 素材与 lesson_import Job，并完成幂等记录。
func (s *Service) CreateURLMaterial(ctx context.Context, idemSvc *idempotency.Service,
	rec *idempotency.Record, in URLInput) (*Accepted, error) {
	normalized, perr := s.validateSourceURL(ctx, in.URL)
	if perr != nil {
		return nil, perr
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return nil, err
	}
	les, err := s.lessons.GetByID(ctx, tx, in.LessonID)
	if err != nil {
		return nil, mapLessonErr(err)
	}
	metadata := map[string]any{}
	if in.PreferredLanguage != nil {
		metadata["preferred_language"] = *in.PreferredLanguage
	}
	m := &Material{
		LessonID:  in.LessonID,
		Kind:      KindURL,
		SourceURL: &normalized,
		Metadata:  mustJSON(metadata),
	}
	if err := s.repo.Create(ctx, tx, m); err != nil {
		return nil, err
	}
	payload := basePayload(m)
	payload["source_url"] = normalized
	if in.PreferredLanguage != nil {
		payload["preferred_language"] = *in.PreferredLanguage
	}
	body, err := s.finishCreation(ctx, tx, idemSvc, rec, les, m, payload)
	if err != nil {
		return nil, s.activeJobConflictErr(ctx, in.LessonID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return body, nil
}

// UploadMaterial 通过 filestore.Publish 两阶段发布已暂存的上传文件，
// 并在发布的第二个事务（tx2，LinkFunc）内完成业务关联：AssertReady、
// 插素材、建 Job、推进课时状态、完成幂等记录。
//
// 发布前的预检（课时存在/维护模式/活动任务互斥）只用于避免常见错误
// 路径留下 temporary 文件垃圾；权威校验仍在 tx2 内执行。
func (s *Service) UploadMaterial(ctx context.Context, idemSvc *idempotency.Service,
	rec *idempotency.Record, in UploadInput) (*Accepted, error) {
	if err := s.preflightWritableLesson(ctx, in.LessonID); err != nil {
		return nil, err
	}
	var accepted *Accepted
	_, err := s.files.Publish(ctx, filestore.PublishInput{
		OwnerType:    filestore.OwnerTypeIdempotency,
		OwnerID:      rec.ID,
		StagedPath:   in.StagedPath,
		OriginalName: in.OriginalName,
		MediaType:    in.MediaType,
	}, func(ctx context.Context, tx pgx.Tx, objectID uuid.UUID) error {
		if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
			return err
		}
		les, err := s.lessons.GetByID(ctx, tx, in.LessonID)
		if err != nil {
			return mapLessonErr(err)
		}
		if err := s.files.AssertReady(ctx, tx, objectID); err != nil {
			return err
		}
		metadata := map[string]any{}
		if in.PreferredLanguage != "" {
			metadata["preferred_language"] = in.PreferredLanguage
		}
		if in.TranscriptionMode != "" {
			metadata["transcription_mode"] = in.TranscriptionMode
		}
		m := &Material{
			LessonID:     in.LessonID,
			Kind:         KindUpload,
			FileObjectID: &objectID,
			Metadata:     mustJSON(metadata),
			OriginalName: &in.OriginalName,
		}
		if err := s.repo.Create(ctx, tx, m); err != nil {
			return err
		}
		payload := basePayload(m)
		payload["file_object_id"] = objectID.String()
		payload["original_name"] = in.OriginalName
		payload["media_type"] = in.MediaType
		if in.PreferredLanguage != "" {
			payload["preferred_language"] = in.PreferredLanguage
		}
		if in.TranscriptionMode != "" {
			payload["transcription_mode"] = in.TranscriptionMode
		}
		body, err := s.finishCreation(ctx, tx, idemSvc, rec, les, m, payload)
		if err != nil {
			return err
		}
		accepted = body
		return nil
	})
	if err != nil {
		return nil, s.activeJobConflictErr(ctx, in.LessonID, err)
	}
	return accepted, nil
}

// ListMaterials 游标分页查询课时素材（created_at ASC, id ASC）。
func (s *Service) ListMaterials(ctx context.Context, in ListInput) (*httpx.Collection[DTO], error) {
	if _, err := s.lessons.GetByID(ctx, s.pool, in.LessonID); err != nil {
		return nil, mapLessonErr(err)
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.repo.ListByLesson(ctx, s.pool, in.LessonID, in.AfterCreatedAt, in.AfterID, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]DTO, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].toDTO())
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := httpx.EncodeCursor(map[string]any{
			"created_at": last.CreatedAt.UTC().Format(time.RFC3339Nano),
			"id":         last.ID.String(),
		})
		next = &cursor
	}
	return &httpx.Collection[DTO]{
		Items:    items,
		PageInfo: httpx.PageInfo{NextCursor: next, HasMore: hasMore},
	}, nil
}

// DeleteMaterial 删除素材：有活动 Job 引用时返回 409 resource_busy；
// 上传素材在同一事务内把文件对象标记为 deleted。
func (s *Service) DeleteMaterial(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return err
	}
	m, err := s.repo.GetByID(ctx, tx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return err
	}
	busy, err := s.repo.HasActiveJobReference(ctx, tx, id)
	if err != nil {
		return err
	}
	if busy {
		return httpx.NewProblemError(http.StatusConflict, "resource_busy",
			"资源繁忙", "素材正被任务使用，不能删除。")
	}
	if err := s.repo.Delete(ctx, tx, id); err != nil {
		return err
	}
	if m.Kind == KindUpload && m.FileObjectID != nil {
		if err := s.repo.MarkFileObjectDeleted(ctx, tx, *m.FileObjectID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// PrepareStaging 为一次上传创建暂存文件路径
// （dataDir/tmp/_staging/{uuid}/payload，满足 filestore.Publish 对
// StagedPath 必须位于数据目录 tmp/ 内的要求）。调用方负责在流程结束后
// 清理整个暂存目录。
func (s *Service) PrepareStaging() (dir, path string, err error) {
	dir = filepath.Join(s.files.DataDir(), "tmp", "_staging", uuid.NewString())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("material: 创建暂存目录失败: %w", err)
	}
	return dir, filepath.Join(dir, "payload"), nil
}

// preflightWritableLesson 是上传发布前的非权威预检：课时存在、非维护
// 模式、无活动 lesson_generation 任务（409 resource_busy）。
func (s *Service) preflightWritableLesson(ctx context.Context, lessonID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return err
	}
	if _, err := s.lessons.GetByID(ctx, tx, lessonID); err != nil {
		return mapLessonErr(err)
	}
	active, err := s.jobs.GetActiveGenerationJob(ctx, tx, lessonID)
	if err != nil {
		return err
	}
	if active != nil {
		return job.ResourceBusyError(active.ID)
	}
	return nil
}

// finishCreation 在调用方事务内完成创建公共段：建 lesson_import Job
// （族唯一冲突 → 409 resource_busy + active_job_id 扩展）、把课时状态
// 推进为 processing、构造 {material, job} 响应并完成幂等记录。
func (s *Service) finishCreation(ctx context.Context, tx pgx.Tx, idemSvc *idempotency.Service,
	rec *idempotency.Record, les *lesson.Lesson, m *Material, payload map[string]any) (*Accepted, error) {
	family := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindLessonImport,
		AggregateType:    "lesson",
		AggregateID:      les.ID,
		GenerationFamily: &family,
		Payload:          mustJSON(payload),
	}
	if err := s.jobs.Create(ctx, tx, j); err != nil {
		// 唯一冲突后的活动任务查询由调用方在中止事务之外完成
		// （activeJobConflictErr）。
		return nil, err
	}
	if les.Status != "processing" {
		if err := s.lessons.UpdateStatus(ctx, tx, les.ID, "processing", les.Version); err != nil {
			return nil, err
		}
	}
	body := &Accepted{Material: m.toDTO(), Job: job.ToDTO(j, nil)}
	resourceType := "material"
	if err := httpx.CompleteJSON(ctx, tx, idemSvc, rec.ID, http.StatusAccepted,
		body, map[string]string{"Location": "/api/v1/jobs/" + j.ID.String()},
		&resourceType, &m.ID, &j.ID); err != nil {
		return nil, err
	}
	return body, nil
}

// basePayload 构造 Job payload 的公共字段。
func basePayload(m *Material) map[string]any {
	return map[string]any{
		"material_id":   m.ID.String(),
		"lesson_id":     m.LessonID.String(),
		"material_kind": m.Kind,
	}
}

// mustJSON 序列化小对象（键均为字符串、值为标量，不会失败）。
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}
