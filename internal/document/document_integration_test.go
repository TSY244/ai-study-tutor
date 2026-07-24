// Package document_test 是文档查询与重生成端点的 HTTP 集成测试：
// 真实 PostgreSQL + 与生产一致的 Gin 引擎，响应体以 api/openapi.yaml 校验。
package document_test

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

	"github.com/TSY244/ai-study-tutor/internal/apitest"
	"github.com/TSY244/ai-study-tutor/internal/document"
	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

const (
	documentsPath  = "/api/v1/lessons/{lesson_id}/documents"
	documentKind   = "/api/v1/lessons/{lesson_id}/documents/{kind}"
	versionsPath   = "/api/v1/lessons/{lesson_id}/documents/{kind}/versions"
	documentByID   = "/api/v1/documents/{document_id}"
	regeneratePath = "/api/v1/lessons/{lesson_id}/regenerations"
)

func newEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	h := document.NewHandler(document.NewService(pool),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	h.RegisterRoutes(e.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"))
	return e
}

// seedLesson 直接落库课程+课时（查询端点只依赖 lessons/documents 两表）。
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

// seedVersion 创建一个文档版本并按需发布。
func seedVersion(t *testing.T, pool *pgxpool.Pool, lessonID uuid.UUID, kind, content string, publish bool) *document.Document {
	t.Helper()
	d := &document.Document{LessonID: lessonID, Kind: kind, ContentMD: content}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	repo := document.Repository{}
	if err := repo.CreateVersion(context.Background(), tx, d); err != nil {
		t.Fatal(err)
	}
	if publish {
		if err := repo.Publish(context.Background(), tx, d.ID); err != nil {
			t.Fatal(err)
		}
		d.IsActive = true
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return d
}

func jsonBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return m
}

func TestListCurrentDocuments(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	// raw_text 已发布；student_notes 有旧版本（未发布）与新版本（已发布）。
	seedVersion(t, pool, lessonID, "raw_text", "# 原文", true)
	seedVersion(t, pool, lessonID, "student_notes", "# 笔记 v1", false)
	seedVersion(t, pool, lessonID, "student_notes", "# 笔记 v2", true)

	path := "/api/v1/lessons/" + lessonID.String() + "/documents"
	rec := apitest.Do(t, engine, http.MethodGet, path, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, documentsPath, http.StatusOK, rec.Body.Bytes())

	m := jsonBody(t, rec)
	if _, hasPageInfo := m["page_info"]; hasPageInfo {
		t.Fatal("非分页集合不得包含 page_info")
	}
	items, _ := m["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items 数 = %d，期望 2（未发布版本不得出现）", len(items))
	}
	// 按 kind 字典序：raw_text 在 student_notes 前。
	first, _ := items[0].(map[string]any)
	second, _ := items[1].(map[string]any)
	if first["kind"] != "raw_text" || second["kind"] != "student_notes" {
		t.Fatalf("items 顺序 = %v/%v，期望 raw_text/student_notes", first["kind"], second["kind"])
	}
	if second["revision"].(float64) != 2 {
		t.Fatalf("student_notes revision = %v，期望 2（当前发布版本）", second["revision"])
	}
	for _, it := range []map[string]any{first, second} {
		if it["is_active"] != true {
			t.Fatalf("items 必须全部为 active: %v", it)
		}
		if _, hasContent := it["content_md"]; hasContent {
			t.Fatal("摘要不得包含 content_md")
		}
	}

	// 空课时 → 200 items: []（非 null）。
	emptyLesson := seedLesson(t, pool)
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+emptyLesson.String()+"/documents", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("空课时 status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("空集合 items 必须是 []: %s", rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, documentsPath, http.StatusOK, rec.Body.Bytes())

	// 课时不存在 → 404。
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+uuid.NewString()+"/documents", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestGetCurrentDocument(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	seedVersion(t, pool, lessonID, "student_notes", "# 旧版", false)
	active := seedVersion(t, pool, lessonID, "student_notes", "# 当前版", true)

	rec := apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+lessonID.String()+"/documents/student_notes", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, documentKind, http.StatusOK, rec.Body.Bytes())
	m := jsonBody(t, rec)
	if m["id"] != active.ID.String() || m["content_md"] != "# 当前版" {
		t.Fatalf("返回版本错误: %v", m)
	}
	if m["source_job_id"] != nil {
		t.Fatalf("source_job_id 应为 null: %v", m["source_job_id"])
	}

	// lesson 存在但该 kind 无 active → 404。
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+lessonID.String()+"/documents/teacher_outline", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("无 active status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")

	// kind 非法 → 404。
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+lessonID.String()+"/documents/bogus", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("非法 kind status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")

	// lesson 不存在 → 404。
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+uuid.NewString()+"/documents/student_notes", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d，期望 404", rec.Code)
	}
}

