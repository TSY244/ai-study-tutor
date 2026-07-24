// Package mastery_test 补充 B4 的薄弱点与进度端点集成测试：
// 列表过滤/分页、详情、resolve/reopen 乐观锁与幂等回放、
// 人工调整后追加快照、进度聚合与 404。
package mastery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/attempt"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

const (
	weaknessesPath = "/api/v1/weaknesses"
	weaknessPath   = "/api/v1/weaknesses/{weakness_id}"
	resolvePath    = "/api/v1/weaknesses/{weakness_id}/resolve"
	reopenPath     = "/api/v1/weaknesses/{weakness_id}/reopen"
	courseProgPath = "/api/v1/courses/{course_id}/progress"
	lessonProgPath = "/api/v1/lessons/{lesson_id}/progress"
)

func newEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	mastery.NewHandler(
		mastery.NewWeaknessService(pool, attempt.Repository{}),
		mastery.NewProgressService(pool, attempt.Repository{}, course.Repository{}, lesson.Repository{}),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)}).
		RegisterRoutes(e.Group("/api/v1"),
			httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"))
	return e
}

// seed 创建课程+课时，并用生产 ApplyService 应用一次答错结果（产生薄弱点与快照）。
func seed(t *testing.T, pool *pgxpool.Pool) (courseID, lessonID, attemptID, questionID uuid.UUID) {
	t.Helper()
	courseID, lessonID, attemptID, questionID = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO courses (id, slug, title) VALUES ($1,$2,'课程')`,
		courseID, "c-"+uuid.NewString()[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,'课时')`,
		lessonID, courseID); err != nil {
		t.Fatal(err)
	}
	quizID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO quizzes (id, lesson_id, title, source, status) VALUES ($1,$2,'q','regular','ready')`,
		quizID, lessonID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO attempts (id, quiz_id, status, auto_score, final_score, max_score,
		    submitted_at, graded_at)
		VALUES ($1,$2,'graded',4,4,10,now(),now())`, attemptID, quizID); err != nil {
		t.Fatal(err)
	}
	truth := false
	applied, err := mastery.NewApplyService(attempt.Repository{}).ApplyAttemptOutcome(ctx, pool, mastery.ApplyInput{
		CourseID: courseID, LessonID: lessonID, AttemptID: attemptID, QuizID: quizID,
		Items: []mastery.OutcomeItem{
			{QuestionID: questionID, Concepts: []string{"极限"}, Correct: &truth, Note: "客观题作答判分"},
		},
	})
	if err != nil || !applied {
		t.Fatalf("应用判分结果失败: applied=%v err=%v", applied, err)
	}
	return courseID, lessonID, attemptID, questionID
}

func do(t *testing.T, engine *gin.Engine, method, path, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{"Content-Type": "application/json"}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	return apitest.Do(t, engine, method, path, headers, body)
}

func jsonBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return m
}

