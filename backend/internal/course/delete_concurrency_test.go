package course_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

func TestDeleteCourseConcurrentGeneration(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			pool := databasetest.New(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			c := &course.Course{Slug: uuid.NewString(), Title: "并发删除"}
			other := &course.Course{Slug: uuid.NewString(), Title: "整体保留"}
			for _, item := range []*course.Course{c, other} {
				if err := (course.Repository{}).Create(ctx, pool, item); err != nil {
					t.Fatal(err)
				}
			}
			l := &lesson.Lesson{CourseID: c.ID, Ordinal: 1, Title: "并发生成"}
			if err := (lesson.Repository{}).Create(ctx, pool, l); err != nil {
				t.Fatal(err)
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			if _, err := tx.Exec(ctx, `SELECT id FROM lessons WHERE id = $1 FOR UPDATE`, l.ID); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				svc := course.NewService(pool)
				if batch {
					done <- svc.DeleteMany(ctx, []uuid.UUID{other.ID, c.ID})
				} else {
					done <- svc.Delete(ctx, c.ID)
				}
			}()
			databasetest.WaitForBlocked(t, pool, tx.Conn().PgConn().PID())
			family := job.GenerationFamilyLessonGeneration
			j := &job.Job{Kind: job.KindQuizGenerate, AggregateType: "lesson", AggregateID: l.ID,
				GenerationFamily: &family, Payload: []byte(`{}`)}
			if err := (&job.Repository{}).Create(ctx, tx, j); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			err = <-done
			var problem *httpx.ProblemError
			if !errors.As(err, &problem) || problem.Problem.Code != "resource_busy" {
				t.Fatalf("并发生成后必须拒绝删除，得到: %v", err)
			}
			for _, id := range []uuid.UUID{c.ID, other.ID} {
				if _, err := (course.Repository{}).GetByID(ctx, pool, id); err != nil {
					t.Fatalf("课程必须保留: %v", err)
				}
			}
			if _, err := (lesson.Repository{}).GetByID(ctx, pool, l.ID); err != nil {
				t.Fatalf("课时必须保留: %v", err)
			}
		})
	}
}
