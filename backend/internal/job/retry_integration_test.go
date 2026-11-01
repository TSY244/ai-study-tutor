package job_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

// 任务审计不受课程级联删除影响，因此重试前必须重新确认业务资源存在。
func seedRetryTarget(t *testing.T, pool *pgxpool.Pool, aggregateType string, targetID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	courseID, lessonID, quizID := uuid.New(), uuid.New(), uuid.New()
	if aggregateType == "lesson" {
		lessonID = targetID
	}
	if _, err := pool.Exec(ctx, `INSERT INTO courses (id, slug, title) VALUES ($1,$2,'重试课程')`,
		courseID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,'重试课时')`,
		lessonID, courseID); err != nil {
		t.Fatal(err)
	}
	switch aggregateType {
	case "attempt":
		if _, err := pool.Exec(ctx, `INSERT INTO quizzes (id, lesson_id, title, source) VALUES ($1,$2,'试卷','regular')`,
			quizID, lessonID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO attempts (id, quiz_id) VALUES ($1,$2)`, targetID, quizID); err != nil {
			t.Fatal(err)
		}
	case "summary_evaluation":
		if _, err := pool.Exec(ctx, `INSERT INTO summary_evaluations (id, lesson_id, summary_text) VALUES ($1,$2,'总结')`,
			targetID, lessonID); err != nil {
			t.Fatal(err)
		}
	}
	return courseID
}

func TestRetryDeletedResource(t *testing.T) {
	for _, target := range []struct{ aggregateType, kind string }{
		{"lesson", job.KindLessonImport},
		{"attempt", job.KindAttemptGrade},
		{"summary_evaluation", job.KindSummaryEvaluate},
	} {
		t.Run(target.aggregateType, func(t *testing.T) {
			pool := databasetest.New(t)
			engine := newJobEngine(t, pool)
			j := createHTTPJob(t, pool, target.kind, target.aggregateType)
			courseID := seedRetryTarget(t, pool, target.aggregateType, j.AggregateID)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'failed', completed_at = now() WHERE id = $1`, j.ID); err != nil {
				t.Fatal(err)
			}
			if err := course.NewService(pool).Delete(ctx, courseID); err != nil {
				t.Fatal(err)
			}
			rec := apitest.Do(t, engine, http.MethodPost, "/api/v1/jobs/"+j.ID.String()+"/retry",
				map[string]string{"Idempotency-Key": uuid.NewString()}, "")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("已删除资源不能创建重试任务: status=%d body=%s", rec.Code, rec.Body.String())
			}
			apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("必须只保留原任务: count=%d err=%v", count, err)
			}
		})
	}
}

func TestRetryWithActiveGeneration(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	ctx := context.Background()
	orig := createHTTPJob(t, pool, job.KindLessonImport, "lesson")
	seedRetryTarget(t, pool, "lesson", orig.AggregateID)
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'failed', generation_family = 'lesson_generation' WHERE id = $1`, orig.ID); err != nil {
		t.Fatal(err)
	}
	active := createHTTPJob(t, pool, job.KindQuizGenerate, "lesson")
	if _, err := pool.Exec(ctx, `UPDATE jobs SET aggregate_id = $1, generation_family = 'lesson_generation' WHERE id = $2`,
		orig.AggregateID, active.ID); err != nil {
		t.Fatal(err)
	}
	rec := apitest.Do(t, engine, http.MethodPost, "/api/v1/jobs/"+orig.ID.String()+"/retry",
		map[string]string{"Idempotency-Key": uuid.NewString()}, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("已有生成任务应返回冲突: status=%d body=%s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
}

func TestRetryWithActiveGrading(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	ctx := context.Background()
	orig := createHTTPJob(t, pool, job.KindAttemptGrade, "attempt")
	seedRetryTarget(t, pool, "attempt", orig.AggregateID)
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'failed' WHERE id = $1`, orig.ID); err != nil {
		t.Fatal(err)
	}
	active := createHTTPJob(t, pool, job.KindAttemptGrade, "attempt")
	if _, err := pool.Exec(ctx, `UPDATE jobs SET aggregate_id = $1 WHERE id = $2`, orig.AggregateID, active.ID); err != nil {
		t.Fatal(err)
	}
	rec := apitest.Do(t, engine, http.MethodPost, "/api/v1/jobs/"+orig.ID.String()+"/retry",
		map[string]string{"Idempotency-Key": uuid.NewString()}, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("已有批阅任务应返回冲突: status=%d body=%s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
}

func TestRetryConcurrentLessonDeletion(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	orig := createHTTPJob(t, pool, job.KindLessonImport, "lesson")
	seedRetryTarget(t, pool, "lesson", orig.AggregateID)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'failed' WHERE id = $1`, orig.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT id FROM lessons WHERE id = $1 FOR UPDATE`, orig.AggregateID); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost,
			"http://localhost/api/v1/jobs/"+orig.ID.String()+"/retry", strings.NewReader("")).WithContext(ctx)
		req.Header.Set("Idempotency-Key", uuid.NewString())
		engine.ServeHTTP(rec, req)
		done <- rec
	}()
	databasetest.WaitForBlocked(t, pool, tx.Conn().PgConn().PID())
	if _, err := tx.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, orig.AggregateID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rec := <-done
	if rec.Code != http.StatusNotFound {
		t.Fatalf("并发删除后不能重试: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("只允许保留原失败任务: count=%d err=%v", count, err)
	}
}