func weaknessID(t *testing.T, pool *pgxpool.Pool, courseID uuid.UUID) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM weaknesses WHERE course_id = $1 AND concept_key = '极限'`, courseID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestWeaknessListGetResolveReopen(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID, _, _, _ := seed(t, pool)
	wID := weaknessID(t, pool, courseID)

	// 列表：过滤生效，契约一致。
	rec := do(t, engine, http.MethodGet,
		"/api/v1/weaknesses?course_id="+courseID.String()+"&mastered=false&severity=3", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("列表 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, weaknessesPath, http.StatusOK, rec.Body.Bytes())
	m := jsonBody(t, rec.Body.Bytes())
	items := m["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != wID {
		t.Fatalf("列表过滤错误: %v", m)
	}
	item := items[0].(map[string]any)
	if item["severity"].(float64) != 3 || item["mastered"] != false ||
		item["review_count"].(float64) != 1 || len(item["evidence"].([]any)) != 1 {
		t.Fatalf("薄弱点字段错误: %v", item)
	}
	// severity 过滤排除。
	rec = do(t, engine, http.MethodGet, "/api/v1/weaknesses?severity=5", "", "")
	if len(jsonBody(t, rec.Body.Bytes())["items"].([]any)) != 0 {
		t.Fatal("severity=5 应为空列表")
	}

	// 详情 200 / 404。
	rec = do(t, engine, http.MethodGet, "/api/v1/weaknesses/"+wID, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("详情 status = %d", rec.Code)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, weaknessPath, http.StatusOK, rec.Body.Bytes())
	rec = do(t, engine, http.MethodGet, "/api/v1/weaknesses/"+uuid.NewString(), "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在薄弱点 status = %d", rec.Code)
	}

	// resolve：版本不一致 → 409 version_conflict + current_version。
	rec = do(t, engine, http.MethodPost, "/api/v1/weaknesses/"+wID+"/resolve",
		uuid.NewString(), `{"version":99}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("版本冲突 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "version_conflict")
	if jsonBody(t, rec.Body.Bytes())["current_version"].(float64) != 1 {
		t.Fatalf("冲突扩展错误: %v", jsonBody(t, rec.Body.Bytes()))
	}

	// resolve 成功：mastered=true、severity 归 1；同 Key 回放一致；追加快照。
	key := uuid.NewString()
	rec = do(t, engine, http.MethodPost, "/api/v1/weaknesses/"+wID+"/resolve", key, `{"version":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, resolvePath, http.StatusOK, rec.Body.Bytes())
	m = jsonBody(t, rec.Body.Bytes())
	if m["mastered"] != true || m["severity"].(float64) != 1 || m["version"].(float64) != 2 {
		t.Fatalf("resolve 结果错误: %v", m)
	}
	rec = do(t, engine, http.MethodPost, "/api/v1/weaknesses/"+wID+"/resolve", key, `{"version":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve 回放 status = %d", rec.Code)
	}
	var snapCount int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM mastery_snapshots
		WHERE course_id = $1 AND source_attempt_id IS NULL`, courseID).Scan(&snapCount); err != nil {
		t.Fatal(err)
	}
	if snapCount != 2 { // lesson + course 各一条
		t.Fatalf("人工调整应追加 2 条快照，实际 %d", snapCount)
	}

	// reopen：重新激活（mastered=false，severity 保持）。
	rec = do(t, engine, http.MethodPost, "/api/v1/weaknesses/"+wID+"/reopen",
		uuid.NewString(), `{"version":2}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, reopenPath, http.StatusOK, rec.Body.Bytes())
	m = jsonBody(t, rec.Body.Bytes())
	if m["mastered"] != false || m["severity"].(float64) != 1 {
		t.Fatalf("reopen 结果错误: %v", m)
	}
}

func TestProgressEndpoints(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID, lessonID, attemptID, _ := seed(t, pool)

	// 课程进度：快照已产生（掌握度 = 0.5*0.4 + 0.3*0 + 0.2*0.5 = 0.3），
	// completed_attempts=1、active_weaknesses=1、趋势含来源作答。
	rec := do(t, engine, http.MethodGet, "/api/v1/courses/"+courseID.String()+"/progress", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("课程进度 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, courseProgPath, http.StatusOK, rec.Body.Bytes())
	m := jsonBody(t, rec.Body.Bytes())
	if m["scope_type"] != "course" || m["mastery"] == nil ||
		m["formula_version"] != "mastery/v1" ||
		m["completed_attempts"].(float64) != 1 || m["active_weaknesses"].(float64) != 1 {
		t.Fatalf("课程进度错误: %v", m)
	}
	trend := m["trend"].([]any)
	if len(trend) != 1 || trend[0].(map[string]any)["source_attempt_id"] != attemptID.String() {
		t.Fatalf("趋势点错误: %v", trend)
	}

	// 课时进度：lesson 范围（含课程级薄弱点规则）。
	rec = do(t, engine, http.MethodGet, "/api/v1/lessons/"+lessonID.String()+"/progress", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("课时进度 status = %d", rec.Code)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, lessonProgPath, http.StatusOK, rec.Body.Bytes())
	m = jsonBody(t, rec.Body.Bytes())
	if m["scope_type"] != "lesson" || m["scope_id"] != lessonID.String() {
		t.Fatalf("课时进度错误: %v", m)
	}

	// 404：不存在的课程/课时。
	rec = do(t, engine, http.MethodGet, "/api/v1/courses/"+uuid.NewString()+"/progress", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课程 status = %d", rec.Code)
	}
	rec = do(t, engine, http.MethodGet, "/api/v1/lessons/"+uuid.NewString()+"/progress", "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d", rec.Code)
	}

	// 无数据范围：mastery/formula_version 为 null，计数为 0。
	otherCourse := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO courses (id, slug, title) VALUES ($1,$2,'空课程')`,
		otherCourse, "c-"+uuid.NewString()[:8]); err != nil {
		t.Fatal(err)
	}
	rec = do(t, engine, http.MethodGet, "/api/v1/courses/"+otherCourse.String()+"/progress", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("空课程进度 status = %d", rec.Code)
	}
	m = jsonBody(t, rec.Body.Bytes())
	if m["mastery"] != nil || m["formula_version"] != nil ||
		m["completed_attempts"].(float64) != 0 || len(m["trend"].([]any)) != 0 {
		t.Fatalf("空课程进度错误: %v", m)
	}
}
