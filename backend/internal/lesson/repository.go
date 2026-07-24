// Package lesson 提供课时聚合的 Repository：B2 最小集与 B3 扩展
// （乐观锁更新、分页列表、文档可用性、引用探测、级联删除）。
package lesson

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
)

var (
	// ErrDuplicateOrdinal：同一课程内 ordinal 唯一冲突。
	ErrDuplicateOrdinal = errors.New("课程内已存在相同序号的课时")
	// ErrNotFound：课时不存在。
	ErrNotFound = errors.New("课时不存在")
	// ErrVersionConflict：乐观锁冲突，映射 409 version_conflict。
	ErrVersionConflict = errors.New("课时版本冲突")
)

// Lesson 是 lessons 表的领域表示。
type Lesson struct {
	ID          uuid.UUID
	CourseID    uuid.UUID
	Ordinal     int32
	Title       string
	Description *string
	Metadata    []byte
	Status      string
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository struct{}

// Create 插入课时（UNIQUE(course_id, ordinal)）。
func (Repository) Create(ctx context.Context, q database.Querier, l *Lesson) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	if l.Metadata == nil {
		l.Metadata = []byte(`{}`)
	}
	err := q.QueryRow(ctx, `
		INSERT INTO lessons (id, course_id, ordinal, title, description, metadata)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING status, version, created_at, updated_at`,
		l.ID, l.CourseID, l.Ordinal, l.Title, l.Description, l.Metadata).
		Scan(&l.Status, &l.Version, &l.CreatedAt, &l.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateOrdinal
	}
	if err != nil {
		return fmt.Errorf("创建课时失败: %w", err)
	}
	return nil
}

// GetByID 读取课时。
func (Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Lesson, error) {
	var l Lesson
	err := q.QueryRow(ctx, `
		SELECT id, course_id, ordinal, title, description, metadata, status,
		       version, created_at, updated_at
		FROM lessons WHERE id = $1`, id).Scan(
		&l.ID, &l.CourseID, &l.Ordinal, &l.Title, &l.Description, &l.Metadata,
		&l.Status, &l.Version, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &l, err
}

// GetCourseID 返回课时所属课程；课时不存在时 found=false（供 mastery 等
// 模块以结构化接口解析范围，不跨模块传递哨兵错误）。
func (Repository) GetCourseID(ctx context.Context, q database.Querier, lessonID uuid.UUID) (uuid.UUID, bool, error) {
	var courseID uuid.UUID
	err := q.QueryRow(ctx, `SELECT course_id FROM lessons WHERE id = $1`, lessonID).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return courseID, true, nil
}

// UpdateStatus 更新课时状态与乐观锁版本。
func (Repository) UpdateStatus(ctx context.Context, q database.Querier, id uuid.UUID, status string, version int64) error {
	tag, err := q.Exec(ctx, `
		UPDATE lessons SET status = $2, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $3`, id, status, version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Update 乐观锁更新课时（ordinal/title/description）：
// version 不匹配返回 ErrVersionConflict；ordinal 唯一冲突返回 ErrDuplicateOrdinal。
func (Repository) Update(ctx context.Context, q database.Querier, l *Lesson) error {
	err := q.QueryRow(ctx, `
		UPDATE lessons
		SET ordinal = $3, title = $4, description = $5,
		    version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
		RETURNING version, updated_at`,
		l.ID, l.Version, l.Ordinal, l.Title, l.Description).Scan(&l.Version, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionConflict
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateOrdinal
	}
	return err
}

// ListByCourse 按 ordinal ASC, id ASC 游标分页返回课程下的课时。
// afterOrdinal 与 afterID 同时非 nil 时从该游标之后开始。
func (Repository) ListByCourse(ctx context.Context, q database.Querier, courseID uuid.UUID,
	afterOrdinal *int32, afterID *uuid.UUID, limit int) ([]Lesson, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := q.Query(ctx, `
		SELECT id, course_id, ordinal, title, description, metadata, status,
		       version, created_at, updated_at
		FROM lessons
		WHERE course_id = $1
		  AND ($2::integer IS NULL OR (ordinal, id) > ($2, $3::uuid))
		ORDER BY ordinal ASC, id ASC
		LIMIT $4`, courseID, afterOrdinal, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Lesson, 0, min(limit, 128))
	for rows.Next() {
		var l Lesson
		if err := rows.Scan(&l.ID, &l.CourseID, &l.Ordinal, &l.Title, &l.Description,
			&l.Metadata, &l.Status, &l.Version, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// DocumentAvailability 记录某课时三类文档是否各存在 active 版本。
type DocumentAvailability struct {
	RawText        bool
	StudentNotes   bool
	TeacherOutline bool
}

// DocumentAvailabilityByLesson 一次 GROUP BY 查询返回多个课时的文档可用性。
func (Repository) DocumentAvailabilityByLesson(ctx context.Context, q database.Querier,
	lessonIDs []uuid.UUID) (map[uuid.UUID]DocumentAvailability, error) {
	out := make(map[uuid.UUID]DocumentAvailability, len(lessonIDs))
	if len(lessonIDs) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `
		SELECT lesson_id, kind FROM documents
		WHERE lesson_id = ANY($1) AND is_active
		GROUP BY lesson_id, kind`, lessonIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var lessonID uuid.UUID
		var kind string
		if err := rows.Scan(&lessonID, &kind); err != nil {
			return nil, err
		}
		avail := out[lessonID]
		switch kind {
		case "raw_text":
			avail.RawText = true
		case "student_notes":
			avail.StudentNotes = true
		case "teacher_outline":
			avail.TeacherOutline = true
		}
		out[lessonID] = avail
	}
	return out, rows.Err()
}

// HasQuizReference 报告课时是否被任何试卷引用（不限状态，保护作答历史）。
func (Repository) HasQuizReference(ctx context.Context, q database.Querier, id uuid.UUID) (bool, error) {
	var referenced bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM quizzes WHERE lesson_id = $1)`, id).Scan(&referenced)
	return referenced, err
}

// DeleteCascade 在调用方事务内物理删除课时及其子数据，并把关联
// file_objects 标记为 deleted。调用方必须已确认无试卷引用、无活动任务；
// jobs/job_events/ai_runs 作为任务审计保留。
func (Repository) DeleteCascade(ctx context.Context, q database.Querier, id uuid.UUID) error {
	stmts := []string{
		// 关联文件对象标记删除（temporary 行的 Owner 字段一并清空以满足 CHECK）。
		`UPDATE file_objects SET state = 'deleted',
		    temporary_owner_type = NULL, temporary_owner_id = NULL,
		    temporary_expires_at = NULL, updated_at = now()
		 WHERE id IN (
		    SELECT file_object_id FROM materials
		    WHERE lesson_id = $1 AND file_object_id IS NOT NULL)
		   AND state <> 'deleted'`,
		`DELETE FROM materials WHERE lesson_id = $1`,
		`DELETE FROM documents WHERE lesson_id = $1`,
		`DELETE FROM questions WHERE lesson_id = $1`,
		`DELETE FROM summary_evaluations WHERE lesson_id = $1`,
		`DELETE FROM weaknesses WHERE lesson_id = $1`,
		`DELETE FROM mastery_snapshots WHERE lesson_id = $1`,
		`DELETE FROM lessons WHERE id = $1`,
	}
	for _, stmt := range stmts {
		if _, err := q.Exec(ctx, stmt, id); err != nil {
			return fmt.Errorf("删除课时子数据失败: %w", err)
		}
	}
	return nil
}
