// Package course 提供课程聚合的 Repository：B2 最小集（创建/读取/乐观锁更新）
// 与 B3 扩展（分页列表、课时数视图、活动任务探测、级联删除）。
package course

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
)

var (
	// ErrSlugTaken：course slug 唯一冲突（courses_slug_key）。
	ErrSlugTaken = errors.New("课程 slug 已存在")
	// ErrNotFound：课程不存在。
	ErrNotFound = errors.New("课程不存在")
	// ErrVersionConflict：乐观锁冲突，映射 409 version_conflict。
	ErrVersionConflict = errors.New("课程版本冲突")
)

// Course 是 courses 表的领域表示。
type Course struct {
	ID          uuid.UUID
	Slug        string
	Title       string
	Description *string
	Archived    bool
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository struct{}

func isUnique(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// Create 插入课程（slug 由服务端生成，唯一）。
func (Repository) Create(ctx context.Context, q database.Querier, c *Course) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	err := q.QueryRow(ctx, `
		INSERT INTO courses (id, slug, title, description)
		VALUES ($1,$2,$3,$4)
		RETURNING version, created_at, updated_at`,
		c.ID, c.Slug, c.Title, c.Description).
		Scan(&c.Version, &c.CreatedAt, &c.UpdatedAt)
	if isUnique(err, "courses_slug_key") {
		return ErrSlugTaken
	}
	if err != nil {
		return fmt.Errorf("创建课程失败: %w", err)
	}
	return nil
}

// GetByID 读取课程。
func (Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Course, error) {
	var c Course
	err := q.QueryRow(ctx, `
		SELECT id, slug, title, description, archived, version, created_at, updated_at
		FROM courses WHERE id = $1`, id).Scan(
		&c.ID, &c.Slug, &c.Title, &c.Description, &c.Archived, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// Exists 返回课程是否存在（供 mastery 等模块以结构化接口校验范围）。
func (Repository) Exists(ctx context.Context, q database.Querier, id uuid.UUID) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

// Update 乐观锁更新：version 不匹配返回 ErrVersionConflict，成功则 version+1。
func (Repository) Update(ctx context.Context, q database.Querier, c *Course) error {
	err := q.QueryRow(ctx, `
		UPDATE courses
		SET title = $3, description = $4, archived = $5,
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
		RETURNING version, updated_at`,
		c.ID, c.Version, c.Title, c.Description, c.Archived).Scan(&c.Version, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionConflict
	}
	return err
}

// CourseWithCount 是课程与其课时数量的组合视图（列表/详情 DTO 用）。
type CourseWithCount struct {
	Course
	LessonCount int64
}

// ListFilter 描述课程列表查询条件；零值表示不过滤/从头开始。
type ListFilter struct {
	// Archived 非 nil 时按归档状态过滤。
	Archived *bool
	// Query 为标题包含过滤的原始子串（Repository 负责 ILIKE 转义）。
	Query string
	// AfterCreatedAt 与 AfterID 同时非 nil 时从该游标之后开始
	// （排序键 created_at ASC, id ASC）。
	AfterCreatedAt *time.Time
	AfterID        *uuid.UUID
	// Limit 为最大返回行数（调用方通常传 limit+1 以判定 has_more）。
	Limit int
}

// List 按 created_at ASC, id ASC 游标分页返回课程（含课时数）。
func (Repository) List(ctx context.Context, q database.Querier, f ListFilter) ([]CourseWithCount, error) {
	var (
		conds []string
		args  []any
	)
	if f.Archived != nil {
		args = append(args, *f.Archived)
		conds = append(conds, fmt.Sprintf("archived = $%d", len(args)))
	}
	if f.Query != "" {
		args = append(args, escapeLike(f.Query))
		conds = append(conds, fmt.Sprintf("title ILIKE '%%' || $%d || '%%' ESCAPE '\\'", len(args)))
	}
	if f.AfterCreatedAt != nil && f.AfterID != nil {
		args = append(args, *f.AfterCreatedAt, *f.AfterID)
		conds = append(conds, fmt.Sprintf("(created_at, id) > ($%d, $%d)", len(args)-1, len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	if f.Limit <= 0 {
		f.Limit = 20
	}
	args = append(args, f.Limit)
	rows, err := q.Query(ctx, `
		SELECT c.id, c.slug, c.title, c.description, c.archived, c.version,
		       c.created_at, c.updated_at,
		       (SELECT count(*) FROM lessons l WHERE l.course_id = c.id) AS lesson_count
		FROM courses c `+where+`
		ORDER BY c.created_at ASC, c.id ASC
		LIMIT $`+fmt.Sprintf("%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CourseWithCount, 0, min(f.Limit, 128))
	for rows.Next() {
		var c CourseWithCount
		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.Description, &c.Archived,
			&c.Version, &c.CreatedAt, &c.UpdatedAt, &c.LessonCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetWithCount 读取单个课程（含课时数）；不存在返回 ErrNotFound。
func (Repository) GetWithCount(ctx context.Context, q database.Querier, id uuid.UUID) (*CourseWithCount, error) {
	var c CourseWithCount
	err := q.QueryRow(ctx, `
		SELECT c.id, c.slug, c.title, c.description, c.archived, c.version,
		       c.created_at, c.updated_at,
		       (SELECT count(*) FROM lessons l WHERE l.course_id = c.id) AS lesson_count
		FROM courses c WHERE c.id = $1`, id).Scan(
		&c.ID, &c.Slug, &c.Title, &c.Description, &c.Archived,
		&c.Version, &c.CreatedAt, &c.UpdatedAt, &c.LessonCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// HasActiveGenerationJob 报告该课程任一课时下是否存在活动的
// lesson_generation 任务（删除保护，04 §8）。
func (Repository) HasActiveGenerationJob(ctx context.Context, q database.Querier, id uuid.UUID) (bool, error) {
	var busy bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM jobs j
			WHERE j.aggregate_type = 'lesson'
			  AND j.aggregate_id IN (SELECT id FROM lessons WHERE course_id = $1)
			  AND j.generation_family = 'lesson_generation'
			  AND j.status IN ('queued','running','validating','cancel_requested'))`, id).Scan(&busy)
	return busy, err
}

// DeleteCascade 在调用方事务内物理删除课程及其全部子数据，并把关联
// file_objects 标记为 deleted（文件字节由文件 Recovery 清理）。
// 删除顺序按外键依赖拓扑排列；jobs/job_events/ai_runs 作为任务审计保留。
func (Repository) DeleteCascade(ctx context.Context, q database.Querier, id uuid.UUID) error {
	stmts := []string{
		// 关联文件对象标记删除（temporary 行的 Owner 字段一并清空以满足 CHECK）。
		`UPDATE file_objects SET state = 'deleted',
		    temporary_owner_type = NULL, temporary_owner_id = NULL,
		    temporary_expires_at = NULL, updated_at = now()
		 WHERE id IN (
		    SELECT m.file_object_id FROM materials m
		    JOIN lessons l ON l.id = m.lesson_id
		    WHERE l.course_id = $1 AND m.file_object_id IS NOT NULL)
		   AND state <> 'deleted'`,
		`DELETE FROM attempt_answers WHERE attempt_id IN (
		    SELECT a.id FROM attempts a
		    JOIN quizzes z ON z.id = a.quiz_id
		    JOIN lessons l ON l.id = z.lesson_id WHERE l.course_id = $1)`,
		`DELETE FROM mastery_snapshots WHERE course_id = $1`,
		`DELETE FROM attempts WHERE quiz_id IN (
		    SELECT z.id FROM quizzes z
		    JOIN lessons l ON l.id = z.lesson_id WHERE l.course_id = $1)`,
		`DELETE FROM quiz_items WHERE quiz_id IN (
		    SELECT z.id FROM quizzes z
		    JOIN lessons l ON l.id = z.lesson_id WHERE l.course_id = $1)`,
		`DELETE FROM quizzes WHERE lesson_id IN (
		    SELECT id FROM lessons WHERE course_id = $1)`,
		`DELETE FROM questions WHERE lesson_id IN (
		    SELECT id FROM lessons WHERE course_id = $1)`,
		`DELETE FROM materials WHERE lesson_id IN (
		    SELECT id FROM lessons WHERE course_id = $1)`,
		`DELETE FROM documents WHERE lesson_id IN (
		    SELECT id FROM lessons WHERE course_id = $1)`,
		`DELETE FROM summary_evaluations WHERE lesson_id IN (
		    SELECT id FROM lessons WHERE course_id = $1)`,
		`DELETE FROM weaknesses WHERE course_id = $1`,
		`DELETE FROM lessons WHERE course_id = $1`,
		`DELETE FROM courses WHERE id = $1`,
	}
	for _, stmt := range stmts {
		if _, err := q.Exec(ctx, stmt, id); err != nil {
			return fmt.Errorf("删除课程子数据失败: %w", err)
		}
	}
	return nil
}

// escapeLike 转义 ILIKE 模式中的特殊字符（配合 ESCAPE '\'）。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
