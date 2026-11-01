package worker_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func TestBilibiliImportPassesTextAndVisualSummaryToLessonGeneration(t *testing.T) {
	e := newEnv(t)
	fakeAI, cli := e.useClaudeURLImport(t)
	lessonID := e.newLesson(t)
	materialID, jobID := e.newURLImport(t, lessonID,
		"https://www.bilibili.com/video/BV1v34y1j7Nu?p=2&vd_source="+urlQuerySentinel, nil)
	cli.Enqueue(func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if cmd.Name != "bili" || !strings.Contains(strings.Join(cmd.Args, " "), "--page 2") {
			t.Fatalf("Bilibili must acquire the requested page with bili-cli: %s %v", cmd.Name, cmd.Args)
		}
		var profile string
		for _, kv := range cmd.Env {
			if strings.HasPrefix(kv, "BILI_CLI_HOME=") {
				profile = strings.TrimPrefix(kv, "BILI_CLI_HOME=")
			}
		}
		if profile == "" {
			t.Fatal("missing task-local bili profile")
		}
		sheet := filepath.Join(profile, "sheet.jpg")
		if err := os.WriteFile(sheet, []byte("frame-fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(map[string]any{"ok": true, "data": map[string]any{
			"title": "极限与图像", "frames_sheet": sheet,
			"transcript": map[string]string{"source": "subtitle:zh-CN", "text": rawSourceSentinel + " 极限表示趋近的值。"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return &proc.Result{Stdout: raw}, nil
	})
	cli.Enqueue(proc.Static(claudeCLISuccessEnvelope(t,
		`{"title":"极限与图像","raw_text_markdown":"## 画面内容\n[00:30] 曲线趋近虚线。\n\n## 综合总结\n曲线与口述共同说明极限。\n\n### 内容概览\n围绕极限展开学习。\n\n### 核心知识点\n极限描述趋近关系。\n\n### 知识结构\n从直观图像到概念。\n\n### 示例与应用\n资料未提供示例。\n\n### 结论与复习\n复习：什么是极限？\n\n### 来源与局限\n依据给定资料，无其他来源。","extraction_note":"画面已采样"}`), nil))
	for _, text := range []string{"教师教纲", "学生笔记"} {
		b, _ := json.Marshal(map[string]string{"markdown": "# " + text + "\n极限的几何意义。" + "\n\n## 综合总结\n\n### 内容概览\n围绕极限展开学习。\n\n### 核心知识点\n极限描述趋近关系。\n\n### 知识结构\n从直观图像到概念。\n\n### 示例与应用\n资料未提供示例。\n\n### 结论与复习\n复习：什么是极限？\n\n### 来源与局限\n依据给定资料，无其他来源。"})
		fakeAI.Enqueue(ai.Response{JSON: b, Model: "claude-test"})
	}
	e.runOnce(t)
	if j := getJob(t, e.pool, jobID); j.Status != job.StatusSucceeded {
		t.Fatalf("Bilibili import failed: status=%s code=%v detail=%v", j.Status, j.ErrorCode, j.ErrorDetail)
	}
	for _, req := range fakeAI.Requests() {
		for _, text := range []string{rawSourceSentinel, "曲线趋近虚线", "曲线与口述共同说明极限"} {
			if !strings.Contains(req.User, text) {
				t.Fatalf("downstream %s missing text/visual evidence %q", req.Purpose, text)
			}
		}
	}
	if len(fakeAI.Requests()) != 2 {
		t.Fatal("both downstream lesson documents must be generated")
	}
	for _, kind := range []string{document.KindRawText, document.KindTeacherOutline, document.KindStudentNotes} {
		if _, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, kind); err != nil {
			t.Fatalf("missing published %s: %v", kind, err)
		}
	}
	m := getMaterial(t, e.pool, materialID)
	var meta struct {
		ParserVersion string `json:"parser_version"`
		Source        struct {
			Extractor string `json:"extractor"`
		} `json:"source"`
	}
	if err := json.Unmarshal(m.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.ParserVersion != "bili-cli-multimodal/v1" || meta.Source.Extractor != "bili-cli" {
		t.Fatalf("wrong Bilibili provenance: %+v", meta)
	}
	assertNoSentinels(t, e.pool, jobID, rawSourceSentinel, urlQuerySentinel, keyLikeSentinel)
}
