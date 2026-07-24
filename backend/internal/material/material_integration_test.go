package material_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/material"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

// testEnv 是一次测试的完整装配：真实数据库 + 真实文件存储 + 完整引擎。
type testEnv struct {
	engine *gin.Engine
	pool   *pgxpool.Pool
	store  *filestore.Store
	svc    *material.Service
}

// newEnv 装配 material 模块（allowlist 注入 youtube.com/bilibili.com；
// DNS 解析注入桩：默认返回公网 IP，两个特殊主机用于失败矩阵）。
func newEnv(t *testing.T, uploadLimitBytes int64) *testEnv {
	t.Helper()
	pool := databasetest.New(t)
	store, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc := material.NewService(pool, store, []string{"youtube.com", "bilibili.com"})
	svc.HostResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		switch host {
		case "blackhole.youtube.com":
			return nil, errors.New("no such host")
		case "internal.youtube.com":
			return []net.IP{net.ParseIP("192.168.10.9")}, nil
		default:
			return []net.IP{net.ParseIP("151.101.1.140")}, nil
		}
	}
	h := material.NewHandler(svc, &httpx.IdemExecutor{Svc: idempotency.NewService(pool)},
		uploadLimitBytes, 0, 0)
	engine := apitest.NewEngine(t)
	h.RegisterRoutes(engine.Group("/api/v1"))
	return &testEnv{engine: engine, pool: pool, store: store, svc: svc}
}

// newLesson 直接经 Repository 播种课程+课时，返回课时 ID。
func (e *testEnv) newLesson(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	c := &course.Course{Slug: "c-" + uuid.NewString(), Title: "课程"}
	if err := (course.Repository{}).Create(ctx, e.pool, c); err != nil {
		t.Fatal(err)
	}
	l := &lesson.Lesson{CourseID: c.ID, Ordinal: 1, Title: "课时"}
	if err := (lesson.Repository{}).Create(ctx, e.pool, l); err != nil {
		t.Fatal(err)
	}
	return l.ID
}

func decodeJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return v
}

// postJSON 发起带幂等键的 JSON POST。
func (e *testEnv) postJSON(t *testing.T, target, body, idemKey string) (
	code int, respBody []byte, location string) {
	t.Helper()
	rec := apitest.Do(t, e.engine, "POST", target, map[string]string{
		"Idempotency-Key": idemKey,
		"Content-Type":    "application/json",
	}, body)
	return rec.Code, rec.Body.Bytes(), rec.Header().Get("Location")
}

// postMultipart 发起带幂等键的 multipart POST。
func (e *testEnv) postMultipart(t *testing.T, target string, fields map[string]string,
	fileField, filename string, content []byte, idemKey string) (int, []byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if fileField != "" {
		fw, err := w.CreateFormFile(fileField, filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	rec := apitest.DoRaw(t, e.engine, "POST", target, map[string]string{
		"Idempotency-Key": idemKey,
		"Content-Type":    w.FormDataContentType(),
	}, &buf)
	return rec.Code, rec.Body.Bytes(), rec.Header().Get("Location")
}

// materialOf 从 202 响应体取 material 对象。
func materialOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	m, ok := decodeJSON(t, body)["material"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 material: %s", body)
	}
	return m
}

func jobOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	j, ok := decodeJSON(t, body)["job"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 job: %s", body)
	}
	return j
}

// --- text 素材 ------------------------------------------------------------