func TestListDocumentVersions(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	seedVersion(t, pool, lessonID, "student_notes", "# v1", false)
	seedVersion(t, pool, lessonID, "student_notes", "# v2", false)
	seedVersion(t, pool, lessonID, "student_notes", "# v3", true)

	base := "/api/v1/lessons/" + lessonID.String() + "/documents/student_notes/versions"

	// 第一页 limit=2：revision DESC [3,2]，has_more=true。
	rec := apitest.Do(t, engine, http.MethodGet, base+"?limit=2", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, versionsPath, http.StatusOK, rec.Body.Bytes())
	m := jsonBody(t, rec)
	items, _ := m["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("第一页 items = %d，期望 2", len(items))
	}
	revs := []float64{items[0].(map[string]any)["revision"].(float64),
		items[1].(map[string]any)["revision"].(float64)}
	if revs[0] != 3 || revs[1] != 2 {
		t.Fatalf("revision 顺序 = %v，期望 [3 2]", revs)
	}
	pageInfo, _ := m["page_info"].(map[string]any)
	if pageInfo["has_more"] != true || pageInfo["next_cursor"] == nil {
		t.Fatalf("page_info = %v，期望 has_more=true 且 next_cursor 非空", pageInfo)
	}
	cursor := pageInfo["next_cursor"].(string)

	// 第二页：[1]，has_more=false，next_cursor=null。
	rec = apitest.Do(t, engine, http.MethodGet, base+"?limit=2&cursor="+cursor, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("第二页 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, versionsPath, http.StatusOK, rec.Body.Bytes())
	m = jsonBody(t, rec)
	items, _ = m["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["revision"].(float64) != 1 {
		t.Fatalf("第二页 items = %v，期望 [revision=1]", items)
	}
	pageInfo, _ = m["page_info"].(map[string]any)
	if pageInfo["has_more"] != false || pageInfo["next_cursor"] != nil {
		t.Fatalf("末页 page_info = %v，期望 has_more=false 且 next_cursor=null", pageInfo)
	}

	// 默认 limit（20）单页全量。
	rec = apitest.Do(t, engine, http.MethodGet, base, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("默认分页 status = %d", rec.Code)
	}
	m = jsonBody(t, rec)
	if items, _ := m["items"].([]any); len(items) != 3 {
		t.Fatalf("默认分页 items = %d，期望 3", len(items))
	}

	// 非法 cursor / limit → 400。
	for _, q := range []string{"?cursor=not-a-cursor", "?limit=0", "?limit=abc", "?limit=101"} {
		rec = apitest.Do(t, engine, http.MethodGet, base+q, nil, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d，期望 400", q, rec.Code)
		}
		contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	}

	// 非法 kind / 不存在课时 → 404。
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+lessonID.String()+"/documents/bogus/versions", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("非法 kind status = %d，期望 404", rec.Code)
	}
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+uuid.NewString()+"/documents/student_notes/versions", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d，期望 404", rec.Code)
	}
}

