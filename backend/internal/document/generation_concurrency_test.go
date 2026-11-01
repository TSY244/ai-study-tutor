package document_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

func TestRegenerationConcurrentLessonDeletion(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	lessonID := seedLesson(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT id FROM lessons WHERE id = $1 FOR UPDATE`, lessonID); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/lessons/"+lessonID.String()+"/regenerations", strings.NewReader(`{"kinds":["student_notes"]}`)).WithContext(ctx)
		req.Host = "localhost"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		engine.ServeHTTP(rec, req)
		done <- rec
	}()
	databasetest.WaitForBlocked(t, pool, tx.Conn().PgConn().PID())
	if _, err := tx.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, lessonID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rec := <-done
	if rec.Code != http.StatusNotFound {
		t.Fatalf("并发删除后不能接受生成任务: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("不得产生孤立任务: count=%d err=%v", count, err)
	}
}