func TestCreateTextMaterial(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/text", lessonID)

	code, body, location := env.postJSON(t, target,
		`{"title":"讲义标题","text":"第一章 正文"}`, uuid.NewString())
	if code != 202 {
		t.Fatalf("status = %d, 期望 202: %s", code, body)
	}
	apitest.Load(t).ValidateJSONResponse(t, "POST",
		"/api/v1/lessons/{lesson_id}/materials/text", 202, body)
	if !strings.HasPrefix(location, "/api/v1/jobs/") {
		t.Fatalf("Location = %q, 期望 /api/v1/jobs/ 前缀", location)
	}
	m := materialOf(t, body)
	if m["kind"] != "text" || m["status"] != "pending" {
		t.Fatalf("material kind/status 不符: %v", m)
	}
	if m["lesson_id"] != lessonID.String() {
		t.Fatalf("material.lesson_id = %v", m["lesson_id"])
	}
	if m["source_url"] != nil || m["original_name"] != nil {
		t.Fatalf("text 素材的 source_url/original_name 必须为 null: %v", m)
	}
	j := jobOf(t, body)
	if j["kind"] != "lesson_import" || j["generation_family"] != "lesson_generation" {
		t.Fatalf("job kind/family 不符: %v", j)
	}
	if j["status"] != "queued" || j["aggregate_id"] != lessonID.String() {
		t.Fatalf("job 状态/聚合不符: %v", j)
	}

	// 数据库副作用：inline_text / metadata.title / 课时状态 / Job payload。
	materialID := m["id"].(string)
	var inlineText, title, lessonStatus, payload string
	err := env.pool.QueryRow(context.Background(), `
		SELECT m.inline_text, m.metadata->>'title', l.status, j.payload::text
		FROM materials m, lessons l, jobs j
		WHERE m.id = $1 AND l.id = m.lesson_id AND j.payload->>'material_id' = $2`,
		materialID, materialID).Scan(&inlineText, &title, &lessonStatus, &payload)
	if err != nil {
		t.Fatal(err)
	}
	if inlineText != "第一章 正文" || title != "讲义标题" {
		t.Fatalf("inline_text/title 落库不符: %q / %q", inlineText, title)
	}
	if lessonStatus != "processing" {
		t.Fatalf("课时状态 = %q, 期望 processing", lessonStatus)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatal(err)
	}
	if p["material_id"] != materialID || p["material_kind"] != "text" ||
		p["lesson_id"] != lessonID.String() || p["title"] != "讲义标题" {
		t.Fatalf("job payload 不符: %s", payload)
	}
}

func TestCreateTextMaterialValidation(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/text", lessonID)

	cases := []struct {
		name      string
		body      string
		wantCode  int
		wantField string
	}{
		{"空 text", `{"text":""}`, 422, "text"},
		{"缺 text", `{"title":"x"}`, 422, "text"},
		{"title 超长", `{"title":"` + strings.Repeat("题", 201) + `","text":"ok"}`, 422, "title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body, _ := env.postJSON(t, target, tc.body, uuid.NewString())
			if code != tc.wantCode {
				t.Fatalf("status = %d, 期望 %d: %s", code, tc.wantCode, body)
			}
			apitest.Load(t).ValidateProblem(t, body, tc.wantCode, "validation_error")
			p := decodeJSON(t, body)
			errs, _ := p["errors"].([]any)
			if len(errs) == 0 || errs[0].(map[string]any)["field"] != tc.wantField {
				t.Fatalf("错误字段不符: %v", p["errors"])
			}
		})
	}

	// 未知字段（additionalProperties:false）→ 400。
	code, body, _ := env.postJSON(t, target, `{"text":"ok","hack":1}`, uuid.NewString())
	if code != 400 {
		t.Fatalf("未知字段 status = %d, 期望 400: %s", code, body)
	}
	apitest.Load(t).ValidateProblem(t, body, 400, "bad_request")

	// 课时不存在 → 404。
	code, body, _ = env.postJSON(t,
		fmt.Sprintf("/api/v1/lessons/%s/materials/text", uuid.NewString()),
		`{"text":"ok"}`, uuid.NewString())
	if code != 404 {
		t.Fatalf("课时 404 status = %d: %s", code, body)
	}
	apitest.Load(t).ValidateProblem(t, body, 404, "not_found")
}

// --- URL 素材 -------------------------------------------------------------

