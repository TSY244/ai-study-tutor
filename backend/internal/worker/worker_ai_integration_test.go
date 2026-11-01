package worker_test

import (
	"context"
	"encoding/json"
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
