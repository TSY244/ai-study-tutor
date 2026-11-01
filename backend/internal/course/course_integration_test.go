// Package course_test 是课程 CRUD 端点的 HTTP 集成测试：
// 真实 PostgreSQL + 与生产一致的 Gin 引擎，响应体以 api/openapi.yaml 校验。
package course_test

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

const coursesPath = "/api/v1/courses"

func newEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	h := course.NewHandler(course.NewService(pool),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	h.RegisterRoutes(e.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"))
	return e
}

// createCourse 经 API 创建课程，返回解码后的 DTO map。
func createCourse(t *testing.T, e *gin.Engine, key, body string) map[string]any {
	t.Helper()
	rec := apitest.Do(t, e, http.MethodPost, coursesPath, map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": key,
	}, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建课程失败：status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	return dto
}

func TestCreateCourseContract(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)

	rec := apitest.Do(t, e, http.MethodPost, coursesPath, map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": uuid.NewString(),
	}, `{"title":"数学分析","description":"大一上"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodPost, coursesPath, http.StatusCreated, rec.Body.Bytes())
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/api/v1/courses/") {
		t.Fatalf("Location=%q", loc)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("缺少 X-Request-ID 响应头")
	}
	var dto map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto["archived"] != false || dto["lesson_count"] != float64(0) || dto["version"] != float64(1) {
		t.Fatalf("新建课程字段不符: %v", dto)
	}
}

func TestCreateCourseIdempotency(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)

	// 缺少 Idempotency-Key → 400 bad_request
	rec := apitest.Do(t, e, http.MethodPost, coursesPath, map[string]string{
		"Content-Type": "application/json",
	}, `{"title":"A"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")

	// 相同 Key + 相同请求体 → 回放 201，id 一致且不新增记录
	key := uuid.NewString()
	first := createCourse(t, e, key, `{"title":"回放课程"}`)
	rec = apitest.Do(t, e, http.MethodPost, coursesPath, map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": key,
	}, `{"title":"回放课程"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("回放 status=%d body=%s", rec.Code, rec.Body.String())
	}
	second := createCourse(t, e, key, `{"title":"回放课程"}`)
	if first["id"] != second["id"] {
		t.Fatalf("幂等回放 id 不一致: %v vs %v", first["id"], second["id"])
	}

	// 相同 Key + 不同请求体 → 409 idempotency_conflict
	rec = apitest.Do(t, e, http.MethodPost, coursesPath, map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": key,
	}, `{"title":"别的课程"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "idempotency_conflict")
}

func TestCreateCourseValidation(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	hdr := map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": uuid.NewString(),
	}

	// 未填写标题可创建，等待 AI 命名。
	rec := apitest.Do(t, e, http.MethodPost, coursesPath, hdr, `{"description":"x"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("optional title: %d %s", rec.Code, rec.Body.String())
	}

	// title 超长 → 422
	rec = apitest.Do(t, e, http.MethodPost, coursesPath, hdr, `{"title":"`+strings.Repeat("长", 201)+`"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")

	// 未知字段（additionalProperties:false）→ 400 bad_request
	rec = apitest.Do(t, e, http.MethodPost, coursesPath, hdr, `{"title":"x","slug":"hack"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")

	// 非法 Idempotency-Key → 400
	rec = apitest.Do(t, e, http.MethodPost, coursesPath, map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": "not-a-uuid",
	}, `{"title":"x"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
}

func TestListCoursesPagination(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)

	for _, title := range []string{"课程甲", "课程乙", "课程丙"} {
		createCourse(t, e, uuid.NewString(), `{"title":"`+title+`"}`)
	}

	// 第一页 limit=2：has_more=true 且有 next_cursor
	rec := apitest.Do(t, e, http.MethodGet, coursesPath+"?limit=2", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, coursesPath, http.StatusOK, rec.Body.Bytes())
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
		t.Fatalf("第一页不符: items=%d has_more=%v", len(page1.Items), page1.PageInfo.HasMore)
	}

	// 第二页经游标：剩余 1 条且无重复
	rec = apitest.Do(t, e, http.MethodGet,
		coursesPath+"?limit=2&cursor="+*page1.PageInfo.NextCursor, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var page2 struct {
		Items    []map[string]any `json:"items"`
		PageInfo struct {
			HasMore bool `json:"has_more"`
		} `json:"page_info"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 1 || page2.PageInfo.HasMore {
		t.Fatalf("第二页不符: items=%d has_more=%v", len(page2.Items), page2.PageInfo.HasMore)
	}
	seen := map[string]bool{}
	for _, it := range append(page1.Items, page2.Items...) {
		id := it["id"].(string)
		if seen[id] {
			t.Fatalf("翻页出现重复课程 %s", id)
		}
		seen[id] = true
	}

	// 非法游标 → 400
	rec = apitest.Do(t, e, http.MethodGet, coursesPath+"?cursor=!!!", nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	// 非法 limit → 400
	rec = apitest.Do(t, e, http.MethodGet, coursesPath+"?limit=0", nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
}

func TestGetUpdateDeleteCourse(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	coursePath := coursesPath + "/{course_id}"

	created := createCourse(t, e, uuid.NewString(), `{"title":"原题"}`)
	id := created["id"].(string)

	// GET 详情
	rec := apitest.Do(t, e, http.MethodGet, coursesPath+"/"+id, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, coursePath, http.StatusOK, rec.Body.Bytes())

	// PATCH 缺 version → 422
	rec = apitest.Do(t, e, http.MethodPatch, coursesPath+"/"+id,
		map[string]string{"Content-Type": "application/json"}, `{"title":"新题"}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")

	// PATCH 正常更新（version=1）
	rec = apitest.Do(t, e, http.MethodPatch, coursesPath+"/"+id,
		map[string]string{"Content-Type": "application/json"},
		`{"title":"新题","archived":true,"version":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodPatch, coursePath, http.StatusOK, rec.Body.Bytes())
	var updated map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated["title"] != "新题" || updated["archived"] != true || updated["version"] != float64(2) {
		t.Fatalf("更新结果不符: %v", updated)
	}

	// 过期版本 → 409 version_conflict
	rec = apitest.Do(t, e, http.MethodPatch, coursesPath+"/"+id,
		map[string]string{"Content-Type": "application/json"}, `{"title":"并发","version":1}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "version_conflict")

	// DELETE → 204；再 GET → 404；再 DELETE → 404
	rec = apitest.Do(t, e, http.MethodDelete, coursesPath+"/"+id, nil, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = apitest.Do(t, e, http.MethodGet, coursesPath+"/"+id, nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	rec = apitest.Do(t, e, http.MethodDelete, coursesPath+"/"+id, nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")

	// 未知 id 的 GET → 404；非法 UUID → 400
	rec = apitest.Do(t, e, http.MethodGet, coursesPath+"/"+uuid.NewString(), nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	rec = apitest.Do(t, e, http.MethodGet, coursesPath+"/not-a-uuid", nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
}

func TestDeleteCourseCascade(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)

	created := createCourse(t, e, uuid.NewString(), `{"title":"级联"}`)
	id := created["id"].(string)
	ctx := context.Background()
	lessonID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,'课时')`,
		lessonID, id); err != nil {
		t.Fatal(err)
	}

	rec := apitest.Do(t, e, http.MethodDelete, coursesPath+"/"+id, nil, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons WHERE course_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("课程删除后课时残留 %d 条", n)
	}
}

func TestBatchDeleteCourses(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	url := coursesPath + ":batch-delete"
	hdr := map[string]string{"Content-Type": "application/json"}

	// 空 ids / 缺 ids / 非法 UUID / 超上限 → 400 bad_request
	rec := apitest.Do(t, e, http.MethodPost, url, hdr, `{"ids":[]}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	rec = apitest.Do(t, e, http.MethodPost, url, hdr, `{}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	rec = apitest.Do(t, e, http.MethodPost, url, hdr, `{"ids":["not-a-uuid"]}`)
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
	many := make([]string, 101)
	for i := range many {
		many[i] = uuid.NewString()
	}
	body, _ := json.Marshal(map[string]any{"ids": many})
	rec = apitest.Do(t, e, http.MethodPost, url, hdr, string(body))
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")

	// 成功删除多个（重复 id 去重后仍全部删除）
	a := createCourse(t, e, uuid.NewString(), `{"title":"批删甲"}`)["id"].(string)
	b := createCourse(t, e, uuid.NewString(), `{"title":"批删乙"}`)["id"].(string)
	body, _ = json.Marshal(map[string]any{"ids": []string{a, b, a}})
	rec = apitest.Do(t, e, http.MethodPost, url, hdr, string(body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, id := range []string{a, b} {
		rec = apitest.Do(t, e, http.MethodGet, coursesPath+"/"+id, nil, "")
		contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	}
}

func TestBatchDeleteCoursesRollback(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	contract := apitest.Load(t)
	ctx := context.Background()
	url := coursesPath + ":batch-delete"
	hdr := map[string]string{"Content-Type": "application/json"}

	// 含不存在 id → 404 且整体回滚（其余课程仍在）
	keep := createCourse(t, e, uuid.NewString(), `{"title":"保留"}`)["id"].(string)
	body, _ := json.Marshal(map[string]any{"ids": []string{keep, uuid.NewString()}})
	rec := apitest.Do(t, e, http.MethodPost, url, hdr, string(body))
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	rec = apitest.Do(t, e, http.MethodGet, coursesPath+"/"+keep, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("404 回滚后课程应保留：status=%d", rec.Code)
	}

	// 含活动 lesson_generation 任务的课程 → 409 resource_busy 且整体回滚
	busy := createCourse(t, e, uuid.NewString(), `{"title":"繁忙"}`)["id"].(string)
	lessonID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,'课时')`,
		lessonID, busy); err != nil {
		t.Fatal(err)
	}
	fam := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindLessonImport,
		AggregateType:    "lesson",
		AggregateID:      lessonID,
		GenerationFamily: &fam,
		Payload:          []byte(`{}`),
	}
	if err := (&job.Repository{}).Create(ctx, pool, j); err != nil {
		t.Fatal(err)
	}
	keep2 := createCourse(t, e, uuid.NewString(), `{"title":"保留二"}`)["id"].(string)
	body, _ = json.Marshal(map[string]any{"ids": []string{keep2, busy}})
	rec = apitest.Do(t, e, http.MethodPost, url, hdr, string(body))
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
	for _, id := range []string{keep2, busy} {
		rec = apitest.Do(t, e, http.MethodGet, coursesPath+"/"+id, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("409 回滚后课程应保留：id=%s status=%d", id, rec.Code)
		}
	}
}

func TestCourseAutoTitleAndManualRenameProtection(t *testing.T) {
	pool := databasetest.New(t)
	e := newEngine(t, pool)
	ctx := context.Background()
	for _, body := range []string{`{}`, `{"title":""}`, `{"title":"   "}`} {
		dto := createCourse(t, e, uuid.NewString(), body)
		id := uuid.MustParse(dto["id"].(string))
		c, err := (course.Repository{}).GetByID(ctx, pool, id)
		if err != nil || !c.TitlePending || c.Title != "待 AI 命名" {
			t.Fatalf("pending course: %+v %v", c, err)
		}
		l := &lesson.Lesson{CourseID: id, Ordinal: 1, Title: "导入课时"}
		if err := (lesson.Repository{}).Create(ctx, pool, l); err != nil {
			t.Fatal(err)
		}
		if err := (course.Repository{}).ApplyGeneratedTitle(ctx, pool, l.ID, "极限与连续"); err != nil {
			t.Fatal(err)
		}
		if err := (course.Repository{}).ApplyGeneratedTitle(ctx, pool, l.ID, "另一份总结"); err != nil {
			t.Fatal(err)
		}
		c, err = (course.Repository{}).GetByID(ctx, pool, id)
		if err != nil || c.TitlePending || c.Title != "极限与连续" || c.Version != 2 {
			t.Fatalf("AI title: %+v %v", c, err)
		}
	}
	dto := createCourse(t, e, uuid.NewString(), `{}`)
	id := uuid.MustParse(dto["id"].(string))
	_, err := course.NewService(pool).Update(ctx, id, course.UpdateInput{Title: ptrTitle("手动命名"), Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	l := &lesson.Lesson{CourseID: id, Ordinal: 1, Title: "课时"}
	if err := (lesson.Repository{}).Create(ctx, pool, l); err != nil {
		t.Fatal(err)
	}
	if err := (course.Repository{}).ApplyGeneratedTitle(ctx, pool, l.ID, "AI 名称"); err != nil {
		t.Fatal(err)
	}
	c, err := (course.Repository{}).GetByID(ctx, pool, id)
	if err != nil || c.TitlePending || c.Title != "手动命名" {
		t.Fatalf("manual title overwritten: %+v %v", c, err)
	}
}
func ptrTitle(s string) *string { return &s }