func TestCreateURLMaterial(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/url", lessonID)

	code, body, location := env.postJSON(t, target,
		`{"url":"HTTPS://YouTube.com/watch?v=abc#frag","preferred_language":"zh-CN"}`,
		uuid.NewString())
	if code != 202 {
		t.Fatalf("status = %d, 期望 202: %s", code, body)
	}
	apitest.Load(t).ValidateJSONResponse(t, "POST",
		"/api/v1/lessons/{lesson_id}/materials/url", 202, body)
	if !strings.HasPrefix(location, "/api/v1/jobs/") {
		t.Fatalf("Location = %q", location)
	}
	m := materialOf(t, body)
	// 规范化：scheme/host 小写、去 fragment。
	if m["source_url"] != "https://youtube.com/watch?v=abc" {
		t.Fatalf("source_url 未规范化: %v", m["source_url"])
	}
	if m["kind"] != "url" || m["original_name"] != nil {
		t.Fatalf("url 素材字段不符: %v", m)
	}
	// 后缀匹配：www.youtube.com 被 youtube.com 覆盖。
	code, body, _ = env.postJSON(t,
		fmt.Sprintf("/api/v1/lessons/%s/materials/url", env.newLesson(t)),
		`{"url":"https://www.youtube.com/watch?v=x"}`, uuid.NewString())
	if code != 202 {
		t.Fatalf("后缀匹配 status = %d: %s", code, body)
	}
	if u := materialOf(t, body)["source_url"]; u != "https://www.youtube.com/watch?v=x" {
		t.Fatalf("source_url = %v", u)
	}
}

func TestCreateURLRejectMatrix(t *testing.T) {
	env := newEnv(t, 0)
	cases := []struct {
		name         string
		url          string
		wantFieldErr string // 空串表示只校验 422 validation_error
	}{
		{"http 协议", "http://youtube.com/x", ""},
		{"非白名单域名", "https://evil.example.com/x", "url_not_allowed"},
		{"localhost", "https://localhost/x", "url_not_allowed"},
		{"回环 IP", "https://127.0.0.1/x", "url_not_allowed"},
		{"私网 192.168", "https://192.168.1.1/x", "url_not_allowed"},
		{"私网 10.x", "https://10.1.2.3/x", "url_not_allowed"},
		{"link-local", "https://169.254.1.1/x", "url_not_allowed"},
		{"白名单域名解析到私网", "https://internal.youtube.com/x", "url_private"},
		{"DNS 解析失败", "https://blackhole.youtube.com/x", "url_unresolvable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := fmt.Sprintf("/api/v1/lessons/%s/materials/url", env.newLesson(t))
			code, body, _ := env.postJSON(t, target,
				fmt.Sprintf(`{"url":%q}`, tc.url), uuid.NewString())
			if code != 422 {
				t.Fatalf("status = %d, 期望 422: %s", code, body)
			}
			apitest.Load(t).ValidateProblem(t, body, 422, "validation_error")
			p := decodeJSON(t, body)
			errs, _ := p["errors"].([]any)
			if len(errs) == 0 {
				t.Fatalf("缺少字段错误: %v", p)
			}
			fe := errs[0].(map[string]any)
			if fe["field"] != "url" {
				t.Fatalf("错误字段 = %v, 期望 url", fe["field"])
			}
			if tc.wantFieldErr != "" && fe["code"] != tc.wantFieldErr {
				t.Fatalf("错误码 = %v, 期望 %s", fe["code"], tc.wantFieldErr)
			}
		})
	}
}

// --- 上传素材 -------------------------------------------------------------

func pdfContent() []byte {
	return append([]byte("%PDF-1.4\n1 0 obj\n"), bytes.Repeat([]byte("x"), 2048)...)
}