func TestGetDocumentVersion(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)
	old := seedVersion(t, pool, lessonID, "student_notes", "# 旧版", false)
	seedVersion(t, pool, lessonID, "student_notes", "# 当前版", true)

	// 旧版本（is_active=false）也可按 ID 读取，含 content_md。
	rec := apitest.Do(t, engine, http.MethodGet, "/api/v1/documents/"+old.ID.String(), nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, documentByID, http.StatusOK, rec.Body.Bytes())
	m := jsonBody(t, rec)
	if m["content_md"] != "# 旧版" || m["is_active"] != false || m["revision"].(float64) != 1 {
		t.Fatalf("旧版本内容错误: %v", m)
	}

	// 不存在 → 404；非法 UUID → 404。
	rec = apitest.Do(t, engine, http.MethodGet, "/api/v1/documents/"+uuid.NewString(), nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在文档 status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	rec = apitest.Do(t, engine, http.MethodGet, "/api/v1/documents/not-a-uuid", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("非法 UUID status = %d，期望 404", rec.Code)
	}
}

// postRegeneration 携带幂等键发起重生成请求。
func postRegeneration(t *testing.T, engine *gin.Engine, lessonID uuid.UUID, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	return apitest.Do(t, engine, http.MethodPost,
		"/api/v1/lessons/"+lessonID.String()+"/regenerations",
		map[string]string{"Content-Type": "application/json", "Idempotency-Key": key}, body)
}

