package worker_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

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

// TestImportTimeout：子进程挂起 + 任务总时限 -> job_timeout 可重试（回 queued）。
func TestImportTimeout(t *testing.T) {
	e := newEnv(t)
	e.runner.JobTimeout = 300 * time.Millisecond
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	e.fake.OnRun = proc.BlockUntilDone()

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
	lessonID := e.newLesson(t)
	materialID, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	e.fake.OnRun = proc.BlockUntilDone()

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

// TestImportNonZeroExit：yt-dlp 非零退出（通用错误）-> 终态 failed，
// 素材 failed + 课时 failed + error_code 落元数据。
func TestImportNonZeroExit(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	materialID, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	e.fake.OnRun = proc.Static(nil, &proc.ExitError{Code: 1, StderrTail: "some hard failure"})

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed {
		t.Fatalf("任务应 failed: %s", j.Status)
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
	// 失败不改变 active 文档（本例无文档可查）。
	if _, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText); err == nil {
		t.Fatal("失败任务不应发布文档")
	}
}

// TestImportSSRFBlocked：Worker 二次校验发现私网地址 -> url_blocked 终态（不重试）。
func TestImportSSRFBlocked(t *testing.T) {
	e := newEnv(t)
	e.handler.Resolver = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.8")}, nil
	}
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed {
		t.Fatalf("不可重试错误应直接 failed: %s", j.Status)
	}
	if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeURLBlocked {
		t.Fatalf("错误码应 url_blocked: %v", j.ErrorCode)
	}
	// 未调用任何外部进程。
	if len(e.fake.Calls()) != 0 {
		t.Fatalf("SSRF 拦截前不应调用外部进程: %+v", e.fake.Calls())
	}
}

// TestTranscriptRequired：无字幕且 subtitle_only -> 可行动 transcript_required。
func TestTranscriptRequired(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	e.fake.OnRun = proc.Static(&proc.Result{}, nil) // 全部成功但无字幕文件

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeTranscriptRequired {
		t.Fatalf("应 failed(transcript_required): %s %v", j.Status, j.ErrorCode)
	}
	if j.ErrorDetail == nil || !strings.Contains(*j.ErrorDetail, "字幕") {
		t.Fatalf("应给出可行动建议: %v", j.ErrorDetail)
	}
}

// TestSubtitleDownloadSuccess：yt-dlp 人工字幕成功 -> 清洗文本发布 + 来源元数据。
func TestSubtitleDownloadSuccess(t *testing.T) {
	e := newEnv(t)
	lessonID := e.newLesson(t)
	materialID, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	e.fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if len(cmd.Args) > 0 && cmd.Args[0] == "--version" {
			return &proc.Result{Stdout: []byte("2026.01.01\n")}, nil
		}
		// 人工字幕下载：在工作目录落盘 vtt 与 info.json。
		writeFile(t, cmd.Dir, "src.zh-Hans.vtt", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n大家好，今天讲极限\n")
		writeFile(t, cmd.Dir, "src.info.json", `{"title":"极限入门","duration":60,"webpage_url":"https://www.youtube.com/watch?v=x","extractor":"youtube"}`)
		return &proc.Result{}, nil
	}

	e.runOnce(t)

	if j := getJob(t, e.pool, jobID); j.Status != job.StatusSucceeded {
		t.Fatalf("任务应成功: %s", j.Status)
	}
	d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}
	if d.ContentMD != "大家好，今天讲极限" {
		t.Fatalf("字幕清洗结果错误: %q", d.ContentMD)
	}
	m := getMaterial(t, e.pool, materialID)
	var meta struct {
		Source struct {
			Title          string `json:"title"`
			SubtitleOrigin string `json:"subtitle_origin"`
			YtDlpVersion   string `json:"ytdlp_version"`
		} `json:"source"`
		ParserVersion string `json:"parser_version"`
	}
	if err := json.Unmarshal(m.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Source.Title != "极限入门" || meta.Source.SubtitleOrigin != "manual" || meta.Source.YtDlpVersion != "2026.01.01" {
		t.Fatalf("来源元数据错误: %+v", meta)
	}
	if meta.ParserVersion != "subtitle/v1" {
		t.Fatalf("解析器版本错误: %s", meta.ParserVersion)
	}
}

// TestWhisperLocalSuccess：无字幕 + whisper_local -> 音轨下载 + 转写成功。
func TestWhisperLocalSuccess(t *testing.T) {
	e := newEnv(t)
	e.handler.WhisperModelPath = "/models/ggml-small.bin"
	lessonID := e.newLesson(t)
	_, jobID := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x",
		map[string]any{"transcription_mode": "whisper_local"})
	e.fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
		switch {
		case len(cmd.Args) > 0 && cmd.Args[0] == "--version":
			return &proc.Result{Stdout: []byte("2026.01.01\n")}, nil
		case len(cmd.Args) > 0 && cmd.Args[0] == "-x":
			writeFile(t, cmd.Dir, "audio.wav", "fake-wav")
			return &proc.Result{}, nil
		case cmd.Name == "whisper-cli":
			// -of <prefix> -> 写 <prefix>.txt（prefix 为绝对路径）。
			for i, a := range cmd.Args {
				if a == "-of" && i+1 < len(cmd.Args) {
					if err := os.WriteFile(cmd.Args[i+1]+".txt", []byte("转写出的课程文本"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			return &proc.Result{}, nil
		}
		return &proc.Result{}, nil // 字幕下载无产出
	}

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusSucceeded {
		t.Fatalf("任务应成功: %s err=%v", j.Status, j.ErrorCode)
	}
	d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, document.KindRawText)
	if err != nil {
		t.Fatal(err)
	}
	if d.ContentMD != "转写出的课程文本" {
		t.Fatalf("转写文本错误: %q", d.ContentMD)
	}
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

	// 第二次导入失败（URL 素材下载硬错误）。
	_, jobID2 := e.newURLImport(t, lessonID, "https://www.youtube.com/watch?v=x", nil)
	e.fake.OnRun = proc.Static(nil, &proc.ExitError{Code: 1, StderrTail: "hard failure"})
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