func TestUploadMaterial(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/upload", lessonID)
	content := pdfContent()
	wantSHA := sha256.Sum256(content)

	code, body, location := env.postMultipart(t, target,
		map[string]string{"preferred_language": "zh", "transcription_mode": "whisper_local"},
		"file", "讲义.pdf", content, uuid.NewString())
	if code != 202 {
		t.Fatalf("status = %d, 期望 202: %s", code, body)
	}
	apitest.Load(t).ValidateJSONResponse(t, "POST",
		"/api/v1/lessons/{lesson_id}/materials/upload", 202, body)
	if !strings.HasPrefix(location, "/api/v1/jobs/") {
		t.Fatalf("Location = %q", location)
	}
	m := materialOf(t, body)
	if m["kind"] != "upload" || m["status"] != "pending" {
		t.Fatalf("material kind/status 不符: %v", m)
	}
	if m["original_name"] != "讲义.pdf" || m["source_url"] != nil {
		t.Fatalf("upload 素材展示字段不符: %v", m)
	}
	j := jobOf(t, body)
	if j["kind"] != "lesson_import" || j["generation_family"] != "lesson_generation" {
		t.Fatalf("job 不符: %v", j)
	}

	// 上传内容 hash 与 file_objects.sha256 一致，文件真实存在于 objects/。
	materialID := m["id"].(string)
	var storageKey, dbSHA, mediaType, metadata string
	err := env.pool.QueryRow(context.Background(), `
		SELECT f.storage_key, f.sha256, f.media_type, m.metadata::text
		FROM materials m JOIN file_objects f ON f.id = m.file_object_id
		WHERE m.id = $1 AND m.kind = 'upload' AND f.state = 'ready'`, materialID).
		Scan(&storageKey, &dbSHA, &mediaType, &metadata)
	if err != nil {
		t.Fatal(err)
	}
	if dbSHA != hex.EncodeToString(wantSHA[:]) {
		t.Fatalf("file_objects.sha256 = %q, 期望内容 hash %q", dbSHA, hex.EncodeToString(wantSHA[:]))
	}
	if mediaType != "application/pdf" {
		t.Fatalf("media_type = %q", mediaType)
	}
	disk, err := os.ReadFile(filepath.Join(env.store.DataDir(), storageKey))
	if err != nil {
		t.Fatalf("读取已发布对象失败: %v", err)
	}
	diskSHA := sha256.Sum256(disk)
	if hex.EncodeToString(diskSHA[:]) != dbSHA {
		t.Fatalf("磁盘文件 hash 与记录不一致")
	}
	if !strings.HasPrefix(storageKey, "objects/") {
		t.Fatalf("storage_key = %q, 期望 objects/ 前缀", storageKey)
	}
	var md map[string]any
	if err := json.Unmarshal([]byte(metadata), &md); err != nil {
		t.Fatal(err)
	}
	if md["preferred_language"] != "zh" || md["transcription_mode"] != "whisper_local" {
		t.Fatalf("metadata 不符: %s", metadata)
	}
	// 暂存目录已清空（发布 rename 接管）。
	entries, err := os.ReadDir(filepath.Join(env.store.DataDir(), "tmp", "_staging"))
	if err == nil && len(entries) != 0 {
		t.Fatalf("暂存目录未清空: %v", entries)
	}
}

func TestUploadMIMEForgery(t *testing.T) {
	env := newEnv(t, 0)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/upload", env.newLesson(t))
	// PE 可执行文件内容伪装成 note.pdf。
	exe := append([]byte("MZ\x90\x00\x03\x00\x00\x00"), bytes.Repeat([]byte("\x00"), 600)...)
	code, body, _ := env.postMultipart(t, target, nil, "file", "note.pdf", exe, uuid.NewString())
	if code != 415 {
		t.Fatalf("status = %d, 期望 415: %s", code, body)
	}
	apitest.Load(t).ValidateProblem(t, body, 415, "unsupported_media_type")
}

func TestUploadMissingFile(t *testing.T) {
	env := newEnv(t, 0)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/upload", env.newLesson(t))
	code, body, _ := env.postMultipart(t, target,
		map[string]string{"preferred_language": "zh"}, "", "", nil, uuid.NewString())
	if code != 400 {
		t.Fatalf("status = %d, 期望 400: %s", code, body)
	}
	apitest.Load(t).ValidateProblem(t, body, 400, "bad_request")
}

