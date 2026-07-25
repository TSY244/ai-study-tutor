package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

// URL 导入走 Claude Web Fetch 后的脱敏哨兵：只准出现在 prompt/上游响应中，
// 绝不持久化到 ai_runs 或 Job error/result。
const (
	urlQuerySentinel  = "SENTINEL-URL-QUERY-7f3a"
	promptSentinel    = "SENTINEL-PROMPT-9b2c"
	rawSourceSentinel = "SENTINEL-RAW-SOURCE-4d8e"
	keyLikeSentinel   = "sk-ant-SENTINEL-KEY-1a2b"
)

// useClaudeURLImport 给 handler 装配 URL 专用的 fake Provider（模拟
// GatewayOnly 注入）并标记可用。
func (e *env) useClaudeURLImport(t *testing.T) *ai.FakeProvider {
	t.Helper()
	fakeAI := ai.NewFakeProvider()
	e.handler.URLImportAI = fakeAI
	e.handler.URLImportAvailable = func() bool { return true }
	return fakeAI
}

// forbidLocalURLAccess 把本地 DNS 解析、HTTP 传输与子进程全部配置为一访问
// 即失败，证明 Claude URL 路径不经过本地下载管线。
func (e *env) forbidLocalURLAccess(t *testing.T) {
	t.Helper()
	e.handler.Resolver = func(context.Context, string) ([]net.IP, error) {
		return nil, errors.New("Claude URL 导入不得解析本地 DNS")
	}
	e.handler.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("Claude URL 导入不得发起本地 HTTP")
	})
	e.fake.OnRun = func(context.Context, proc.Cmd) (*proc.Result, error) {
		return nil, errors.New("Claude URL 导入不得调用本地外部工具")
	}
}

// assertNoActiveDocs 断言课时下没有任何 active 的 raw_text/teacher_outline/
// student_notes 文档（失败任务的 all-or-nothing 边界）。
func assertNoActiveDocs(t *testing.T, e *env, lessonID uuid.UUID) {
	t.Helper()
	for _, kind := range []string{document.KindRawText, document.KindTeacherOutline, document.KindStudentNotes} {
		if _, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, kind); err == nil {
			t.Fatalf("失败任务不应发布 %s 文档", kind)
		}
	}
}

