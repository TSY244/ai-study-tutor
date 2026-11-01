// Package material 实现课时素材（text/url/upload）的领域模型、Repository、
// 业务 Service 与 HTTP Handler（B3）。
//
// 架构边界：Handler 只做 HTTP 绑定与响应映射；事务边界在 Service
// （pool.Begin）；SQL 集中在 Repository（方法接受 database.Querier）。
// API 不下载 URL 内容、不调用 Claude、不执行长任务：三种创建均以
// 202 + lesson_import Job 异步落地，素材处理由 B5 Worker 推进。
package material

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
)

// 素材类型（materials.kind CHECK 约束）。
const (
	KindURL    = "url"
	KindUpload = "upload"
	KindText   = "text"
)

// 素材状态机：pending -> ready | failed（由 B5 导入任务推进）。
const (
	StatusPending = "pending"
	StatusReady   = "ready"
	StatusFailed  = "failed"
)

// ErrNotFound：素材不存在。Service 层映射为 404 not_found Problem。
var ErrNotFound = errors.New("素材不存在")

// Material 是 materials 表的领域表示。OriginalName 不是 materials 列：
// 由查询 LEFT JOIN file_objects 填充，或上传发布时由发布输入填充
// （仅用于 DTO 展示）。
type Material struct {
	Version      int64
	ID           uuid.UUID
	LessonID     uuid.UUID
	Kind         string
	SourceURL    *string
	FileObjectID *uuid.UUID
	InlineText   *string
	Metadata     []byte
	Status       string
	CreatedAt    time.Time
	OriginalName *string
}

// Repository 是素材聚合的无状态 Repository；所有方法接受
// database.Querier，由调用方决定独立执行还是组合进更大的事务边界。
type Repository struct{}

// selectSQL 关联 file_objects 取上传文件的展示名。
const selectSQL = `
	SELECT m.id, m.lesson_id, m.kind, m.source_url, m.file_object_id, m.inline_text,
	       m.metadata, m.status, m.created_at, f.original_name,m.version
	FROM materials m
	LEFT JOIN file_objects f ON f.id = m.file_object_id`

func scanMaterial(row pgx.Row) (*Material, error) {
	var m Material
	err := row.Scan(&m.ID, &m.LessonID, &m.Kind, &m.SourceURL, &m.FileObjectID,
		&m.InlineText, &m.Metadata, &m.Status, &m.CreatedAt, &m.OriginalName, &m.Version)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 插入素材（status 由数据库默认为 pending）。
func (Repository) Create(ctx context.Context, q database.Querier, m *Material) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Metadata == nil {
		m.Metadata = []byte(`{}`)
	}
	return q.QueryRow(ctx, `
		INSERT INTO materials (id, lesson_id, kind, source_url, file_object_id, inline_text, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING status, created_at,version`,
		m.ID, m.LessonID, m.Kind, m.SourceURL, m.FileObjectID, m.InlineText, m.Metadata).
		Scan(&m.Status, &m.CreatedAt, &m.Version)
}

// GetByID 读取素材（含上传文件展示名）。
func (Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Material, error) {
	m, err := scanMaterial(q.QueryRow(ctx, selectSQL+` WHERE m.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// ListByLesson 按 (created_at, id) 升序返回课时素材；after 为游标位置
// （返回严格位于其后的行），nil 表示从头开始。
func (Repository) ListByLesson(ctx context.Context, q database.Querier, lessonID uuid.UUID,
	afterCreatedAt *time.Time, afterID *uuid.UUID, limit int) ([]Material, error) {
	rows, err := q.Query(ctx, selectSQL+`
		WHERE m.lesson_id = $1
		  AND ($2::timestamptz IS NULL
		       OR m.created_at > $2
		       OR (m.created_at = $2 AND m.id > $3::uuid))
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT $4`, lessonID, afterCreatedAt, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Material
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.LessonID, &m.Kind, &m.SourceURL, &m.FileObjectID,
			&m.InlineText, &m.Metadata, &m.Status, &m.CreatedAt, &m.OriginalName, &m.Version); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// HasActiveJobReference 判定是否存在引用该素材的活动 Job
// （payload->>'material_id' 匹配且状态非终态）。
func (Repository) HasActiveJobReference(ctx context.Context, q database.Querier, id uuid.UUID) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM jobs
			WHERE payload->>'material_id' = $1
			  AND status IN ('queued','running','validating','cancel_requested'))`,
		id.String()).Scan(&exists)
	return exists, err
}

// SetResult 由 Worker 导入任务回写素材状态与元数据（与任务终态同事务）。
// 不存在返回 ErrNotFound。
func (Repository) SetResult(ctx context.Context, q database.Querier, id uuid.UUID, status string, metadata []byte) error {
	tag, err := q.Exec(ctx, `
		UPDATE materials SET status = $2, metadata = $3 WHERE id = $1`, id, status, metadata)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete 删除素材；不存在返回 ErrNotFound。
func (Repository) Delete(ctx context.Context, q database.Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM materials WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkFileObjectDeleted 把上传素材引用的文件对象标记为 deleted
// （与素材删除同事务；文件体由文件 Recovery 后续清理）。
func (Repository) MarkFileObjectDeleted(ctx context.Context, q database.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE file_objects SET state = 'deleted', updated_at = now()
		WHERE id = $1 AND state <> 'deleted'`, id)
	return err
}