func TestUploadFilenamePathTraversal(t *testing.T) {
	env := newEnv(t, 0)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/upload", env.newLesson(t))
	code, body, _ := env.postMultipart(t, target, nil,
		"file", "../../etc/passwd.txt", []byte("plain text 内容\n"), uuid.NewString())
	if code != 202 {
		t.Fatalf("status = %d, 期望 202: %s", code, body)
	}
	if name := materialOf(t, body)["original_name"]; name != "passwd.txt" {
		t.Fatalf("original_name = %v, 期望清洗为 passwd.txt", name)
	}
}

func TestUploadInvalidTranscriptionMode(t *testing.T) {
	env := newEnv(t, 0)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/upload", env.newLesson(t))
	code, body, _ := env.postMultipart(t, target,
		map[string]string{"transcription_mode": "cloud_whisper"},
		"file", "a.txt", []byte("hello\n"), uuid.NewString())
	if code != 422 {
		t.Fatalf("status = %d, 期望 422: %s", code, body)
	}
	apitest.Load(t).ValidateProblem(t, body, 422, "validation_error")
	p := decodeJSON(t, body)
	errs, _ := p["errors"].([]any)
	if len(errs) == 0 || errs[0].(map[string]any)["field"] != "transcription_mode" {
		t.Fatalf("错误字段不符: %v", p["errors"])
	}
}

func TestUploadTooLarge(t *testing.T) {
	env := newEnv(t, 512) // 注入极小上限触发 413
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/upload", env.newLesson(t))
	code, body, _ := env.postMultipart(t, target, nil, "file", "big.pdf", pdfContent(), uuid.NewString())
	if code != 413 {
		t.Fatalf("status = %d, 期望 413: %s", code, body)
	}
	apitest.Load(t).ValidateProblem(t, body, 413, "upload_too_large")
}

func TestUploadMissingIdempotencyKey(t *testing.T) {
	env := newEnv(t, 0)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/upload", env.newLesson(t))
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("hi\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	rec := apitest.DoRaw(t, env.engine, "POST", target,
		map[string]string{"Content-Type": w.FormDataContentType()}, &buf)
	if rec.Code != 400 {
		t.Fatalf("status = %d, 期望 400: %s", rec.Code, rec.Body)
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), 400, "bad_request")
}

// --- 列表 -----------------------------------------------------------------

func TestListMaterials(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	// 直接经 Repository 插入 3 条（避免创建端点的活动任务互斥）。
	repo := material.Repository{}
	wantOrder := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		text := fmt.Sprintf("正文 %d", i)
		m := &material.Material{LessonID: lessonID, Kind: material.KindText, InlineText: &text}
		if err := repo.Create(context.Background(), env.pool, m); err != nil {
			t.Fatal(err)
		}
		wantOrder = append(wantOrder, m.ID.String())
		time.Sleep(2 * time.Millisecond)
	}

	target := fmt.Sprintf("/api/v1/lessons/%s/materials?limit=2", lessonID)
	rec := apitest.Do(t, env.engine, "GET", target, nil, "")
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	apitest.Load(t).ValidateJSONResponse(t, "GET",
		"/api/v1/lessons/{lesson_id}/materials", 200, rec.Body.Bytes())
	page1 := decodeJSON(t, rec.Body.Bytes())
	items1, _ := page1["items"].([]any)
	if len(items1) != 2 {
		t.Fatalf("第一页条数 = %d, 期望 2", len(items1))
	}
	pi, _ := page1["page_info"].(map[string]any)
	if pi["has_more"] != true || pi["next_cursor"] == nil {
		t.Fatalf("page_info 不符: %v", pi)
	}
	if items1[0].(map[string]any)["id"] != wantOrder[0] ||
		items1[1].(map[string]any)["id"] != wantOrder[1] {
		t.Fatalf("第一页顺序不符: %v", items1)
	}

	// 跟随游标取第二页。
	cursor := pi["next_cursor"].(string)
	rec2 := apitest.Do(t, env.engine, "GET",
		fmt.Sprintf("/api/v1/lessons/%s/materials?limit=2&cursor=%s", lessonID, cursor), nil, "")
	if rec2.Code != 200 {
		t.Fatalf("第二页 status = %d: %s", rec2.Code, rec2.Body)
	}
	page2 := decodeJSON(t, rec2.Body.Bytes())
	items2, _ := page2["items"].([]any)
	if len(items2) != 1 || items2[0].(map[string]any)["id"] != wantOrder[2] {
		t.Fatalf("第二页不符: %v", items2)
	}
	pi2, _ := page2["page_info"].(map[string]any)
	if pi2["has_more"] != false || pi2["next_cursor"] != nil {
		t.Fatalf("第二页 page_info 不符: %v", pi2)
	}

	// 非法游标 → 400。
	rec3 := apitest.Do(t, env.engine, "GET",
		fmt.Sprintf("/api/v1/lessons/%s/materials?cursor=%%20%%20", lessonID), nil, "")
	if rec3.Code != 400 {
		t.Fatalf("非法游标 status = %d: %s", rec3.Code, rec3.Body)
	}

	// 课时不存在 → 404。
	rec4 := apitest.Do(t, env.engine, "GET",
		fmt.Sprintf("/api/v1/lessons/%s/materials", uuid.NewString()), nil, "")
	if rec4.Code != 404 {
		t.Fatalf("课时 404 status = %d: %s", rec4.Code, rec4.Body)
	}
	apitest.Load(t).ValidateProblem(t, rec4.Body.Bytes(), 404, "not_found")
}

