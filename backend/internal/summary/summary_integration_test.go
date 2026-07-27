// Package summary_test 是学习总结评估端点的 HTTP 集成测试：
// 真实 PostgreSQL + 与生产一致的 Gin 引擎，响应体以 api/openapi.yaml 校验。
package summary_test

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
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
	"github.com/TSY244/ai-study-tutor/backend/internal/summary"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

const (
	createPath = "/api/v1/lessons/{lesson_id}/summary-evaluations"
	getPath    = "/api/v1/summary-evaluations/{evaluation_id}"
)

func newEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	h := summary.NewHandler(summary.NewService(pool), &httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
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

func get(t *testing.T, engine *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	return apitest.Do(t, engine, http.MethodGet, path, nil, "")
}

// createOK 创建一个评估并断言 202，返回 (evaluationID, jobID)。
func createOK(t *testing.T, engine *gin.Engine, lessonID uuid.UUID, key, body string) (string, string) {
	t.Helper()
	rec := post(t, engine, "/api/v1/lessons/"+lessonID.String()+"/summary-evaluations", key, body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("创建 status = %d, body = %s，期望 202", rec.Code, rec.Body)
	}
	m := jsonBody(t, rec)
	return m["evaluation"].(map[string]any)["id"].(string),
		m["job"].(map[string]any)["id"].(string)
}

func TestCreateSummaryEvaluation(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	rec := post(t, engine, "/api/v1/lessons/"+lessonID.String()+"/summary-evaluations",
		uuid.NewString(), `{"summary":"极限是趋势"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, createPath, http.StatusAccepted, rec.Body.Bytes())

	m := jsonBody(t, rec)
	ev := m["evaluation"].(map[string]any)
	j := m["job"].(map[string]any)

	// 评估：创建即 evaluating（表无 queued），coverage_score 为 null、四数组为 []。
	if ev["lesson_id"] != lessonID.String() || ev["status"] != "evaluating" {
		t.Fatalf("evaluation 字段错误: %v", ev)
	}
	if v, ok := ev["coverage_score"]; !ok || v != nil {
		t.Fatalf("coverage_score = %v，期望 null", ev["coverage_score"])
	}
	for _, field := range []string{"correct_points", "misconceptions", "missing_points", "suggestions"} {
		arr, ok := ev[field].([]any)
		if !ok || len(arr) != 0 {
			t.Fatalf("%s = %v，期望 []", field, ev[field])
		}
	}
	if v, ok := ev["error"]; !ok || v != nil {
		t.Fatalf("error = %v，期望 null", ev["error"])
	}
	if v, ok := ev["completed_at"]; !ok || v != nil {
		t.Fatalf("completed_at = %v，期望 null", ev["completed_at"])
	}
	if ev["created_at"] == nil || ev["created_at"] == "" {
		t.Fatalf("created_at 缺失: %v", ev)
	}

	// 任务：kind/aggregate 指向评估，不属任何生成族。
	if j["kind"] != "summary_evaluate" || j["status"] != "queued" ||
		j["aggregate_type"] != "summary_evaluation" || j["aggregate_id"] != ev["id"] {
		t.Fatalf("job 字段错误: %v", j)
	}
	if v, ok := j["generation_family"]; !ok || v != nil {
		t.Fatalf("generation_family = %v，期望 null", j["generation_family"])
	}

	// 落库：评估行与 Job payload 三件套快照。
	var status, summaryText string
	var jobID string
	if err := pool.QueryRow(context.Background(),
		`SELECT status, summary_text, job_id::text FROM summary_evaluations WHERE id = $1`,
		ev["id"].(string)).Scan(&status, &summaryText, &jobID); err != nil {
		t.Fatal(err)
	}
	if status != "evaluating" || summaryText != "极限是趋势" || jobID != j["id"].(string) {
		t.Fatalf("评估行错误: status=%s summary=%q job_id=%s", status, summaryText, jobID)
	}
	var payload map[string]any
	if err := pool.QueryRow(context.Background(),
		`SELECT payload FROM jobs WHERE id = $1`, j["id"].(string)).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["evaluation_id"] != ev["id"] || payload["lesson_id"] != lessonID.String() ||
		payload["summary"] != "极限是趋势" {
		t.Fatalf("payload = %v", payload)
	}

	// 边界：50000 字符接受。
	lid := seedLesson(t, pool)
	rec = post(t, engine, "/api/v1/lessons/"+lid.String()+"/summary-evaluations",
		uuid.NewString(), `{"summary":"`+strings.Repeat("总", 50000)+`"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("50000 字符 status = %d, body = %s，期望 202", rec.Code, rec.Body)
	}
}

func TestCreateSummaryEvaluationIdempotency(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	key := uuid.NewString()
	path := "/api/v1/lessons/" + lessonID.String() + "/summary-evaluations"

	evalID, jobID := createOK(t, engine, lessonID, key, `{"summary":"第一版总结"}`)

	// 回放：同键同体 → 相同 evaluation/job，不产生新行。
	rec := post(t, engine, path, key, `{"summary":"第一版总结"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("回放 status = %d, body = %s", rec.Code, rec.Body)
	}
	m := jsonBody(t, rec)
	if got := m["evaluation"].(map[string]any)["id"]; got != evalID {
		t.Fatalf("回放 evaluation.id = %v，期望 %v", got, evalID)
	}
	if got := m["job"].(map[string]any)["id"]; got != jobID {
		t.Fatalf("回放 job.id = %v，期望 %v", got, jobID)
	}
	var evalCount, jobCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM summary_evaluations`).Scan(&evalCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs`).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if evalCount != 1 || jobCount != 1 {
		t.Fatalf("回放后行数 evaluations=%d jobs=%d，期望各 1", evalCount, jobCount)
	}

	// 同键异体 → 409 idempotency_conflict。
	rec = post(t, engine, path, key, `{"summary":"另一版总结"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("异体冲突 status = %d，期望 409", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "idempotency_conflict")

	// 不同键 → 正常创建（总结评估无互斥约束）。
	if _, newJobID := createOK(t, engine, lessonID, uuid.NewString(), `{"summary":"另一版总结"}`); newJobID == jobID {
		t.Fatal("不同键应创建新任务")
	}
}

func TestCreateSummaryEvaluationValidation(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	path := "/api/v1/lessons/" + lessonID.String() + "/summary-evaluations"

	// 缺 summary / 空 summary / 超长 → 422 validation_error。
	for _, body := range []string{
		`{}`,
		`{"summary":""}`,
		`{"summary":"` + strings.Repeat("总", 50001) + `"}`,
	} {
		rec := post(t, engine, path, uuid.NewString(), body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("body %.40s status = %d，期望 422", body, rec.Code)
		}
		contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")
	}

	// 未知字段 / 畸形 JSON → 400 bad_request（DisallowUnknownFields）。
	for _, body := range []string{`{"summary":"ok","extra":1}`, `{"summary":`} {
		rec := post(t, engine, path, uuid.NewString(), body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %.40s status = %d，期望 400", body, rec.Code)
		}
		contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	}
}

func TestCreateSummaryEvaluationNotFoundAndMissingKey(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	// 课时不存在 → 404 not_found。
	rec := post(t, engine, "/api/v1/lessons/"+uuid.NewString()+"/summary-evaluations",
		uuid.NewString(), `{"summary":"总结"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")

	// 非法课时 UUID → 404。
	rec = post(t, engine, "/api/v1/lessons/not-a-uuid/summary-evaluations",
		uuid.NewString(), `{"summary":"总结"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("非法 UUID status = %d，期望 404", rec.Code)
	}

	// 缺 Idempotency-Key / 非 UUID → 400 bad_request。
	lessonID := seedLesson(t, pool)
	path := "/api/v1/lessons/" + lessonID.String() + "/summary-evaluations"
	rec = apitest.Do(t, engine, http.MethodPost, path,
		map[string]string{"Content-Type": "application/json"}, `{"summary":"总结"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺幂等键 status = %d，期望 400", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	rec = post(t, engine, path, "not-a-uuid", `{"summary":"总结"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法幂等键 status = %d，期望 400", rec.Code)
	}
}

func TestGetSummaryEvaluation(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	repo := summary.Repository{}
	ctx := context.Background()

	// 进行中：evaluating + null 分数 + 空数组。
	lessonID := seedLesson(t, pool)
	evalID, _ := createOK(t, engine, lessonID, uuid.NewString(), `{"summary":"第一版"}`)
	rec := get(t, engine, "/api/v1/summary-evaluations/"+evalID)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, getPath, http.StatusOK, rec.Body.Bytes())
	if got := jsonBody(t, rec)["status"]; got != "evaluating" {
		t.Fatalf("进行中 status = %v，期望 evaluating", got)
	}

	// 完成：worker 写入五字段 → succeeded + coverage_score + 四数组。
	result := json.RawMessage(`{
		"coverage_score":0.8,
		"correct_points":["趋势"],
		"misconceptions":[],
		"missing_points":["严谨定义"],
		"suggestions":["补充 epsilon-delta"]
	}`)
	if err := repo.Complete(ctx, pool, uuid.MustParse(evalID), result); err != nil {
		t.Fatal(err)
	}
	rec = get(t, engine, "/api/v1/summary-evaluations/"+evalID)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET completed status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, getPath, http.StatusOK, rec.Body.Bytes())
	ev := jsonBody(t, rec)
	if ev["status"] != "succeeded" {
		t.Fatalf("completed 映射 = %v，期望 succeeded", ev["status"])
	}
	if ev["coverage_score"].(float64) != 0.8 {
		t.Fatalf("coverage_score = %v，期望 0.8", ev["coverage_score"])
	}
	if pts := ev["correct_points"].([]any); len(pts) != 1 || pts[0] != "趋势" {
		t.Fatalf("correct_points = %v", ev["correct_points"])
	}
	if pts := ev["missing_points"].([]any); len(pts) != 1 || pts[0] != "严谨定义" {
		t.Fatalf("missing_points = %v", ev["missing_points"])
	}
	if pts := ev["suggestions"].([]any); len(pts) != 1 || pts[0] != "补充 epsilon-delta" {
		t.Fatalf("suggestions = %v", ev["suggestions"])
	}
	if arr := ev["misconceptions"].([]any); len(arr) != 0 {
		t.Fatalf("misconceptions = %v，期望 []", ev["misconceptions"])
	}
	if v, ok := ev["error"]; !ok || v != nil {
		t.Fatalf("error = %v，期望 null", ev["error"])
	}
	if v, _ := ev["completed_at"].(string); v == "" {
		t.Fatalf("completed_at 应为时间字符串: %v", ev["completed_at"])
	}

	// 失败：worker 终态同事务写 jobs 错误列 → error 取自 jobs（detail/retryable）。
	evalID2, jobID2 := createOK(t, engine, lessonID, uuid.NewString(), `{"summary":"第二版"}`)
	if err := repo.Fail(ctx, pool, uuid.MustParse(evalID2), "claude_timeout"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE jobs SET status='failed', error_code='claude_timeout',
			error_detail='Claude 调用超时', error_retryable=true, completed_at=now()
		WHERE id = $1`, jobID2); err != nil {
		t.Fatal(err)
	}
	rec = get(t, engine, "/api/v1/summary-evaluations/"+evalID2)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET failed status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, getPath, http.StatusOK, rec.Body.Bytes())
	ev = jsonBody(t, rec)
	if ev["status"] != "failed" {
		t.Fatalf("failed 映射 = %v，期望 failed", ev["status"])
	}
	e := ev["error"].(map[string]any)
	if e["code"] != "claude_timeout" || e["detail"] != "Claude 调用超时" || e["retryable"] != true {
		t.Fatalf("error = %v", e)
	}
	if v, ok := ev["coverage_score"]; !ok || v != nil {
		t.Fatalf("failed coverage_score = %v，期望 null", ev["coverage_score"])
	}

	// 失败但 jobs 无错误列（如取消路径）→ 回退 result.code 与错误码分类。
	evalID3, _ := createOK(t, engine, lessonID, uuid.NewString(), `{"summary":"第三版"}`)
	if err := repo.Fail(ctx, pool, uuid.MustParse(evalID3), "cancelled"); err != nil {
		t.Fatal(err)
	}
	rec = get(t, engine, "/api/v1/summary-evaluations/"+evalID3)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET cancelled status = %d, body = %s", rec.Code, rec.Body)
	}
	ev = jsonBody(t, rec)
	e = ev["error"].(map[string]any)
	if e["code"] != "cancelled" || e["detail"] != "" || e["retryable"] != false {
		t.Fatalf("回退 error = %v", e)
	}

	// 不存在 / 非法 UUID → 404 not_found。
	rec = get(t, engine, "/api/v1/summary-evaluations/"+uuid.NewString())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在 status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	rec = get(t, engine, "/api/v1/summary-evaluations/not-a-uuid")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("非法 UUID status = %d，期望 404", rec.Code)
	}
}
