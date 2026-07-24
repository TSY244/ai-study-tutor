package export_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/export"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

type testEnv struct {
	engine http.Handler
	pool   *pgxpool.Pool
	store  *filestore.Store
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	pool := databasetest.New(t)
	st, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h := export.NewHandler(export.NewService(pool, st))
	engine := apitest.NewEngine(t)
	h.RegisterRoutes(engine.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, 400, "bad_request"))
	return &testEnv{engine: engine, pool: pool, store: st}
}

// seedLesson 创建课程与课时，返回课时 ID。
func seedLesson(t *testing.T, pool *pgxpool.Pool, title string) uuid.UUID {
	t.Helper()
	courseID, lessonID := uuid.New(), uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO courses (id, slug, title) VALUES ($1,$2,'测试课程')`,
		courseID, "c-"+uuid.NewString()[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,$3)`,
		lessonID, courseID, title); err != nil {
		t.Fatal(err)
	}
	return lessonID
}

// seedActiveDocument 创建并发布一个 active 文档版本。
func seedActiveDocument(t *testing.T, pool *pgxpool.Pool, lessonID uuid.UUID, kind, content string) {
	t.Helper()
	docs := document.Repository{}
	d := &document.Document{LessonID: lessonID, Kind: kind, ContentMD: content}
	if err := docs.CreateVersion(context.Background(), pool, d); err != nil {
		t.Fatal(err)
	}
	if err := docs.Publish(context.Background(), pool, d.ID); err != nil {
		t.Fatal(err)
	}
}

func postExport(t *testing.T, env *testEnv, lessonID uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	return apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/lessons/"+lessonID.String()+"/exports",
		map[string]string{"Content-Type": "application/json"}, body)
}

const (
	lessonTitle = "第 1 课 函数与极限"
	notesMD     = "这是学生笔记。\n重点：极限定义。"
	outlineMD   = "教师教纲内容。"
)

// expectedMarkdown 与 service 渲染格式独立地拼出期望内容。
func expectedMarkdown() string {
	return "# " + lessonTitle + "\n" +
		"\n## 学生笔记\n\n" + notesMD + "\n" +
		"\n## 教师教纲\n\n" + outlineMD + "\n"
}

// markdown 同步导出：200 + FileObject 契约 + sha256/下载链接/落盘内容一致。
func TestExportMarkdownSync(t *testing.T) {
	env := newTestEnv(t)
	lessonID := seedLesson(t, env.pool, lessonTitle)
	seedActiveDocument(t, env.pool, lessonID, "student_notes", notesMD)
	seedActiveDocument(t, env.pool, lessonID, "teacher_outline", outlineMD)

	rec := postExport(t, env, lessonID, `{"format":"markdown"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200: %s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPost,
		"/api/v1/lessons/{lesson_id}/exports", http.StatusOK, rec.Body.Bytes())

	var resp struct {
		ID          string  `json:"id"`
		DisplayName *string `json:"display_name"`
		MediaType   string  `json:"media_type"`
		SizeBytes   int64   `json:"size_bytes"`
		SHA256      string  `json:"sha256"`
		DownloadURL string  `json:"download_url"`
		CreatedAt   string  `json:"created_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	objectID, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("id 不是 UUID: %v", err)
	}
	want := expectedMarkdown()
	sum := sha256.Sum256([]byte(want))
	if resp.DisplayName == nil || *resp.DisplayName != lessonTitle+".md" {
		t.Fatalf("display_name = %v，期望 %q", resp.DisplayName, lessonTitle+".md")
	}
	if resp.MediaType != "text/markdown" {
		t.Fatalf("media_type = %q", resp.MediaType)
	}
	if resp.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 = %q，与内容不一致", resp.SHA256)
	}
	if resp.SizeBytes != int64(len(want)) {
		t.Fatalf("size_bytes = %d，期望 %d", resp.SizeBytes, len(want))
	}
	wantURL := "/api/v1/files/" + resp.ID + "/download"
	if resp.DownloadURL != wantURL {
		t.Fatalf("download_url = %q，期望 %q", resp.DownloadURL, wantURL)
	}
	if resp.CreatedAt == "" {
		t.Fatal("created_at 不能为空")
	}
	// 落盘对象：ready 且内容与渲染结果一致。
	obj, err := env.store.GetByID(context.Background(), env.pool, objectID)
	if err != nil {
		t.Fatal(err)
	}
	if obj.State != filestore.StateReady {
		t.Fatalf("对象 state = %s，期望 ready", obj.State)
	}
	abs, err := env.store.AbsPath(obj.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(abs)
	if err != nil || string(got) != want {
		t.Fatalf("落盘内容不符: err=%v\n%s", err, got)
	}
}

