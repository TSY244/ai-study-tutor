// Package quiz_test 是常规/复习试卷生成端点的 HTTP 集成测试：
// 真实 PostgreSQL + 与生产一致的 Gin 引擎，响应体以 api/openapi.yaml 校验。
package quiz_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

const (
	quizGenPath   = "/api/v1/lessons/{lesson_id}/quiz-generations"
	reviewGenPath = "/api/v1/lessons/{lesson_id}/review-quiz-generations"
)

// newEngine 装配默认策略的 GenerationService；需要注入策略的测试用 newEngineWithPolicy。
func newEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	return newEngineWithPolicy(t, pool, quiz.NewGenerationService(pool))
}

func newEngineWithPolicy(t *testing.T, pool *pgxpool.Pool, svc *quiz.GenerationService) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	h := quiz.NewGenerationHandler(svc, &httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	h.RegisterRoutes(e.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"))
	return e
}

func seedLesson(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	courseID, lessonID := uuid.New(), uuid.New()
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
	return lessonID
}

func jsonBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return m
}

func post(t *testing.T, engine *gin.Engine, path, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	return apitest.Do(t, engine, http.MethodPost, path,
		map[string]string{"Content-Type": "application/json", "Idempotency-Key": key}, body)
}

func jobPayload(t *testing.T, pool *pgxpool.Pool, jobID string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := pool.QueryRow(context.Background(),
		`SELECT payload FROM jobs WHERE id = $1`, jobID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func jobCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateQuizGeneration(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	// 202 + payload 正确（显式 question_count/title）。
	lessonID := seedLesson(t, pool)
	rec := post(t, engine, "/api/v1/lessons/"+lessonID.String()+"/quiz-generations",
		uuid.NewString(), `{"question_count":10,"title":"极限练习"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, quizGenPath, http.StatusAccepted, rec.Body.Bytes())
	j := jsonBody(t, rec)["job"].(map[string]any)
	if j["kind"] != "quiz_generate" || j["generation_family"] != "lesson_generation" ||
		j["status"] != "queued" || j["aggregate_id"] != lessonID.String() {
		t.Fatalf("job 字段错误: %v", j)
	}
	payload := jobPayload(t, pool, j["id"].(string))
	if payload["lesson_id"] != lessonID.String() || payload["question_count"].(float64) != 10 ||
		payload["title"] != "极限练习" {
		t.Fatalf("payload = %v", payload)
	}

	// 显式 null：question_count/title 序列化为 null 且被接受。
	lessonID2 := seedLesson(t, pool)
	rec = post(t, engine, "/api/v1/lessons/"+lessonID2.String()+"/quiz-generations",
		uuid.NewString(), `{"question_count":null,"title":null}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("null 字段 status = %d, body = %s", rec.Code, rec.Body)
	}
	payload = jobPayload(t, pool, jsonBody(t, rec)["job"].(map[string]any)["id"].(string))
	if v, ok := payload["question_count"]; !ok || v != nil {
		t.Fatalf("payload.question_count = %v，期望 null", payload["question_count"])
	}
	if v, ok := payload["title"]; !ok || v != nil {
		t.Fatalf("payload.title = %v，期望 null", payload["title"])
	}

	// 边界值 5/15 接受；4/16 拒绝（每个 202 用独立课时避免族冲突）。
	for _, tc := range []struct {
		count int
		want  int
	}{
		{5, http.StatusAccepted}, {15, http.StatusAccepted},
		{4, http.StatusUnprocessableEntity}, {16, http.StatusUnprocessableEntity},
	} {
		lid := seedLesson(t, pool)
		rec := post(t, engine, "/api/v1/lessons/"+lid.String()+"/quiz-generations",
			uuid.NewString(), questionCountBody(tc.count))
		if rec.Code != tc.want {
			t.Fatalf("question_count=%d status = %d，期望 %d", tc.count, rec.Code, tc.want)
		}
		if tc.want == http.StatusUnprocessableEntity {
			contract.ValidateProblem(t, rec.Body.Bytes(), tc.want, "validation_error")
		}
	}

	// title 超长 → 422。
	lid := seedLesson(t, pool)
	rec = post(t, engine, "/api/v1/lessons/"+lid.String()+"/quiz-generations",
		uuid.NewString(), `{"title":"`+strings.Repeat("题", 301)+`"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("title 超长 status = %d，期望 422", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")
}

// questionCountBody 构造 {"question_count":n}。
func questionCountBody(n int) string {
	b, _ := json.Marshal(map[string]int{"question_count": n})
	return string(b)
}

func TestCreateQuizGenerationPolicy(t *testing.T) {
	pool := databasetest.New(t)
	contract := apitest.Load(t)

	// 默认策略放行（独立于其他用例的课时）。
	engine := newEngine(t, pool)
	lid := seedLesson(t, pool)
	rec := post(t, engine, "/api/v1/lessons/"+lid.String()+"/quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("默认策略 status = %d, body = %s，期望 202", rec.Code, rec.Body)
	}

	// 注入 false 策略 → 422 practice_not_needed，不创建 Job。
	svc := quiz.NewGenerationService(pool)
	svc.QuizPolicy = func(context.Context, database.Querier, uuid.UUID) (bool, error) {
		return false, nil
	}
	engine = newEngineWithPolicy(t, pool, svc)
	before := jobCount(t, pool)
	lid2 := seedLesson(t, pool)
	rec = post(t, engine, "/api/v1/lessons/"+lid2.String()+"/quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("false 策略 status = %d, body = %s，期望 422", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "practice_not_needed")
	if got := jobCount(t, pool); got != before {
		t.Fatalf("策略拒绝后 jobs 数 = %d，期望 %d（不得创建任务）", got, before)
	}

	// 复习端点不受策略影响（同一个注入 false 策略的 service 仍放行复习）。
	rec = post(t, engine, "/api/v1/lessons/"+lid2.String()+"/review-quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("复习不受策略限制 status = %d, body = %s，期望 202", rec.Code, rec.Body)
	}
}

func TestCreateQuizGenerationConflictAndIdempotency(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	key := uuid.NewString()
	path := "/api/v1/lessons/" + lessonID.String() + "/quiz-generations"

	rec := post(t, engine, path, key, `{"question_count":8}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("首次 status = %d, body = %s", rec.Code, rec.Body)
	}
	jobID := jsonBody(t, rec)["job"].(map[string]any)["id"].(string)

	// 回放：同键同体 → 相同 job。
	rec = post(t, engine, path, key, `{"question_count":8}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("回放 status = %d, body = %s", rec.Code, rec.Body)
	}
	if got := jsonBody(t, rec)["job"].(map[string]any)["id"]; got != jobID {
		t.Fatalf("回放 job.id = %v，期望 %v", got, jobID)
	}
	if got := jobCount(t, pool); got != 1 {
		t.Fatalf("回放后 jobs 数 = %d，期望 1", got)
	}

	// 同键异体 → 409 idempotency_conflict。
	rec = post(t, engine, path, key, `{"question_count":9}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("异体冲突 status = %d，期望 409", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "idempotency_conflict")

	// 不同键 → 409 resource_busy（含 active_job_id 扩展）。
	rec = post(t, engine, path, uuid.NewString(), `{"question_count":9}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("族冲突 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
	m := jsonBody(t, rec)
	if m["active_job_id"] != jobID || m["generation_family"] != "lesson_generation" {
		t.Fatalf("resource_busy 扩展 = %v，期望 active_job_id=%s", m, jobID)
	}
}

func TestCreateReviewQuizGeneration(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	// 202 + payload 含 include_lesson_ids/weakness_ids。
	lessonID := seedLesson(t, pool)
	includeID, weaknessID := uuid.New(), uuid.New()
	key := uuid.NewString()
	body := `{"question_count":12,"include_lesson_ids":["` + includeID.String() +
		`"],"weakness_ids":["` + weaknessID.String() + `"]}`
	path := "/api/v1/lessons/" + lessonID.String() + "/review-quiz-generations"

	rec := post(t, engine, path, key, body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, reviewGenPath, http.StatusAccepted, rec.Body.Bytes())
	j := jsonBody(t, rec)["job"].(map[string]any)
	if j["kind"] != "review_quiz_generate" || j["generation_family"] != "lesson_generation" {
		t.Fatalf("job 字段错误: %v", j)
	}
	payload := jobPayload(t, pool, j["id"].(string))
	includes, _ := payload["include_lesson_ids"].([]any)
	weaknesses, _ := payload["weakness_ids"].([]any)
	if len(includes) != 1 || includes[0] != includeID.String() {
		t.Fatalf("payload.include_lesson_ids = %v", payload["include_lesson_ids"])
	}
	if len(weaknesses) != 1 || weaknesses[0] != weaknessID.String() {
		t.Fatalf("payload.weakness_ids = %v", payload["weakness_ids"])
	}
	if payload["question_count"].(float64) != 12 {
		t.Fatalf("payload.question_count = %v", payload["question_count"])
	}

	// 回放同键同体 → 相同 job。
	rec = post(t, engine, path, key, body)
	if rec.Code != http.StatusAccepted ||
		jsonBody(t, rec)["job"].(map[string]any)["id"] != j["id"] {
		t.Fatalf("回放结果错误: %d %s", rec.Code, rec.Body)
	}

	// 缺省字段：include_lesson_ids/weakness_ids 序列化为 [] 而非 null。
	lessonID2 := seedLesson(t, pool)
	rec = post(t, engine, "/api/v1/lessons/"+lessonID2.String()+"/review-quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("空体 status = %d, body = %s", rec.Code, rec.Body)
	}
	payload = jobPayload(t, pool, jsonBody(t, rec)["job"].(map[string]any)["id"].(string))
	for _, field := range []string{"include_lesson_ids", "weakness_ids"} {
		v, ok := payload[field]
		if !ok || v == nil {
			t.Fatalf("payload.%s = %v，期望 []", field, payload[field])
		}
		if arr, ok := v.([]any); !ok || len(arr) != 0 {
			t.Fatalf("payload.%s = %v，期望空数组", field, v)
		}
	}
	if v, ok := payload["question_count"]; !ok || v != nil {
		t.Fatalf("payload.question_count = %v，期望 null", payload["question_count"])
	}

	// question_count 越界 → 422。
	lessonID3 := seedLesson(t, pool)
	rec = post(t, engine, "/api/v1/lessons/"+lessonID3.String()+"/review-quiz-generations",
		uuid.NewString(), `{"question_count":20}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("越界 status = %d，期望 422", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")

	// 非法 UUID 数组元素 → 400（JSON 结构无效）。
	rec = post(t, engine, "/api/v1/lessons/"+lessonID3.String()+"/review-quiz-generations",
		uuid.NewString(), `{"include_lesson_ids":["not-a-uuid"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 UUID status = %d, body = %s，期望 400", rec.Code, rec.Body)
	}

	// 课时不存在 → 404。
	rec = post(t, engine, "/api/v1/lessons/"+uuid.NewString()+"/review-quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestQuizGenerationMaintenance(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	if _, err := pool.Exec(context.Background(),
		`UPDATE system_state SET maintenance_mode = true WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}
	rec := post(t, engine, "/api/v1/lessons/"+lessonID.String()+"/quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("维护模式 status = %d, body = %s，期望 503", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusServiceUnavailable, "maintenance_mode")
	if got := jobCount(t, pool); got != 0 {
		t.Fatalf("维护模式不得创建任务，jobs = %d", got)
	}
}
