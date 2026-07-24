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
	materialID, jobID := e.newTextImport(t, lessonID, "极限描述函数在某一点附近的趋势。")
	fakeAI.Enqueue(ai.Response{JSON: json.RawMessage(`{"markdown":"# 教师教纲\n围绕极限概念授课。"}`), Model: "fake-claude", ProviderRequestID: "req_teacher"})
	fakeAI.Enqueue(ai.Response{JSON: json.RawMessage(`{"markdown":"# 学生笔记\n极限描述趋势。"}`), Model: "fake-claude", ProviderRequestID: "req_student"})

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
