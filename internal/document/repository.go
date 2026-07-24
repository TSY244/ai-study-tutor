// Package document 提供文档版本的 Repository、查询 Service 与 HTTP Handler。
// 不变量：每个 lesson+kind 只有一个 is_active 版本（部分唯一索引
// uq_documents_active）；发布新版本时旧版本保留。
package document

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

var (
	// ErrNotFound：文档不存在。
	ErrNotFound = errors.New("文档不存在")
)

// Document 是 documents 表的领域表示。
type Document struct {
	ID          uuid.UUID
	LessonID    uuid.UUID
	Kind        string // raw_text/student_notes/teacher_outline
	Revision    int32
	ContentMD   string
	SourceJobID *uuid.UUID
	IsActive    bool
	CreatedAt   time.Time
}

type Repository struct{}

// CreateVersion 插入新文档版本：revision 自动取同 lesson+kind 的
// 最大值 +1（依赖 UNIQUE(lesson_id, kind, revision) 防并发冲突）。
func (Repository) CreateVersion(ctx context.Context, q database.Querier, d *Document) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	err := q.QueryRow(ctx, `
		INSERT INTO documents (id, lesson_id, kind, revision, content_md, source_job_id)
		SELECT $1, $2, $3, COALESCE(MAX(revision), 0) + 1, $4, $5
		FROM documents WHERE lesson_id = $2 AND kind = $3
		RETURNING revision, created_at`,
		d.ID, d.LessonID, d.Kind, d.ContentMD, d.SourceJobID).
		Scan(&d.Revision, &d.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

// Publish 把指定版本置为 active（同事务内停用旧版本）。
func (Repository) Publish(ctx context.Context, q database.Querier, id uuid.UUID) error {
	tag, err := q.Exec(ctx, `
		UPDATE documents SET is_active = false
		WHERE (lesson_id, kind) IN (SELECT lesson_id, kind FROM documents WHERE id = $1)
		  AND is_active`, id)
	if err != nil {
		return err
	}
	tag, err = q.Exec(ctx, `UPDATE documents SET is_active = true WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetActive 读取当前发布版本；不存在返回 ErrNotFound。
func (Repository) GetActive(ctx context.Context, q database.Querier, lessonID uuid.UUID, kind string) (*Document, error) {
	var d Document
	err := q.QueryRow(ctx, `
		SELECT id, lesson_id, kind, revision, content_md, source_job_id, is_active, created_at
		FROM documents WHERE lesson_id = $1 AND kind = $2 AND is_active`, lessonID, kind).Scan(
		&d.ID, &d.LessonID, &d.Kind, &d.Revision, &d.ContentMD, &d.SourceJobID, &d.IsActive, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

// GetByID 读取任意版本。
func (Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Document, error) {
	var d Document
	err := q.QueryRow(ctx, `
		SELECT id, lesson_id, kind, revision, content_md, source_job_id, is_active, created_at
		FROM documents WHERE id = $1`, id).Scan(
		&d.ID, &d.LessonID, &d.Kind, &d.Revision, &d.ContentMD, &d.SourceJobID, &d.IsActive, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

// ListActiveByLesson 返回课时的全部当前发布版本（每 kind 至多一条），
// 按 kind 字典序排列（raw_text < student_notes < teacher_outline）。
func (Repository) ListActiveByLesson(ctx context.Context, q database.Querier, lessonID uuid.UUID) ([]Document, error) {
	rows, err := q.Query(ctx, `
		SELECT id, lesson_id, kind, revision, content_md, source_job_id, is_active, created_at
		FROM documents WHERE lesson_id = $1 AND is_active
		ORDER BY kind ASC`, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.LessonID, &d.Kind, &d.Revision, &d.ContentMD,
			&d.SourceJobID, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListVersions 按 revision DESC 返回指定 lesson+kind 的历史版本（游标分页）。
// beforeRevision 为 nil 时取第一页，否则只取 revision 严格小于它的行
// （UNIQUE(lesson_id, kind, revision) 使 revision 本身就是无歧义游标）。
// limit 由调用方给出（通常取页面大小 +1 以判定 has_more）。
func (Repository) ListVersions(ctx context.Context, q database.Querier,
	lessonID uuid.UUID, kind string, beforeRevision *int32, limit int) ([]Document, error) {
	rows, err := q.Query(ctx, `
		SELECT id, lesson_id, kind, revision, content_md, source_job_id, is_active, created_at
		FROM documents
		WHERE lesson_id = $1 AND kind = $2
		  AND ($3::int IS NULL OR revision < $3)
		ORDER BY revision DESC
		LIMIT $4`, lessonID, kind, beforeRevision, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.LessonID, &d.Kind, &d.Revision, &d.ContentMD,
			&d.SourceJobID, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