// assertNoSentinels 断言该 Job 的 ai_runs 记录与最终 error/result 不含任何哨兵。
func assertNoSentinels(t *testing.T, pool *pgxpool.Pool, jobID uuid.UUID, sentinels ...string) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT coalesce(model,'')||' '||coalesce(prompt_template_version,'')||' '||
			coalesce(schema_version,'')||' '||coalesce(prompt_hash,'')||' '||
			coalesce(anthropic_request_id,'')||' '||coalesce(stop_reason,'')||' '||
			coalesce(error_type,'')||' '||purpose
		FROM ai_runs WHERE job_id = $1`, jobID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	runsBlob := ""
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		runsBlob += s + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	j := getJob(t, pool, jobID)
	jobBlob := string(j.Result)
	if j.ErrorCode != nil {
		jobBlob += " " + *j.ErrorCode
	}
	if j.ErrorDetail != nil {
		jobBlob += " " + *j.ErrorDetail
	}
	for _, s := range sentinels {
		if strings.Contains(runsBlob, s) {
			t.Fatalf("ai_runs 泄漏哨兵 %q", s)
		}
		if strings.Contains(jobBlob, s) {
			t.Fatalf("Job error/result 泄漏哨兵 %q", s)
		}
	}
}

// TestTextImportSuccess：文本素材全链路成功——raw_text 文档发布、
// 素材/课时推进、阶段事件序列、result 摘要。
func TestTextImportSuccess(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	materialID, jobID := e.newTextImport(t, lessonID, "第一章 极限\r\n\r\n\r\n  极限的  定义。")

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusSucceeded {
		t.Fatalf("任务应成功: status=%s err=%v", j.Status, j.ErrorCode)
	}
	// raw_text 文档已发布且内容规范化。
	d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}
	if d.ContentMD != "第一章 极限\n\n 极限的 定义。" {
		t.Fatalf("文档内容未按 norm/v1 规范化: %q", d.ContentMD)
	}
	if d.Revision != 1 || d.SourceJobID == nil || *d.SourceJobID != jobID {
		t.Fatalf("文档版本/来源错误: %+v", d)
	}
	// 素材 ready + 元数据版本记录。
	m := getMaterial(t, e.pool, materialID)
	if m.Status != "ready" {
		t.Fatalf("素材应 ready: %s", m.Status)
	}
	var meta map[string]any
	if err := json.Unmarshal(m.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta["parser_version"] != "txt/v1" || meta["normalize_version"] != "norm/v1" || meta["chunk_version"] != "chunk/v1" {
		t.Fatalf("解析器版本记录错误: %v", meta)
	}
	// 课时 ready。
	if l := getLesson(t, e.pool, lessonID); l.Status != "ready" {
		t.Fatalf("课时应 ready: %s", l.Status)
	}
	// result 摘要（小体积，含可复现校验 hash）。
	var result map[string]any
	if err := json.Unmarshal(j.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result["document_id"] != d.ID.String() || result["chunk_count"].(float64) != 1 {
		t.Fatalf("result 摘要错误: %v", result)
	}
	if _, ok := result["chunks_sha256"].(string); !ok {
		t.Fatalf("result 缺少 chunks_sha256: %v", result)
	}
	// 阶段事件序列。
	stages := strings.Join(jobStages(t, e.pool, jobID), " ")
	for _, want := range []string{
		"job.queued:queued", "job.started:queued",
		"job.progress:acquiring", "job.progress:detecting", "job.progress:extracting",
		"job.progress:normalizing", "job.progress:chunking", "job.validating:validating",
		"job.progress:publishing", "job.succeeded:succeeded",
	} {
		if !strings.Contains(stages, want) {
			t.Fatalf("事件序列缺少 %s: %s", want, stages)
		}
	}
	// 任务临时目录已清理。
	if _, err := readDirStat(e.store.TmpDirFor(jobID)); err == nil {
		t.Fatalf("任务临时目录应已清理: %s", e.store.TmpDirFor(jobID))
	}
}

// TestUploadTxtImportSuccess：上传 TXT 走两阶段发布接入 + 导入成功。
func TestUploadTxtImportSuccess(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	materialID, jobID := e.newUploadImport(t, lessonID, "notes.txt", []byte("上传的文本内容。"), nil)

	e.runOnce(t)

	if j := getJob(t, e.pool, jobID); j.Status != job.StatusSucceeded {
		t.Fatalf("任务应成功: %s", j.Status)
	}
	m := getMaterial(t, e.pool, materialID)
	if m.Status != "ready" {
		t.Fatalf("素材应 ready: %s", m.Status)
	}
	d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}
	if d.ContentMD != "上传的文本内容。" {
		t.Fatalf("文档内容错误: %q", d.ContentMD)
	}
}

// TestDuplicateUploadsBothSucceed：重复文件——两次上传相同内容各自成功，
// 产生两个同 hash 不同 ID 的 ready 对象，文档 revision 递增。
func TestDuplicateUploadsBothSucceed(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	content := []byte("完全相同的素材内容。")

	m1, j1 := e.newUploadImport(t, lessonID, "a.txt", content, nil)
	e.runOnce(t)
	m2, j2 := e.newUploadImport(t, lessonID, "b.txt", content, nil)
	e.runOnce(t)

	if getJob(t, e.pool, j1).Status != job.StatusSucceeded || getJob(t, e.pool, j2).Status != job.StatusSucceeded {
		t.Fatal("两个任务都应成功")
	}
	mm1 := getMaterial(t, e.pool, m1)
	mm2 := getMaterial(t, e.pool, m2)
	o1, err := e.store.GetByID(context.Background(), e.pool, *mm1.FileObjectID)
	if err != nil {
		t.Fatal(err)
	}
	o2, err := e.store.GetByID(context.Background(), e.pool, *mm2.FileObjectID)
	if err != nil {
		t.Fatal(err)
	}
	if o1.ID == o2.ID || o1.SHA256 != o2.SHA256 || o1.State != "ready" || o2.State != "ready" {
		t.Fatalf("重复文件应各自独立发布: %+v / %+v", o1, o2)
	}
	d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}
	if d.Revision != 2 {
		t.Fatalf("重复导入应产生 revision 2: %d", d.Revision)
	}
}

// TestImportTimeout：Claude 提取挂起 + 任务总时限 -> job_timeout 可重试（回 queued）。
func TestImportTimeout(t *testing.T) {
	e := newEnv(t)
	e.runner.JobTimeout = 300 * time.Millisecond
	fakeAI := e.useClaudeURLImport(t)
	fakeAI.OnGenerate = func(ctx context.Context, _ ai.Request) (ai.Response, error) {
		<-ctx.Done()
		return ai.Response{}, ctx.Err()
	}
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusQueued {
		t.Fatalf("可重试超时应回 queued: %s", j.Status)
	}
	if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeJobTimeout {
		t.Fatalf("错误码应 job_timeout: %v", j.ErrorCode)
	}
}

// TestImportCancelRequested：执行中请求取消 -> cancelled；素材保持 pending，
// 课时回 draft（无发布文档）。
func TestImportCancelRequested(t *testing.T) {
	e := newEnv(t)
	fakeAI := e.useClaudeURLImport(t)
	fakeAI.OnGenerate = func(ctx context.Context, _ ai.Request) (ai.Response, error) {
		<-ctx.Done()
		return ai.Response{}, ctx.Err()
	}
	lessonID := e.newLesson(t)
	materialID, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		e.runOnce(t)
	}()
	// 等任务进入 running 后请求取消。
	waitFor(t, func() bool { return getJob(t, e.pool, jobID).Status == job.StatusRunning })
	if err := (&job.Repository{}).RequestCancel(context.Background(), e.pool, jobID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("任务未及时结束")
	}

	if j := getJob(t, e.pool, jobID); j.Status != job.StatusCancelled {
		t.Fatalf("任务应 cancelled: %s", j.Status)
	}
	if m := getMaterial(t, e.pool, materialID); m.Status != "pending" {
		t.Fatalf("取消后素材应保持 pending: %s", m.Status)
	}
	if l := getLesson(t, e.pool, lessonID); l.Status != "draft" {
		t.Fatalf("取消后课时应回 draft: %s", l.Status)
	}
	stages := strings.Join(jobStages(t, e.pool, jobID), " ")
	if !strings.Contains(stages, "job.cancel_requested") || !strings.Contains(stages, "job.cancelled") {
		t.Fatalf("缺少取消事件: %s", stages)
	}
}

// TestClaudeURLImportSuccess：URL 素材全链路成功——Claude Web Fetch 提取、
// 恰好发布 raw_text+teacher_outline+student_notes、来源元数据标记
// claude_web_fetch、ai_runs 恰好一条 lesson_url_extract，全程不触碰本地
// DNS/HTTP/子进程，哨兵不落入 ai_runs 与 Job result。
func TestClaudeURLImportSuccess(t *testing.T) {
	e := newEnv(t)
	e.forbidLocalURLAccess(t)
	fakeAI := e.useClaudeURLImport(t)
	lessonID := e.newLesson(t)
	// 课时标题注入 prompt/key 哨兵：只进入文档生成 prompt，绝不持久化。
	if _, err := e.pool.Exec(context.Background(), `UPDATE lessons SET title = $1 WHERE id = $2`,
		"极限入门 "+promptSentinel+" "+keyLikeSentinel, lessonID); err != nil {
		t.Fatal(err)
	}
	rawURL := "https://www.youtube.com/watch?v=abc&utm_source=" + urlQuerySentinel
	materialID, jobID := e.newURLImport(t, lessonID, rawURL, nil)

	fakeAI.Enqueue(ai.Response{
		JSON:              json.RawMessage(`{"title":"极限入门","raw_text_markdown":"` + rawSourceSentinel + ` 大家好，今天讲极限","extraction_note":"含字幕原文"}`),
		Model:             "claude-test",
		ProviderRequestID: "req_extract",
	})
	fakeAI.Enqueue(ai.Response{
		JSON:              json.RawMessage(`{"markdown":"# 教师教纲\n围绕极限概念授课。"}`),
		Model:             "claude-test",
		ProviderRequestID: "req_teacher",
	})
	fakeAI.Enqueue(ai.Response{
		JSON:              json.RawMessage(`{"markdown":"# 学生笔记\n极限描述趋势。"}`),
		Model:             "claude-test",
		ProviderRequestID: "req_student",
	})

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusSucceeded {
		t.Fatalf("任务应成功: %s %v", j.Status, j.ErrorCode)
	}

	// 恰好发布三份文档。
	rawDoc, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatalf("raw_text 未发布: %v", err)
	}
	if rawDoc.ContentMD != rawSourceSentinel+" 大家好，今天讲极限" {
		t.Fatalf("raw_text 内容错误: %q", rawDoc.ContentMD)
	}
	for _, kind := range []string{document.KindTeacherOutline, document.KindStudentNotes} {
		d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, kind)
		if err != nil {
			t.Fatalf("%s 未发布: %v", kind, err)
		}
		if d.SourceJobID == nil || *d.SourceJobID != jobID {
			t.Fatalf("%s source_job_id 错误: %+v", kind, d)
		}
	}

	// 恰好三次 Claude 调用，第一次是 canonical URL 的 Web Fetch 提取。
	reqs := fakeAI.Requests()
	if len(reqs) != 3 {
		t.Fatalf("Claude 调用次数 = %d, want 3", len(reqs))
	}
	extract := reqs[0]
	if extract.Purpose != ai.PurposeLessonURLExtract || extract.WebFetch == nil {
		t.Fatalf("第一次调用应为 lesson_url_extract Web Fetch 请求: %+v", extract.Purpose)
	}
	if extract.WebFetch.URL != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("Web Fetch URL 应为剥离追踪参数的 canonical URL: %q", extract.WebFetch.URL)
	}
	if len(extract.WebFetch.AllowedDomains) != 2 ||
		extract.WebFetch.AllowedDomains[0] != "www.youtube.com" ||
		extract.WebFetch.AllowedDomains[1] != "youtube.com" {
		t.Fatalf("allowed_domains 错误: %v", extract.WebFetch.AllowedDomains)
	}

	// 来源元数据标记 Claude Web Fetch 提取路径。
	m := getMaterial(t, e.pool, materialID)
	if m.Status != "ready" {
		t.Fatalf("素材应 ready: %s", m.Status)
	}
	var meta struct {
		Source struct {
			Title          string `json:"title"`
			WebpageURL     string `json:"webpage_url"`
			Extractor      string `json:"extractor"`
			ExtractionNote string `json:"extraction_note"`
			SubtitleOrigin string `json:"subtitle_origin"`
			YtDlpVersion   string `json:"ytdlp_version"`
		} `json:"source"`
		ParserVersion string `json:"parser_version"`
	}
	if err := json.Unmarshal(m.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Source.Extractor != "claude_web_fetch" {
		t.Fatalf("extractor 应 claude_web_fetch: %q", meta.Source.Extractor)
	}
	if meta.ParserVersion != "claude-web-fetch/v1" {
		t.Fatalf("parser_version 应 claude-web-fetch/v1: %q", meta.ParserVersion)
	}
	if meta.Source.Title != "极限入门" || meta.Source.ExtractionNote != "含字幕原文" {
		t.Fatalf("来源摘要错误: %+v", meta.Source)
	}
	if meta.Source.WebpageURL != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("webpage_url 应为 canonical URL: %q", meta.Source.WebpageURL)
	}
	if meta.Source.SubtitleOrigin != "" || meta.Source.YtDlpVersion != "" {
		t.Fatalf("Claude 路径不应携带本地下载元数据: %+v", meta.Source)
	}

	// ai_runs 恰好一条 lesson_url_extract（另有教纲/笔记各一条）。
	var extractRuns int
	if err := e.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM ai_runs WHERE job_id = $1 AND purpose = 'lesson_url_extract' AND redacted`,
		jobID).Scan(&extractRuns); err != nil {
		t.Fatal(err)
	}
	if extractRuns != 1 {
		t.Fatalf("应恰好记录一条 lesson_url_extract ai_run: %d", extractRuns)
	}

	// 阶段事件包含 Claude 提取与文档生成。
	stages := strings.Join(jobStages(t, e.pool, jobID), " ")
	for _, want := range []string{
		"job.progress:extracting", "job.progress:generating_teacher_outline",
		"job.progress:generating_student_notes", "job.progress:publishing",
	} {
		if !strings.Contains(stages, want) {
			t.Fatalf("事件序列缺少 %s: %s", want, stages)
		}
	}

	// 本地管线从未被调用（stub 一访问即失败，能走到成功即证明）。
	if len(e.fake.Calls()) != 0 {
		t.Fatalf("Claude URL 导入不应调用外部进程: %+v", e.fake.Calls())
	}
	assertNoSentinels(t, e.pool, jobID, urlQuerySentinel, promptSentinel, rawSourceSentinel, keyLikeSentinel)
}