// 没有任何 active 文档的课时 → 404 not_found（中文 detail）。
func TestExportMarkdownNoDocuments(t *testing.T) {
	env := newTestEnv(t)
	lessonID := seedLesson(t, env.pool, "空课时")

	rec := postExport(t, env, lessonID, `{"format":"markdown"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d，期望 404: %s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	if !strings.Contains(rec.Body.String(), "该课时还没有可导出的文档。") {
		t.Fatalf("detail 不符: %s", rec.Body.String())
	}
}

// 课时不存在 → 404（markdown 与 zip 一致）。
func TestExportLessonNotFound(t *testing.T) {
	env := newTestEnv(t)
	for _, body := range []string{`{"format":"markdown"}`, `{"format":"zip"}`} {
		rec := postExport(t, env, uuid.New(), body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s 状态码 = %d，期望 404", body, rec.Code)
		}
		apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	}
}

// format 非法或缺失 → 422 validation_error（field=format）。
func TestExportInvalidFormat(t *testing.T) {
	env := newTestEnv(t)
	lessonID := seedLesson(t, env.pool, "课时")
	for _, body := range []string{`{"format":"pdf"}`, `{}`} {
		rec := postExport(t, env, lessonID, body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s 状态码 = %d，期望 422: %s", body, rec.Code, rec.Body.String())
		}
		apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(),
			http.StatusUnprocessableEntity, "validation_error")
		var p struct {
			Errors []struct {
				Field string `json:"field"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if len(p.Errors) == 0 || p.Errors[0].Field != "format" {
			t.Fatalf("%s 字段错误应为 format: %s", body, rec.Body.String())
		}
	}
}