func TestCreateLessonRegeneration(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	ctx := context.Background()

	lessonID := seedLesson(t, pool)
	key := uuid.NewString()
	body := `{"kinds":["teacher_outline","student_notes","student_notes"],"reason":"内容过时"}`

	rec := postRegeneration(t, engine, lessonID, key, body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, regeneratePath, http.StatusAccepted, rec.Body.Bytes())
	m := jsonBody(t, rec)
	j, _ := m["job"].(map[string]any)
	if j["kind"] != "lesson_regenerate" || j["generation_family"] != "lesson_generation" ||
		j["status"] != "queued" || j["aggregate_id"] != lessonID.String() {
		t.Fatalf("job 字段错误: %v", j)
	}
	jobID := j["id"].(string)

	// payload：kinds 去重且保序，reason/lesson_id 正确。
	var payload map[string]any
	if err := pool.QueryRow(ctx,
		`SELECT payload FROM jobs WHERE id = $1`, jobID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	kinds, _ := payload["kinds"].([]any)
	if len(kinds) != 2 || kinds[0] != "teacher_outline" || kinds[1] != "student_notes" {
		t.Fatalf("payload.kinds = %v，期望 [teacher_outline student_notes]", payload["kinds"])
	}
	if payload["reason"] != "内容过时" || payload["lesson_id"] != lessonID.String() {
		t.Fatalf("payload = %v", payload)
	}

	// 幂等回放：同键同体 → 相同 job，不产生第二个任务。
	rec = postRegeneration(t, engine, lessonID, key, body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("回放 status = %d, body = %s", rec.Code, rec.Body)
	}
	if got := jsonBody(t, rec)["job"].(map[string]any)["id"]; got != jobID {
		t.Fatalf("回放 job.id = %v，期望 %v", got, jobID)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("jobs 数 = %d，期望 1（回放不得创建新任务）", count)
	}

	// 同键异体 → 409 idempotency_conflict。
	rec = postRegeneration(t, engine, lessonID, key, `{"kinds":["student_notes"]}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("异体冲突 status = %d，期望 409", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "idempotency_conflict")

	// 不同幂等键 → 族唯一冲突 409 resource_busy（含 active_job_id 扩展）。
	rec = postRegeneration(t, engine, lessonID, uuid.NewString(), `{"kinds":["student_notes"]}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("族冲突 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
	m = jsonBody(t, rec)
	if m["active_job_id"] != jobID || m["generation_family"] != "lesson_generation" {
		t.Fatalf("resource_busy 扩展 = %v，期望 active_job_id=%s", m, jobID)
	}
}

func TestCreateLessonRegenerationValidation(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)

	lessonID := seedLesson(t, pool)

	cases := []struct {
		name string
		body string
	}{
		{"kinds 缺失", `{"reason":"x"}`},
		{"kinds 空数组", `{"kinds":[]}`},
		{"kinds 非法枚举", `{"kinds":["raw_text"]}`},
		{"kinds 混合非法", `{"kinds":["student_notes","bogus"]}`},
		{"reason 超长", `{"kinds":["student_notes"],"reason":"` + strings.Repeat("长", 1001) + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postRegeneration(t, engine, lessonID, uuid.NewString(), tc.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, body = %s，期望 422", rec.Code, rec.Body)
			}
			contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")
		})
	}

	// 畸形 JSON / 空体 / 未知字段 → 400 bad_request。
	for _, body := range []string{`{`, ``, `{"kinds":["student_notes"],"extra":1}`} {
		rec := postRegeneration(t, engine, lessonID, uuid.NewString(), body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body=%q status = %d，期望 400", body, rec.Code)
		}
		contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	}

	// 缺失/非法 Idempotency-Key → 400。
	rec := apitest.Do(t, engine, http.MethodPost,
		"/api/v1/lessons/"+lessonID.String()+"/regenerations",
		map[string]string{"Content-Type": "application/json"}, `{"kinds":["student_notes"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺失幂等键 status = %d，期望 400", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	rec = postRegeneration(t, engine, lessonID, "not-a-uuid", `{"kinds":["student_notes"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法幂等键 status = %d，期望 400", rec.Code)
	}

	// 课时不存在 → 404。
	rec = postRegeneration(t, engine, uuid.New(), uuid.NewString(), `{"kinds":["student_notes"]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d，期望 404", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestCreateLessonRegenerationMaintenance(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	ctx := context.Background()

	lessonID := seedLesson(t, pool)
	if _, err := pool.Exec(ctx,
		`UPDATE system_state SET maintenance_mode = true WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}
	rec := postRegeneration(t, engine, lessonID, uuid.NewString(), `{"kinds":["student_notes"]}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("维护模式 status = %d, body = %s，期望 503", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusServiceUnavailable, "maintenance_mode")

	// 维护中的失败不得留下幂等完成记录，也不得有任务落库。
	var jobs, records int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jobs`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM idempotency_records WHERE state = 'completed'`).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if jobs != 0 || records != 0 {
		t.Fatalf("维护模式不得落库: jobs=%d completed_records=%d", jobs, records)
	}
}

// 确保 repository 新方法不破坏既有事务组合用法（domaintest 已覆盖基本约束）。
func TestRepositoryListMethodsInTx(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	lessonID := seedLesson(t, pool)
	seedVersion(t, pool, lessonID, "raw_text", "# 原文", true)
	seedVersion(t, pool, lessonID, "student_notes", "# v1", false)
	seedVersion(t, pool, lessonID, "student_notes", "# v2", true)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo := document.Repository{}
	active, err := repo.ListActiveByLesson(ctx, tx, lessonID)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 || active[0].Kind != "raw_text" || active[1].Kind != "student_notes" {
		t.Fatalf("ListActiveByLesson = %+v", active)
	}
	vers, err := repo.ListVersions(ctx, tx, lessonID, "student_notes", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(vers) != 2 || vers[0].Revision != 2 || vers[1].Revision != 1 {
		t.Fatalf("ListVersions = %+v", vers)
	}
	before := int32(2)
	vers, err = repo.ListVersions(ctx, tx, lessonID, "student_notes", &before, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(vers) != 1 || vers[0].Revision != 1 {
		t.Fatalf("ListVersions(游标) = %+v", vers)
	}
}