// TestClaudeURLImportNotConfigured：无 URL 专用 Provider 或能力谓词为 false
// 时 -> 非重试 claude_not_configured，不做本地下载降级，不发任何 Claude 调用。
func TestClaudeURLImportNotConfigured(t *testing.T) {
	t.Run("provider missing", func(t *testing.T) {
		e := newEnv(t)
		e.forbidLocalURLAccess(t)
		lessonID := e.newLesson(t)
		_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)

		e.runOnce(t)

		j := getJob(t, e.pool, jobID)
		if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeClaudeNotConfigured {
			t.Fatalf("应 failed(claude_not_configured): %s %v", j.Status, j.ErrorCode)
		}
		assertNoActiveDocs(t, e, lessonID)
	})

	t.Run("provider unavailable", func(t *testing.T) {
		e := newEnv(t)
		e.forbidLocalURLAccess(t)
		fakeAI := ai.NewFakeProvider()
		e.handler.URLImportAI = fakeAI
		e.handler.URLImportAvailable = func() bool { return false }
		lessonID := e.newLesson(t)
		_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)

		e.runOnce(t)

		j := getJob(t, e.pool, jobID)
		if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeClaudeNotConfigured {
			t.Fatalf("应 failed(claude_not_configured): %s %v", j.Status, j.ErrorCode)
		}
		if len(fakeAI.Requests()) != 0 {
			t.Fatalf("能力谓词为 false 时不应发起 Claude 调用: %d", len(fakeAI.Requests()))
		}
		assertNoActiveDocs(t, e, lessonID)
	})
}