// zip 导出：202 + Job 契约 + 数据库行（kind/aggregate/payload）。
func TestExportZipAccepted(t *testing.T) {
	env := newTestEnv(t)
	lessonID := seedLesson(t, env.pool, lessonTitle)

	rec := postExport(t, env, lessonID, `{"format":"zip"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("状态码 = %d，期望 202: %s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPost,
		"/api/v1/lessons/{lesson_id}/exports", http.StatusAccepted, rec.Body.Bytes())

	var resp struct {
		Job struct {
			ID               string  `json:"id"`
			Kind             string  `json:"kind"`
			AggregateType    *string `json:"aggregate_type"`
			AggregateID      *string `json:"aggregate_id"`
			GenerationFamily *string `json:"generation_family"`
			Status           string  `json:"status"`
		} `json:"job"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Job.Kind != "lesson_export" || resp.Job.Status != "queued" {
		t.Fatalf("job = %+v", resp.Job)
	}
	if resp.Job.AggregateType == nil || *resp.Job.AggregateType != "lesson" {
		t.Fatalf("aggregate_type = %v", resp.Job.AggregateType)
	}
	if resp.Job.AggregateID == nil || *resp.Job.AggregateID != lessonID.String() {
		t.Fatalf("aggregate_id = %v，期望 %s", resp.Job.AggregateID, lessonID)
	}
	if resp.Job.GenerationFamily != nil {
		t.Fatalf("generation_family 应为 null，得到 %v", *resp.Job.GenerationFamily)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/v1/jobs/"+resp.Job.ID {
		t.Fatalf("Location = %q", loc)
	}
	var payload string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT payload::text FROM jobs WHERE id = $1`, resp.Job.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `"lesson_id"`) || !strings.Contains(payload, `"zip"`) {
		t.Fatalf("payload 不符: %s", payload)
	}
}

// 维护模式下 zip 导出被拒 → 503 maintenance_mode。
func TestExportZipMaintenanceMode(t *testing.T) {
	env := newTestEnv(t)
	lessonID := seedLesson(t, env.pool, lessonTitle)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE system_state SET maintenance_mode = true WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}

	rec := postExport(t, env, lessonID, `{"format":"zip"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("状态码 = %d，期望 503: %s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(),
		http.StatusServiceUnavailable, "maintenance_mode")
}

// 先导出再下载：200 + Content-Disposition filename* + 内容一致。
func TestDownloadExportedMarkdown(t *testing.T) {
	env := newTestEnv(t)
	lessonID := seedLesson(t, env.pool, lessonTitle)
	seedActiveDocument(t, env.pool, lessonID, "student_notes", notesMD)
	seedActiveDocument(t, env.pool, lessonID, "raw_text", "原始课文。")

	exportRec := postExport(t, env, lessonID, `{"format":"markdown"}`)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("导出状态码 = %d: %s", exportRec.Code, exportRec.Body.String())
	}
	var resp struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(exportRec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	rec := apitest.Do(t, env.engine, http.MethodGet, resp.DownloadURL, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("下载状态码 = %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/markdown" {
		t.Fatalf("Content-Type = %q", ct)
	}
	wantName := url.QueryEscape(lessonTitle + ".md")
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment; filename*=UTF-8''"+wantName) {
		t.Fatalf("Content-Disposition = %q，期望含 filename*=UTF-8''%s", cd, wantName)
	}
	want := "# " + lessonTitle + "\n" +
		"\n## 学生笔记\n\n" + notesMD + "\n" +
		"\n## 原文\n\n原始课文。\n"
	if rec.Body.String() != want {
		t.Fatalf("下载内容不符:\n%q\n期望:\n%q", rec.Body.String(), want)
	}
	if cl := rec.Header().Get("Content-Length"); cl != fmt.Sprint(len(want)) {
		t.Fatalf("Content-Length = %q，期望 %d", cl, len(want))
	}
}

// insertFileObject 直接插入指定状态的文件对象行（404 矩阵用）。
func insertFileObject(t *testing.T, pool *pgxpool.Pool, state string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	sha := strings.Repeat("ab", 32)
	if state == "temporary" {
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO file_objects (id, storage_key, original_name, media_type,
				size_bytes, sha256, state,
				temporary_owner_type, temporary_owner_id, temporary_expires_at)
			VALUES ($1,$2,'x.md','text/markdown',3,$3,$4,'idempotency',$5,now() + interval '1 hour')`,
			id, "tmp/"+uuid.NewString()+"/"+id.String(), sha, state, uuid.New()); err != nil {
			t.Fatal(err)
		}
		return id
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO file_objects (id, storage_key, original_name, media_type,
			size_bytes, sha256, state)
		VALUES ($1,$2,'x.md','text/markdown',3,$3,$4)`,
		id, "objects/ab/"+id.String(), sha, state); err != nil {
		t.Fatal(err)
	}
	return id
}

// 下载 404 矩阵：不存在、temporary、quarantined、deleted 一律 404
// （不泄露存在性）；ready 行但磁盘内容缺失 → 404「文件内容缺失」。
func TestDownloadNotFoundMatrix(t *testing.T) {
	env := newTestEnv(t)
	cases := map[string]uuid.UUID{
		"不存在":         uuid.New(),
		"temporary":   insertFileObject(t, env.pool, "temporary"),
		"quarantined": insertFileObject(t, env.pool, "quarantined"),
		"deleted":     insertFileObject(t, env.pool, "deleted"),
	}
	for name, id := range cases {
		rec := apitest.Do(t, env.engine, http.MethodGet,
			"/api/v1/files/"+id.String()+"/download", nil, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s 状态码 = %d，期望 404", name, rec.Code)
		}
		apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	}

	missing := insertFileObject(t, env.pool, "ready")
	rec := apitest.Do(t, env.engine, http.MethodGet,
		"/api/v1/files/"+missing.String()+"/download", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("磁盘缺失 状态码 = %d，期望 404", rec.Code)
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), http.StatusNotFound, "not_found")
	if !strings.Contains(rec.Body.String(), "文件内容缺失") {
		t.Fatalf("detail 不符: %s", rec.Body.String())
	}
}