// --- 删除 -----------------------------------------------------------------

func TestDeleteMaterial(t *testing.T) {
	env := newEnv(t, 0)

	// 404：素材不存在。
	rec := apitest.Do(t, env.engine, "DELETE",
		fmt.Sprintf("/api/v1/materials/%s", uuid.NewString()), nil, "")
	if rec.Code != 404 {
		t.Fatalf("删除不存在素材 status = %d: %s", rec.Code, rec.Body)
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), 404, "not_found")

	// 409：素材被活动 Job 引用（创建端点产生的 queued lesson_import）。
	lessonID := env.newLesson(t)
	code, body, _ := env.postJSON(t,
		fmt.Sprintf("/api/v1/lessons/%s/materials/text", lessonID),
		`{"text":"忙碌素材"}`, uuid.NewString())
	if code != 202 {
		t.Fatalf("创建 status = %d: %s", code, body)
	}
	busyID := materialOf(t, body)["id"].(string)
	rec = apitest.Do(t, env.engine, "DELETE", "/api/v1/materials/"+busyID, nil, "")
	if rec.Code != 409 {
		t.Fatalf("删除忙碌素材 status = %d: %s", rec.Code, rec.Body)
	}
	apitest.Load(t).ValidateProblem(t, rec.Body.Bytes(), 409, "resource_busy")

	// 204：上传素材 + Job 已终态 → 删除成功，文件对象标记 deleted。
	code, body, _ = env.postMultipart(t,
		fmt.Sprintf("/api/v1/lessons/%s/materials/upload", env.newLesson(t)),
		nil, "file", "notes.pdf", pdfContent(), uuid.NewString())
	if code != 202 {
		t.Fatalf("上传 status = %d: %s", code, body)
	}
	uploadID := materialOf(t, body)["id"].(string)
	if _, err := env.pool.Exec(context.Background(), `
		UPDATE jobs SET status = 'succeeded', completed_at = now(), updated_at = now()
		WHERE payload->>'material_id' = $1`, uploadID); err != nil {
		t.Fatal(err)
	}
	rec = apitest.Do(t, env.engine, "DELETE", "/api/v1/materials/"+uploadID, nil, "")
	if rec.Code != 204 {
		t.Fatalf("删除上传素材 status = %d: %s", rec.Code, rec.Body)
	}
	var cnt int
	if err := env.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM materials WHERE id = $1`, uploadID).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatal("素材未删除")
	}
	var fileState string
	if err := env.pool.QueryRow(context.Background(), `
		SELECT f.state FROM file_objects f
		WHERE f.id = (SELECT file_object_id FROM materials WHERE id = $1)`, uploadID).
		Scan(&fileState); err != nil {
		// materials 行已删，改用 JSON 记录的 file_object_id 查询。
		var state string
		if err2 := env.pool.QueryRow(context.Background(), `
			SELECT state FROM file_objects
			WHERE original_name = 'notes.pdf' AND media_type = 'application/pdf'
			ORDER BY created_at DESC LIMIT 1`).Scan(&state); err2 != nil {
			t.Fatalf("查询文件对象失败: %v / %v", err, err2)
		}
		fileState = state
	}
	if fileState != "deleted" {
		t.Fatalf("文件对象状态 = %q, 期望 deleted", fileState)
	}
}

// --- 幂等 / 并发互斥 / 维护模式 --------------------------------------------

func TestIdempotencyReplayAndConflict(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/text", lessonID)
	key := uuid.NewString()

	code1, body1, _ := env.postJSON(t, target, `{"text":"幂等正文"}`, key)
	if code1 != 202 {
		t.Fatalf("首次 status = %d: %s", code1, body1)
	}
	// 同 Key 同体 → 回放：相同 material/job id。
	code2, body2, _ := env.postJSON(t, target, `{"text":"幂等正文"}`, key)
	if code2 != 202 {
		t.Fatalf("回放 status = %d: %s", code2, body2)
	}
	if materialOf(t, body1)["id"] != materialOf(t, body2)["id"] ||
		jobOf(t, body1)["id"] != jobOf(t, body2)["id"] {
		t.Fatalf("回放 id 不一致: %s vs %s", body1, body2)
	}
	// 同 Key 异体 → 409 idempotency_conflict。
	key2 := uuid.NewString()
	if code, _, _ := env.postJSON(t, fmt.Sprintf("/api/v1/lessons/%s/materials/text", env.newLesson(t)),
		`{"text":"一"}`, key2); code != 202 {
		t.Fatalf("准备请求 status = %d", code)
	}
	code3, body3, _ := env.postJSON(t, fmt.Sprintf("/api/v1/lessons/%s/materials/text", env.newLesson(t)),
		`{"text":"二"}`, key2)
	if code3 != 409 {
		t.Fatalf("冲突 status = %d: %s", code3, body3)
	}
	apitest.Load(t).ValidateProblem(t, body3, 409, "idempotency_conflict")
}

func TestCreateResourceBusy(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	target := fmt.Sprintf("/api/v1/lessons/%s/materials/text", lessonID)

	code1, body1, _ := env.postJSON(t, target, `{"text":"第一份"}`, uuid.NewString())
	if code1 != 202 {
		t.Fatalf("首次 status = %d: %s", code1, body1)
	}
	// 不同幂等键也不能创建第二个活动 lesson_generation 任务。
	code2, body2, _ := env.postJSON(t, target, `{"text":"第二份"}`, uuid.NewString())
	if code2 != 409 {
		t.Fatalf("冲突 status = %d: %s", code2, body2)
	}
	apitest.Load(t).ValidateProblem(t, body2, 409, "resource_busy")
	p := decodeJSON(t, body2)
	if p["active_job_id"] != jobOf(t, body1)["id"] {
		t.Fatalf("active_job_id 扩展不符: %v", p["active_job_id"])
	}
	if p["generation_family"] != "lesson_generation" {
		t.Fatalf("generation_family 扩展不符: %v", p["generation_family"])
	}
}

func TestMaintenanceMode(t *testing.T) {
	env := newEnv(t, 0)
	lessonID := env.newLesson(t)
	if _, err := env.pool.Exec(context.Background(),
		`UPDATE system_state SET maintenance_mode = true WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}
	code, body, _ := env.postJSON(t,
		fmt.Sprintf("/api/v1/lessons/%s/materials/text", lessonID),
		`{"text":"维护期写入"}`, uuid.NewString())
	if code != 503 {
		t.Fatalf("status = %d, 期望 503: %s", code, body)
	}
	apitest.Load(t).ValidateProblem(t, body, 503, "maintenance_mode")
}
