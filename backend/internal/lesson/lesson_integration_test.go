// Package lesson_test 是课时 CRUD 端点的 HTTP 集成测试：
// 真实 PostgreSQL + 与生产一致的 Gin 引擎，响应体以 api/openapi.yaml 校验。
package lesson_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

const (
	lessonsPath = "/api/v1/courses/{course_id}/lessons"
	lessonPath  = "/api/v1/lessons/{lesson_id}"
)

func newEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	h := lesson.NewHandler(lesson.NewService(pool),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	h.RegisterRoutes(e.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"))
	return e
}

// seedCourse 直接落库课程，返回课程 ID。
func seedCourse(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	c := &course.Course{Slug: "c-" + uuid.NewString(), Title: "课程"}
	if err := (course.Repository{}).Create(context.Background(), pool, c); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

// createLesson 经 API 创建课时，返回解码后的 DTO map。
func createLesson(t *testing.T, e *gin.Engine, courseID uuid.UUID, key, body string) map[string]any {
	t.Helper()
	rec := apitest.Do(t, e, http.MethodPost,
		"/api/v1/courses/"+courseID.String()+"/lessons", map[string]string{
			"Content-Type": "application/json", "Idempotency-Key": key,
		}, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建课时失败：status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	return dto
}

func TestCreateLessonContract(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID := seedCourse(t, pool)

	rec := apitest.Do(t, e, http.MethodPost,
		"/api/v1/courses/"+courseID.String()+"/lessons", map[string]string{
			"Content-Type": "application/json", "Idempotency-Key": uuid.NewString(),
		}, `{"ordinal":1,"title":"极限"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodPost, lessonsPath, http.StatusCreated, rec.Body.Bytes())
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/api/v1/lessons/") {
		t.Fatalf("Location=%q", loc)
	}
	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto["status"] != "draft" || dto["description"] != "" || dto["version"] != float64(1) {
		t.Fatalf("新建课时字段不符: %v", dto)
	}
	if dto["active_job_id"] != nil || dto["mastery"] != nil {
		t.Fatalf("新建课时汇总字段应为 null: %v", dto)
	}
	docs, _ := dto["documents"].(map[string]any)
	if docs["raw_text"] != false || docs["student_notes"] != false || docs["teacher_outline"] != false {
		t.Fatalf("documents 可用性应全 false: %v", docs)
	}
}

func TestCreateLessonValidation(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID := seedCourse(t, pool)
	url := "/api/v1/courses/" + courseID.String() + "/lessons"
	hdr := func() map[string]string {
		return map[string]string{
			"Content-Type": "application/json", "Idempotency-Key": uuid.NewString(),
		}
	}

	// 缺 ordinal / 缺 title / ordinal 越界 → 422
	for _, body := range []string{
		`{"title":"x"}`,
		`{"ordinal":1}`,
		`{"ordinal":0,"title":"x"}`,
	} {
		rec := apitest.Do(t, e, http.MethodPost, url, hdr(), body)
		contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")
	}

	// ordinal 冲突 → 422 validation_error（field=ordinal, code=duplicate）
	createLesson(t, e, courseID, uuid.NewString(), `{"ordinal":7,"title":"甲"}`)
	rec := apitest.Do(t, e, http.MethodPost, url, hdr(), `{"ordinal":7,"title":"乙"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")
	var prob struct {
		Errors []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatal(err)
	}
	if len(prob.Errors) == 0 || prob.Errors[0].Field != "ordinal" || prob.Errors[0].Code != "duplicate" {
		t.Fatalf("ordinal 冲突字段错误不符: %+v", prob.Errors)
	}

	// 课程不存在 → 404
	rec = apitest.Do(t, e, http.MethodPost,
		"/api/v1/courses/"+uuid.NewString()+"/lessons", hdr(), `{"ordinal":1,"title":"x"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestCreateLessonIdempotency(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID := seedCourse(t, pool)
	url := "/api/v1/courses/" + courseID.String() + "/lessons"
	key := uuid.NewString()

	first := createLesson(t, e, courseID, key, `{"ordinal":1,"title":"回放"}`)
	second := createLesson(t, e, courseID, key, `{"ordinal":1,"title":"回放"}`)
	if first["id"] != second["id"] {
		t.Fatalf("幂等回放 id 不一致: %v vs %v", first["id"], second["id"])
	}

	// 相同 Key + 不同请求体 → 409 idempotency_conflict
	rec := apitest.Do(t, e, http.MethodPost, url, map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": key,
	}, `{"ordinal":2,"title":"不同"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "idempotency_conflict")
}

func TestListLessonsPagination(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID := seedCourse(t, pool)
	base := "/api/v1/courses/" + courseID.String() + "/lessons"

	for _, body := range []string{
		`{"ordinal":2,"title":"第二课"}`, `{"ordinal":1,"title":"第一课"}`, `{"ordinal":3,"title":"第三课"}`,
	} {
		createLesson(t, e, courseID, uuid.NewString(), body)
	}

	// 默认按 ordinal 升序
	rec := apitest.Do(t, e, http.MethodGet, base, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, lessonsPath, http.StatusOK, rec.Body.Bytes())
	var list struct {
		Items    []map[string]any `json:"items"`
		PageInfo struct {
			NextCursor *string `json:"next_cursor"`
			HasMore    bool    `json:"has_more"`
		} `json:"page_info"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 3 || list.PageInfo.HasMore {
		t.Fatalf("列表不符: items=%d has_more=%v", len(list.Items), list.PageInfo.HasMore)
	}
	for i, want := range []float64{1, 2, 3} {
		if list.Items[i]["ordinal"] != want {
			t.Fatalf("排序不符: %v", list.Items)
		}
	}

	// limit=2 翻页
	rec = apitest.Do(t, e, http.MethodGet, base+"?limit=2", nil, "")
	var page1 struct {
		Items    []map[string]any `json:"items"`
		PageInfo struct {
			NextCursor *string `json:"next_cursor"`
			HasMore    bool    `json:"has_more"`
		} `json:"page_info"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatal(err)
	}
	if len(page1.Items) != 2 || !page1.PageInfo.HasMore || page1.PageInfo.NextCursor == nil {
		t.Fatalf("第一页不符: %+v", page1.PageInfo)
	}
	rec = apitest.Do(t, e, http.MethodGet, base+"?limit=2&cursor="+*page1.PageInfo.NextCursor, nil, "")
	var page2 struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 1 || page2.Items[0]["ordinal"] != float64(3) {
		t.Fatalf("第二页不符: %v", page2.Items)
	}
}

func TestGetLessonEnrichment(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	ctx := context.Background()
	courseID := seedCourse(t, pool)
	created := createLesson(t, e, courseID, uuid.NewString(), `{"ordinal":1,"title":"富化"}`)
	lessonID := created["id"].(string)

	// 播种掌握度快照（80 → 0.8）与活动 lesson_generation 任务
	if _, err := pool.Exec(ctx, `
		INSERT INTO mastery_snapshots (id, scope_type, course_id, lesson_id, formula_version, value)
		VALUES ($1, 'lesson', $2, $3, 'v1', 80)`,
		uuid.New(), courseID, lessonID); err != nil {
		t.Fatal(err)
	}
	lid, _ := uuid.Parse(lessonID)
	fam := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindLessonImport,
		AggregateType:    "lesson",
		AggregateID:      lid,
		GenerationFamily: &fam,
		Payload:          []byte(`{"material_id":"` + uuid.NewString() + `"}`),
	}
	if err := (&job.Repository{}).Create(ctx, pool, j); err != nil {
		t.Fatal(err)
	}

	rec := apitest.Do(t, e, http.MethodGet, "/api/v1/lessons/"+lessonID, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, lessonPath, http.StatusOK, rec.Body.Bytes())
	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto["mastery"] != 0.8 {
		t.Fatalf("mastery=%v，期望 0.8", dto["mastery"])
	}
	if dto["active_job_id"] != j.ID.String() {
		t.Fatalf("active_job_id=%v，期望 %s", dto["active_job_id"], j.ID)
	}

	// 不存在 → 404；非法 UUID → 404（路径标识的资源不可能存在）
	rec = apitest.Do(t, e, http.MethodGet, "/api/v1/lessons/"+uuid.NewString(), nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	rec = apitest.Do(t, e, http.MethodGet, "/api/v1/lessons/not-a-uuid", nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestUpdateLesson(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID := seedCourse(t, pool)
	created := createLesson(t, e, courseID, uuid.NewString(), `{"ordinal":1,"title":"原题"}`)
	createLesson(t, e, courseID, uuid.NewString(), `{"ordinal":2,"title":"占位"}`)
	id := created["id"].(string)
	url := "/api/v1/lessons/" + id

	// 缺 version → 422
	rec := apitest.Do(t, e, http.MethodPatch, url,
		map[string]string{"Content-Type": "application/json"}, `{"title":"新题"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")

	// 正常更新
	rec = apitest.Do(t, e, http.MethodPatch, url,
		map[string]string{"Content-Type": "application/json"},
		`{"title":"新题","description":"d","version":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodPatch, lessonPath, http.StatusOK, rec.Body.Bytes())
	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto["title"] != "新题" || dto["description"] != "d" || dto["version"] != float64(2) {
		t.Fatalf("更新结果不符: %v", dto)
	}

	// 过期版本 → 409 version_conflict
	rec = apitest.Do(t, e, http.MethodPatch, url,
		map[string]string{"Content-Type": "application/json"}, `{"title":"并发","version":1}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "version_conflict")

	// ordinal 撞已有课时 → 422（field=ordinal）
	rec = apitest.Do(t, e, http.MethodPatch, url,
		map[string]string{"Content-Type": "application/json"}, `{"ordinal":2,"version":2}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")
}

func TestDeleteLesson(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	ctx := context.Background()
	courseID := seedCourse(t, pool)
	created := createLesson(t, e, courseID, uuid.NewString(), `{"ordinal":1,"title":"待删"}`)
	id := created["id"].(string)
	url := "/api/v1/lessons/" + id

	// 活动 lesson_generation 任务占用 → 409 resource_busy（含 active_job_id 扩展）
	lid, _ := uuid.Parse(id)
	fam := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindLessonImport,
		AggregateType:    "lesson",
		AggregateID:      lid,
		GenerationFamily: &fam,
		Payload:          []byte(`{}`),
	}
	if err := (&job.Repository{}).Create(ctx, pool, j); err != nil {
		t.Fatal(err)
	}
	rec := apitest.Do(t, e, http.MethodDelete, url, nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
	var busy map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &busy); err != nil {
		t.Fatal(err)
	}
	if busy["active_job_id"] != j.ID.String() {
		t.Fatalf("active_job_id 扩展缺失或不符: %v", busy)
	}

	// 任务结束后可删除
	if _, err := pool.Exec(ctx,
		`UPDATE jobs SET status = 'succeeded', completed_at = now() WHERE id = $1`, j.ID); err != nil {
		t.Fatal(err)
	}
	rec = apitest.Do(t, e, http.MethodDelete, url, nil, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = apitest.Do(t, e, http.MethodGet, url, nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
}

func TestDeleteLessonReferencedByQuiz(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	ctx := context.Background()
	courseID := seedCourse(t, pool)
	created := createLesson(t, e, courseID, uuid.NewString(), `{"ordinal":1,"title":"被引用"}`)
	id := created["id"].(string)

	// 播种引用该课时的试卷（保护作答历史，禁止删除）
	if _, err := pool.Exec(ctx,
		`INSERT INTO quizzes (id, lesson_id, title, source) VALUES ($1,$2,'试卷','regular')`,
		uuid.New(), id); err != nil {
		t.Fatal(err)
	}
	rec := apitest.Do(t, e, http.MethodDelete, "/api/v1/lessons/"+id, nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
}
