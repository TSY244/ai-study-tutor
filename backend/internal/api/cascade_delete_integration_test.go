package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
	"github.com/google/uuid"
)

func gradedCascadeQuiz(e *contentEnv) (string, string) {
	e.t.Helper()
	z := e.call("POST", "/lessons/"+e.lesson+"/quizzes", "/lessons/{lesson_id}/quizzes", map[string]any{"title": "待级联删除", "questions": []any{question("有序性？")}}, 201)
	zid := z["id"].(string)
	a := e.call("POST", "/quizzes/"+zid+"/attempts", "/quizzes/{quiz_id}/attempts", map[string]any{}, 201)
	aid := a["id"].(string)
	qid := a["questions"].([]any)[0].(map[string]any)["id"].(string)
	saved := e.call("PATCH", "/attempts/"+aid+"/answers/"+qid, "/attempts/{attempt_id}/answers/{question_id}", map[string]any{"answer_version": 0, "answer": map[string]any{"value": "B"}, "client_updated_at": time.Now().UTC().Format(time.RFC3339)}, 200)
	e.call("POST", "/attempts/"+aid+"/submit", "/attempts/{attempt_id}/submit", map[string]any{"version": saved["attempt_version"]}, 200)
	return zid, aid
}
func assertNoRows(t *testing.T, e *contentEnv, query string, args ...any) {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil || n != 0 {
		t.Fatalf("orphan rows: %d %v (%s)", n, err, query)
	}
}
func TestCascadeDeleteLessonWithAllChildren(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[batch], func(t *testing.T) {
			e := newContentEnv(t)
			e.raw("数组必须有序。")
			e.call("POST", "/lessons/"+e.lesson+"/knowledge-points", "/lessons/{lesson_id}/knowledge-points", map[string]any{"label": "有序性", "explanation": "依赖有序数据", "learning_objective": "判断前提", "source_excerpt": "数组必须有序", "importance": "core"}, 201)
			e.call("POST", "/lessons/"+e.lesson+"/materials/text", "/lessons/{lesson_id}/materials/text", map[string]any{"text": "素材"}, 202)
			e.settleJobs()
			e.call("POST", "/lessons/"+e.lesson+"/summary-evaluations", "/lessons/{lesson_id}/summary-evaluations", map[string]any{"summary": "总结"}, 202)
			e.settleJobs()
			fileID := uuid.NewString()
			e.exec(`INSERT INTO file_objects(id,storage_key,state) VALUES($1,$2,'ready')`, fileID, "cascade/"+fileID)
			e.exec(`INSERT INTO materials(id,lesson_id,kind,file_object_id,status) VALUES($1,$2,'upload',$3,'ready')`, uuid.New(), e.lesson, fileID)
			zid, aid := gradedCascadeQuiz(e)
			other := e.call("POST", "/courses/"+e.course+"/lessons", "/courses/{course_id}/lessons", map[string]any{"title": "保留的课时", "ordinal": 2}, 201)["id"].(string)
			// Course-wide manual evidence without attempt IDs must survive cleanup.
			evidence, _ := json.Marshal([]any{map[string]any{"attempt_id": aid, "quiz_id": zid}, map[string]any{"note": "人工备注"}})
			e.exec(`INSERT INTO weaknesses(id,course_id,concept_key,severity,evidence) VALUES($1,$2,'manual',2,$3)`, uuid.New(), e.course, evidence)
			if batch {
				e.call("POST", "/lessons:batch-delete", "/lessons:batch-delete", map[string]any{"ids": []string{e.lesson}}, 204)
			} else {
				e.call("DELETE", "/lessons/"+e.lesson, "/lessons/{lesson_id}", nil, 204)
			}
			for _, table := range []string{"materials", "documents", "questions", "quizzes", "course_knowledge_points", "summary_evaluations", "weaknesses", "mastery_snapshots"} {
				assertNoRows(t, e, "SELECT count(*) FROM "+table+" WHERE lesson_id=$1", e.lesson)
			}
			assertNoRows(t, e, `SELECT count(*) FROM attempts WHERE quiz_id=$1`, zid)
			assertNoRows(t, e, `SELECT count(*) FROM attempt_answers WHERE attempt_id=$1`, aid)
			assertNoRows(t, e, `SELECT count(*) FROM mastery_snapshots WHERE source_attempt_id=$1`, aid)
			var state string
			if err := e.pool.QueryRow(context.Background(), `SELECT state FROM file_objects WHERE id=$1`, fileID).Scan(&state); err != nil || state != "deleted" {
				t.Fatalf("file state %s %v", state, err)
			}
			var retained []byte
			if err := e.pool.QueryRow(context.Background(), `SELECT evidence FROM weaknesses WHERE course_id=$1 AND lesson_id IS NULL`, e.course).Scan(&retained); err != nil || string(retained) != `[{"note": "人工备注"}]` {
				t.Fatalf("manual evidence: %s %v", retained, err)
			}
			e.call("GET", "/lessons/"+e.lesson, "/lessons/{lesson_id}", nil, 404)
			e.call("GET", "/lessons/"+other, "/lessons/{lesson_id}", nil, 200)
			p := e.call("GET", "/courses/"+e.course+"/progress", "/courses/{course_id}/progress", nil, 200)
			if p["completed_attempts"] != float64(0) {
				t.Fatalf("stale progress: %v", p)
			}
		})
	}
}
func TestCascadeDeleteQuizWithGradedAttempt(t *testing.T) {
	e := newContentEnv(t)
	zid, aid := gradedCascadeQuiz(e)
	e.call("DELETE", "/quizzes/"+zid, "/quizzes/{quiz_id}", nil, 204)
	for _, query := range []string{`SELECT count(*) FROM attempts WHERE id=$1`, `SELECT count(*) FROM attempt_answers WHERE attempt_id=$1`, `SELECT count(*) FROM mastery_snapshots WHERE source_attempt_id=$1`} {
		assertNoRows(t, e, query, aid)
	}
	assertNoRows(t, e, `SELECT count(*) FROM questions WHERE lesson_id=$1`, e.lesson)
	assertNoRows(t, e, `SELECT count(*) FROM weaknesses w CROSS JOIN LATERAL jsonb_array_elements(w.evidence) evidence WHERE evidence->>'attempt_id'=$1`, aid)
	p := e.call("GET", "/lessons/"+e.lesson+"/progress", "/lessons/{lesson_id}/progress", nil, 200)
	if p["completed_attempts"] != float64(0) {
		t.Fatalf("stale progress %v", p)
	}
	e.call("GET", "/lessons/"+e.lesson, "/lessons/{lesson_id}", nil, 200)
}
func TestCascadeDeleteWaitsForAttemptThenRechecksJobs(t *testing.T) {
	for _, scope := range []string{"lesson", "quiz", "course"} {
		t.Run(scope, func(t *testing.T) {
			e := newContentEnv(t)
			zid, aid := gradedCascadeQuiz(e)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			tx, err := e.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(ctx, `SELECT id FROM attempts WHERE id=$1 FOR UPDATE`, aid); err != nil {
				t.Fatal(err)
			}
			id := uuid.MustParse(e.lesson)
			done := make(chan error, 1)
			go func() {
				switch scope {
				case "lesson":
					done <- lesson.NewService(e.pool).Delete(ctx, id)
				case "quiz":
					done <- quiz.NewQueryService(e.pool).DeleteManual(ctx, uuid.MustParse(zid))
				case "course":
					done <- course.NewService(e.pool).Delete(ctx, uuid.MustParse(e.course))
				}
			}()
			databasetest.WaitForBlocked(t, e.pool, tx.Conn().PgConn().PID())
			if _, err = tx.Exec(ctx, `INSERT INTO jobs(id,kind,aggregate_type,aggregate_id,payload) VALUES($1,'attempt_grade','attempt',$2,'{}')`, uuid.New(), aid); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var problem *httpx.ProblemError
			if err = <-done; !errors.As(err, &problem) || problem.Problem.Code != "resource_busy" {
				t.Fatalf("must protect concurrent job: %v", err)
			}
			e.call("GET", "/attempts/"+aid, "/attempts/{attempt_id}", nil, 200)
			e.call("GET", "/lessons/"+e.lesson, "/lessons/{lesson_id}", nil, 200)
		})
	}
}

