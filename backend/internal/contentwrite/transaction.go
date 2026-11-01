// Package contentwrite coordinates manual edits with background generation.
package contentwrite

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

func NotFound() error {
	return httpx.NewProblemError(404, "not_found", "资源不存在", "请求的资源不存在或已删除。")
}
func Conflict() error {
	return httpx.NewProblemError(409, "version_conflict", "版本冲突", "内容已被修改，请重新读取后重试。")
}
func Invalid(field, message string) error {
	p := httpserver.NewProblem(422, "validation_error", "请求参数无效", "请修正标记的字段后重试。")
	p.Errors = []httpserver.FieldError{httpx.FieldErr(field, "invalid", message)}
	return &httpx.ProblemError{Problem: p}
}
func Lesson(ctx context.Context, q database.Querier, id uuid.UUID) error {
	var courseID uuid.UUID
	if err := q.QueryRow(ctx, `SELECT c.id FROM courses c JOIN lessons l ON l.course_id=c.id WHERE l.id=$1 FOR KEY SHARE OF c`, id).Scan(&courseID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotFound()
		}
		return err
	}
	var locked uuid.UUID
	if err := q.QueryRow(ctx, `SELECT id FROM lessons WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotFound()
		}
		return err
	}
	active, err := (&job.Repository{}).GetActiveRelatedLessonJob(ctx, q, id)
	if err != nil {
		return err
	}
	if active != nil {
		return job.ResourceBusyError(active.ID)
	}
	return nil
}
func Run(ctx context.Context, pool *pgxpool.Pool, lessonID uuid.UUID, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = gate.AssertBusinessOpen(ctx, tx); err != nil {
		return err
	}
	if err = Lesson(ctx, tx, lessonID); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RunCourse also supports course-wide concepts without an associated lesson.
func RunCourse(ctx context.Context, pool *pgxpool.Pool, courseID uuid.UUID, lessonID *uuid.UUID, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = gate.AssertBusinessOpen(ctx, tx); err != nil {
		return err
	}
	var locked uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM courses WHERE id=$1 FOR KEY SHARE`, courseID).Scan(&locked); errors.Is(err, pgx.ErrNoRows) {
		return NotFound()
	}
	if err != nil {
		return err
	}
	ids := []uuid.UUID{}
	rows, err := tx.Query(ctx, `SELECT id FROM lessons WHERE course_id=$1 AND ($2::uuid IS NULL OR id=$2) ORDER BY id`, courseID, lessonID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if lessonID != nil && len(ids) == 0 {
		return NotFound()
	}
	for _, id := range ids {
		if err = Lesson(ctx, tx, id); err != nil {
			return err
		}
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
