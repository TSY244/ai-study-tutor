package worker_test

import (
	"context"
	"encoding/json"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
)

func TestLessonImportWithAIProviderPublishesAllDocuments(t *testing.T) {
	e := newEnv(t)
	fakeAI := ai.NewFakeProvider()
	e.handler.AI = fakeAI
	lessonID := e.newLesson(t)
	if _, err := e.pool.Exec(context.Background(), `UPDATE courses SET title='待 AI 命名', title_pending=true WHERE id=(SELECT course_id FROM lessons WHERE id=$1)`, lessonID); err != nil {
		t.Fatal(err)
	}
	materialID, jobID := e.newTextImport(t, lessonID, "极限描述函数在某一点附近的趋势。")
	fakeAI.Enqueue(ai.Response{JSON: json.RawMessage(`{"markdown":"# 极限入门\n围绕极限概念授课。\n\n## 综合总结\n\n### 内容概览\n围绕极限展开学习。\n\n### 核心知识点\n极限描述趋近关系。\n\n### 知识结构\n从直观图像到概念。\n\n### 示例与应用\n资料未提供示例。\n\n### 结论与复习\n复习：什么是极限？\n\n### 来源与局限\n依据给定资料，无其他来源。"}`), Model: "fake-claude", ProviderRequestID: "req_teacher"})
	fakeAI.Enqueue(ai.Response{JSON: json.RawMessage(`{"markdown":"# 学生笔记\n极限描述趋势。\n\n## 综合总结\n\n### 内容概览\n围绕极限展开学习。\n\n### 核心知识点\n极限描述趋近关系。\n\n### 知识结构\n从直观图像到概念。\n\n### 示例与应用\n资料未提供示例。\n\n### 结论与复习\n复习：什么是极限？\n\n### 来源与局限\n依据给定资料，无其他来源。"}`), Model: "fake-claude", ProviderRequestID: "req_student"})

	e.runOnce(t)

	if j := getJob(t, e.pool, jobID); j.Status != "succeeded" {
		t.Fatalf("lesson_import 应成功: %s %v", j.Status, j.ErrorCode)
	}
	for _, kind := range []string{document.KindRawText, document.KindTeacherOutline, document.KindStudentNotes} {
		d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, kind)
		if err != nil {
			t.Fatalf("%s 未发布: %v", kind, err)
		}
		if d.SourceJobID == nil || *d.SourceJobID != jobID {
			t.Fatalf("%s source_job_id 错误: %+v", kind, d)
		}
	}
	var courseTitle string
	var titlePending bool
	if err := e.pool.QueryRow(context.Background(), `SELECT title,title_pending FROM courses WHERE id=(SELECT course_id FROM lessons WHERE id=$1)`, lessonID).Scan(&courseTitle, &titlePending); err != nil {
		t.Fatal(err)
	}
	if courseTitle != "极限入门" || titlePending {
		t.Fatalf("AI title not published: %q pending=%v", courseTitle, titlePending)
	}
	m := getMaterial(t, e.pool, materialID)
	if m.Status != "ready" {
		t.Fatalf("素材应 ready: %s", m.Status)
	}
	var aiRuns int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM ai_runs WHERE job_id = $1 AND redacted`, jobID).Scan(&aiRuns); err != nil {
		t.Fatal(err)
	}
	if aiRuns != 2 {
		t.Fatalf("lesson_import 应记录两条脱敏 ai_run: %d", aiRuns)
	}
	stages := strings.Join(jobStages(t, e.pool, jobID), " ")
	for _, want := range []string{"generating_teacher_outline", "generating_student_notes"} {
		if !strings.Contains(stages, want) {
			t.Fatalf("阶段事件缺少 %s: %s", want, stages)
		}
	}
}

func TestAudioTranscriptFeedsAISummaryAndIsRetainedAsRawDocument(t *testing.T) {
	e := newEnv(t)
	lid := e.newLesson(t)
	e.handler.WhisperModelPath = "/models/base.bin"
	transcript := "二分查找要求有序输入。每次比较中间元素，排除一半范围。"
	transcribed := false
	e.fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
		switch cmd.Name {
		case "ffmpeg":
			if strings.Contains(strings.Join(cmd.Args, " "), "0:s:0") {
				return nil, &proc.ExitError{Code: 1}
			}
			return &proc.Result{}, nil
		case "whisper-cli":
			if err := os.WriteFile(filepath.Join(cmd.Dir, "transcript.txt"), []byte(transcript), 0600); err != nil {
				t.Fatal(err)
			}
			transcribed = true
			return &proc.Result{}, nil
		default:
			t.Fatalf("unexpected command %s", cmd.Name)
			return nil, nil
		}
	}
	fakeAI := ai.NewFakeProvider()
	e.handler.AI = fakeAI
	fakeAI.OnGenerate = func(ctx context.Context, req ai.Request) (ai.Response, error) {
		if !transcribed || !strings.Contains(req.User, transcript) {
			t.Fatal("AI must receive ASR text only after transcription")
		}
		markdown := "# 二分查找\n\n## 综合总结\n\n### 内容概览\n学习二分查找。\n\n### 核心知识点\n输入有序。\n\n### 知识结构\n比较后缩小范围。\n\n### 示例与应用\n资料未提供示例。\n\n### 结论与复习\n复习二分查找的前提。\n\n### 来源与局限\n依据音频转写，识别可能有误。"
		raw, _ := json.Marshal(map[string]string{"markdown": markdown})
		return ai.Response{JSON: raw, Model: "test"}, nil
	}
	_, jid := e.newUploadImport(t, lid, "讲课.mp3", append([]byte("ID3"), make([]byte, 512)...), map[string]any{"transcription_mode": "whisper_local", "preferred_language": "zh"})
	e.runOnce(t)
	if j := getJob(t, e.pool, jid); j.Status != "succeeded" {
		t.Fatalf("audio import failed %s %v", j.Status, j.ErrorCode)
	}
	raw, err := (document.Repository{}).GetActive(context.Background(), e.pool, lid, document.KindRawText)
	if err != nil || !strings.Contains(raw.ContentMD, transcript) {
		t.Fatalf("transcript was lost: %v %v", raw, err)
	}
	for _, kind := range []string{document.KindStudentNotes, document.KindTeacherOutline} {
		if _, err := (document.Repository{}).GetActive(context.Background(), e.pool, lid, kind); err != nil {
			t.Fatal(err)
		}
	}
	if len(fakeAI.Requests()) != 2 {
		t.Fatal("must generate both notes and outline")
	}
}