func TestCascadeQuizDeletePreservesQuestionsSharedByAnotherQuiz(t *testing.T) {
	e := newContentEnv(t)
	create := func(title string) map[string]any {
		return e.call("POST", "/lessons/"+e.lesson+"/quizzes", "/lessons/{lesson_id}/quizzes", map[string]any{"title": title, "questions": []any{question(title)}}, 201)
	}
	first, other := create("第一份"), create("保留的测验")
	firstID, otherID := first["id"].(string), other["id"].(string)
	var questionID uuid.UUID
	if err := e.pool.QueryRow(context.Background(), `SELECT question_id FROM quiz_items WHERE quiz_id=$1`, firstID).Scan(&questionID); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO quiz_items(quiz_id,question_id,position,score) VALUES($1,$2,2,5)`, otherID, questionID)
	e.call("DELETE", "/quizzes/"+firstID, "/quizzes/{quiz_id}", nil, 204)
	remaining := e.call("GET", "/quizzes/"+otherID, "/quizzes/{quiz_id}", nil, 200)
	if len(remaining["questions"].([]any)) != 2 {
		t.Fatal("shared question was removed")
	}
}

func TestSubmissionLocksParentsBeforeAttempt(t *testing.T) {
	for _, parent := range []string{"course", "lesson"} {
		t.Run(parent, func(t *testing.T) {
			e := newContentEnv(t)
			z := e.call("POST", "/lessons/"+e.lesson+"/quizzes", "/lessons/{lesson_id}/quizzes", map[string]any{"title": "并发提交", "questions": []any{question("测试")}}, 201)
			a := e.call("POST", "/quizzes/"+z["id"].(string)+"/attempts", "/quizzes/{quiz_id}/attempts", map[string]any{}, 201)
			aid := a["id"].(string)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			tx, err := e.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			stmt, id := `SELECT id FROM courses WHERE id=$1 FOR UPDATE`, e.course
			if parent == "lesson" {
				stmt, id = `SELECT id FROM lessons WHERE id=$1 FOR UPDATE`, e.lesson
			}
			if _, err = tx.Exec(ctx, stmt, id); err != nil {
				t.Fatal(err)
			}
			done := make(chan int, 1)
			go func() {
				done <- e.request("POST", "/attempts/"+aid+"/submit", map[string]any{"version": a["version"]}, uuid.NewString()).Code
			}()
			databasetest.WaitForBlocked(t, e.pool, tx.Conn().PgConn().PID())
			// If submission took the attempt first, deletion would deadlock here.
			if _, err = tx.Exec(ctx, `SELECT id FROM attempts WHERE id=$1 FOR UPDATE NOWAIT`, aid); err != nil {
				t.Fatalf("submission locked the child before parent: %v", err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if status := <-done; status != 200 {
				t.Fatalf("submit status %d", status)
			}
		})
	}
}