// TestClaudeURLImportSSRFBlocked：Worker 侧再校验发现非白名单主机 ->
// url_blocked 终态（不重试），不调用 Claude 也不调用本地工具。
func TestClaudeURLImportSSRFBlocked(t *testing.T) {
	e := newEnv(t)
	e.forbidLocalURLAccess(t)
	fakeAI := e.useClaudeURLImport(t)
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://evil.example.com/watch?v=x", nil)

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed {
		t.Fatalf("不可重试错误应直接 failed: %s", j.Status)
	}
	if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeURLBlocked {
		t.Fatalf("错误码应 url_blocked: %v", j.ErrorCode)
	}
	if len(fakeAI.Requests()) != 0 {
		t.Fatalf("URL 校验失败前不应调用 Claude: %d", len(fakeAI.Requests()))
	}
	if len(e.fake.Calls()) != 0 {
		t.Fatalf("URL 校验失败前不应调用外部进程: %+v", e.fake.Calls())
	}
	assertNoActiveDocs(t, e, lessonID)
}

// TestClaudeURLImportMaterialFetchFailed：Claude 无法提取链接内容 ->
// material_fetch_failed；不可重试直接终态，可重试回 queued。
// 上游错误原因的哨兵不得落入 ai_runs 或 Job error/result。
func TestClaudeURLImportMaterialFetchFailed(t *testing.T) {
	t.Run("non-retryable terminal", func(t *testing.T) {
		e := newEnv(t)
		e.forbidLocalURLAccess(t)
		fakeAI := e.useClaudeURLImport(t)
		lessonID := e.newLesson(t)
		materialID, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
		fakeAI.EnqueueError(ai.WrapError(ai.ErrMaterialFetchFailed,
			"Claude 无法提取该链接内容。",
			errors.New("upstream echoed "+keyLikeSentinel+" and "+rawSourceSentinel)))

		e.runOnce(t)

		j := getJob(t, e.pool, jobID)
		if j.Status != job.StatusFailed {
			t.Fatalf("不可重试提取失败应 failed: %s", j.Status)
		}
		if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeMaterialFetchFailed {
			t.Fatalf("错误码应 material_fetch_failed: %v", j.ErrorCode)
		}
		m := getMaterial(t, e.pool, materialID)
		if m.Status != "failed" {
			t.Fatalf("素材应 failed: %s", m.Status)
		}
		var meta map[string]any
		if err := json.Unmarshal(m.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if meta["error_code"] != worker.ErrCodeMaterialFetchFailed {
			t.Fatalf("素材元数据应记录错误码: %v", meta)
		}
		if l := getLesson(t, e.pool, lessonID); l.Status != "failed" {
			t.Fatalf("课时应 failed: %s", l.Status)
		}
		// 失败的提取仍记录一条脱敏 ai_run（只有错误码，无上游文本）。
		var errorType *string
		if err := e.pool.QueryRow(context.Background(),
			`SELECT error_type FROM ai_runs WHERE job_id = $1 AND purpose = 'lesson_url_extract' AND redacted`,
			jobID).Scan(&errorType); err != nil {
			t.Fatal(err)
		}
		if errorType == nil || *errorType != worker.ErrCodeMaterialFetchFailed {
			t.Fatalf("ai_run error_type 应 material_fetch_failed: %v", errorType)
		}
		assertNoActiveDocs(t, e, lessonID)
		assertNoSentinels(t, e.pool, jobID, keyLikeSentinel, rawSourceSentinel)
	})

	t.Run("retryable requeues", func(t *testing.T) {
		e := newEnv(t)
		e.forbidLocalURLAccess(t)
		fakeAI := e.useClaudeURLImport(t)
		lessonID := e.newLesson(t)
		_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
		fakeAI.EnqueueError(ai.MaterialFetchError(true))

		e.runOnce(t)

		j := getJob(t, e.pool, jobID)
		if j.Status != job.StatusQueued {
			t.Fatalf("可重试提取失败应回 queued: %s", j.Status)
		}
		if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeMaterialFetchFailed {
			t.Fatalf("错误码应 material_fetch_failed: %v", j.ErrorCode)
		}
		assertNoActiveDocs(t, e, lessonID)
	})
}

// TestClaudeURLImportTooLarge：Claude 提取的正文超过上限 ->
// material_too_large 终态，不发布任何文档。
func TestClaudeURLImportTooLarge(t *testing.T) {
	e := newEnv(t)
	e.forbidLocalURLAccess(t)
	fakeAI := e.useClaudeURLImport(t)
	e.handler.MaxTextRunes = 10
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	fakeAI.Enqueue(ai.Response{
		JSON:  json.RawMessage(`{"title":"极限入门","raw_text_markdown":"这段文本远远超出允许的字符上限。","extraction_note":""}`),
		Model: "claude-test",
	})

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeTooLarge {
		t.Fatalf("应 failed(material_too_large): %s %v", j.Status, j.ErrorCode)
	}
	assertNoActiveDocs(t, e, lessonID)
}

// TestClaudeURLImportDocumentFailure：提取成功但任一文档生成失败 ->
// 不发布任何文档（raw_text 也不发布，all-or-nothing）。
func TestClaudeURLImportDocumentFailure(t *testing.T) {
	e := newEnv(t)
	e.forbidLocalURLAccess(t)
	fakeAI := e.useClaudeURLImport(t)
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	fakeAI.Enqueue(ai.Response{
		JSON:  json.RawMessage(`{"title":"极限入门","raw_text_markdown":"大家好，今天讲极限","extraction_note":""}`),
		Model: "claude-test",
	})
	fakeAI.EnqueueError(ai.ErrorForCode(ai.ErrRefused, "Claude 拒绝了文档生成请求。"))

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != ai.ErrRefused {
		t.Fatalf("应 failed(claude_refused): %s %v", j.Status, j.ErrorCode)
	}
	assertNoActiveDocs(t, e, lessonID)
}

// TestWorkerLostRecovery：Worker 丢失（租约过期）-> Recovery 回收重试 ->
// 重跑成功，不产生重复文档版本。
func TestWorkerLostRecovery(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	_, jobID := e.newTextImport(t, lessonID, "恢复测试文本。")

	// 模拟 Worker 崩溃：领取后不做任何处理，租约很快过期。
	ctx := context.Background()
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := (&job.Repository{}).Claim(ctx, tx, "crashed-worker", 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || claimed.ID != jobID {
		t.Fatal("应领取到目标任务")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)

	// Recovery 回收（退避置零便于立即重跑）。
	repo := &job.Repository{Backoff: func(int32) time.Duration { return 0 }}
	tx, err = e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requeued, failed, cancelled, err := repo.RecoverExpiredLeases(ctx, tx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if requeued != 1 || failed != 0 || cancelled != 0 {
		t.Fatalf("回收结果错误: requeued=%d failed=%d cancelled=%d", requeued, failed, cancelled)
	}
	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusQueued || j.ErrorCode == nil || *j.ErrorCode != job.ErrCodeWorkerLost {
		t.Fatalf("应回收为 queued(worker_lost): %s %v", j.Status, j.ErrorCode)
	}

	// 重跑成功；文档只有一个版本（崩溃未产生半成品）。
	e.runOnce(t)
	if j := getJob(t, e.pool, jobID); j.Status != job.StatusSucceeded {
		t.Fatalf("重跑应成功: %s", j.Status)
	}
	d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}
	if d.Revision != 1 {
		t.Fatalf("崩溃恢复后不应产生重复版本: %d", d.Revision)
	}
}

// TestUnsupportedJobKind：未注册 kind 快速失败（B6/B7 补注册）。
func TestUnsupportedJobKind(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	j := &job.Job{
		Kind:          job.KindLessonExport,
		AggregateType: "lesson",
		AggregateID:   lessonID,
		Payload:       []byte(`{}`),
	}
	if err := (&job.Repository{}).Create(context.Background(), e.pool, j); err != nil {
		t.Fatal(err)
	}

	e.runOnce(t)

	got := getJob(t, e.pool, j.ID)
	if got.Status != job.StatusFailed || got.ErrorCode == nil || *got.ErrorCode != worker.ErrCodeUnsupportedKind {
		t.Fatalf("应 failed(unsupported_job_kind): %s %v", got.Status, got.ErrorCode)
	}
}

// TestTextTooLarge：文本超过上限 -> material_too_large 终态。
func TestTextTooLarge(t *testing.T) {
	e := newEnv(t)
	e.handler.MaxTextRunes = 10
	lessonID := e.newLesson(t)
	_, jobID := e.newTextImport(t, lessonID, "这段文本远远超出允许的字符上限。")

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeTooLarge {
		t.Fatalf("应 failed(material_too_large): %s %v", j.Status, j.ErrorCode)
	}
}

// TestEmptyContent：清洗后无文本 -> material_empty_content 终态。
func TestEmptyContent(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	_, jobID := e.newTextImport(t, lessonID, "   \n\n\n  ")

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeEmptyContent {
		t.Fatalf("应 failed(material_empty_content): %s %v", j.Status, j.ErrorCode)
	}
}

// TestFailureKeepsActiveDocument：已有发布文档时，后续导入失败/回滚
// 不改变 active 文档（发布单事务语义）；素材/课时侧写按 D11 推进。
func TestFailureKeepsActiveDocument(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)

	// 第一次导入成功：raw_text revision 1 active。
	_, jobID1 := e.newTextImport(t, lessonID, "第一版文档内容。")
	e.runOnce(t)
	if j := getJob(t, e.pool, jobID1); j.Status != job.StatusSucceeded {
		t.Fatalf("第一次导入应成功: %s", j.Status)
	}
	before, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}

	// 第二次导入失败（URL 素材 Claude 提取硬错误）。
	e.forbidLocalURLAccess(t)
	fakeAI := e.useClaudeURLImport(t)
	_, jobID2 := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	fakeAI.EnqueueError(ai.MaterialFetchError(false))
	e.runOnce(t)
	if j := getJob(t, e.pool, jobID2); j.Status != job.StatusFailed {
		t.Fatalf("第二次导入应失败: %s", j.Status)
	}

	// active 文档仍是 revision 1，内容不变。
	after, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.Revision != 1 || after.ContentMD != "第一版文档内容。" {
		t.Fatalf("失败不应改变 active 文档: %+v", after)
	}
}
