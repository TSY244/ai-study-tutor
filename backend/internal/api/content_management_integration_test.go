package api_test

import (
	"context"
	"encoding/json"
	"github.com/TSY244/ai-study-tutor/backend/internal/api"
	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type contentEnv struct {
	t              *testing.T
	pool           *pgxpool.Pool
	engine         *gin.Engine
	course, lesson string
}

func newContentEnv(t *testing.T) *contentEnv {
	pool := databasetest.New(t)
	files, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	engine := apitest.NewEngine(t)
	api.RegisterRoutes(engine, api.Deps{Pool: pool, Files: files})
	e := &contentEnv{t: t, pool: pool, engine: engine}
	c := e.call("POST", "/courses", "/courses", map[string]any{"title": "CRUD 测试课程"}, 201)
	e.course = c["id"].(string)
	l := e.call("POST", "/courses/"+e.course+"/lessons", "/courses/{course_id}/lessons", map[string]any{"title": "二分查找", "ordinal": 1}, 201)
	e.lesson = l["id"].(string)
	return e
}
func (e *contentEnv) request(method, path string, body any, key string) *httptest.ResponseRecorder {
	e.t.Helper()
	data := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		data = string(b)
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	return apitest.Do(e.t, e.engine, method, "/api/v1"+path, headers, data)
}
func (e *contentEnv) call(method, path, template string, body any, status int) map[string]any {
	e.t.Helper()
	rec := e.request(method, path, body, uuid.NewString())
	if rec.Code != status {
		e.t.Fatalf("%s %s: got %d want %d: %s", method, path, rec.Code, status, rec.Body.String())
	}
	if status == 204 {
		return nil
	}
	if status < 300 {
		apitest.Load(e.t).ValidateJSONResponse(e.t, method, "/api/v1"+template, status, rec.Body.Bytes())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		e.t.Fatal(err)
	}
	return out
}
func (e *contentEnv) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), query, args...); err != nil {
		e.t.Fatal(err)
	}
}
func (e *contentEnv) settleJobs() {
	e.exec(`UPDATE jobs SET status='succeeded',completed_at=now() WHERE status IN ('queued','running','validating')`)
	e.exec(`UPDATE lessons SET status='ready' WHERE id=$1`, e.lesson)
	e.exec(`UPDATE materials SET status='ready' WHERE lesson_id=$1`, e.lesson)
	e.exec(`UPDATE summary_evaluations SET status='completed',result='{}',completed_at=now() WHERE lesson_id=$1`, e.lesson)
}
func question(stem string) map[string]any {
	return map[string]any{"type": "single_choice", "stem": stem, "options": map[string]string{"A": "有序", "B": "任意顺序"}, "correct_answer": "A", "explanation": "二分查找依赖有序输入。", "tested_concepts": []string{"有序性"}, "difficulty": "easy", "cognitive_level": "apply", "score": 5}
}
func (e *contentEnv) raw(text string) map[string]any {
	return e.call("POST", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", map[string]any{"content_md": text}, 201)
}

func TestContentDocumentAndKnowledgeLifecycle(t *testing.T) {
	e := newContentEnv(t)
	doc := e.raw("数组必须有序。区间为空时目标不存在。")
	pointBody := map[string]any{"label": "有序性", "explanation": "依赖有序数据", "learning_objective": "判断适用前提", "source_excerpt": "数组必须有序", "importance": "core"}
	path := "/lessons/" + e.lesson + "/knowledge-points"
	key := uuid.NewString()
	r1 := e.request("POST", path, pointBody, key)
	r2 := e.request("POST", path, pointBody, key)
	var first, second map[string]any
	_ = json.Unmarshal(r1.Body.Bytes(), &first)
	_ = json.Unmarshal(r2.Body.Bytes(), &second)
	if r1.Code != 201 || r2.Code != 201 || !reflect.DeepEqual(first, second) {
		t.Fatalf("create replay: %d %d", r1.Code, r2.Code)
	}
	var point map[string]any
	json.Unmarshal(r1.Body.Bytes(), &point)
	id := point["id"].(string)
	e.call("GET", "/knowledge-points/"+id, "/knowledge-points/{knowledge_point_id}", nil, 200)
	e.call("PATCH", "/knowledge-points/"+id, "/knowledge-points/{knowledge_point_id}", map[string]any{"version": 1, "explanation": "检验输入有序且比较规则一致"}, 200)
	e.call("PATCH", "/knowledge-points/"+id, "/knowledge-points/{knowledge_point_id}", map[string]any{"version": 1, "label": "过期修改"}, 409)
	e.call("PATCH", "/knowledge-points/"+id, "/knowledge-points/{knowledge_point_id}", map[string]any{"version": 2, "source_excerpt": "原文没有的内容"}, 422)
	e.call("DELETE", "/knowledge-points/"+id, "/knowledge-points/{knowledge_point_id}", nil, 204)
	e.call("GET", "/knowledge-points/"+id, "/knowledge-points/{knowledge_point_id}", nil, 404)
	e.call("POST", path, "/lessons/{lesson_id}/knowledge-points", pointBody, 201)
	e.call("POST", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", map[string]any{"content_md": "重复创建"}, 409)
	e.call("PUT", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", map[string]any{"content_md": "新原文", "expected_revision": 999}, 409)
	next := e.call("PUT", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", map[string]any{"content_md": "有序输入是前提。", "expected_revision": 1}, 200)
	old := e.call("GET", "/documents/"+doc["id"].(string), "/documents/{document_id}", nil, 200)
	if old["content_md"] != "数组必须有序。区间为空时目标不存在。" || old["is_active"] != false {
		t.Fatal("document history changed")
	}
	e.call("DELETE", "/documents/"+next["id"].(string), "/documents/{document_id}", nil, 204)
	e.call("GET", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", nil, 404)
	third := e.raw("删除后新建的原文。")
	if third["revision"] != float64(3) {
		t.Fatal("deleted document revision was reused")
	}
	e.call("PUT", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", map[string]any{"content_md": "过期修改", "expected_revision": 2}, 409)

}
func TestContentQuizRevisionKeepsHistoricalAttempt(t *testing.T) {
	e := newContentEnv(t)
	z := e.call("POST", "/lessons/"+e.lesson+"/quizzes", "/lessons/{lesson_id}/quizzes", map[string]any{"title": "人工测验", "generation_notes": "检验有序性", "questions": []any{question("二分查找的输入要求是什么？")}}, 201)
	id := z["id"].(string)
	e.call("GET", "/quizzes/"+id+"/editor", "/quizzes/{quiz_id}/editor", nil, 200)
	student := e.call("GET", "/quizzes/"+id, "/quizzes/{quiz_id}", nil, 200)
	for _, q := range student["questions"].([]any) {
		if _, ok := q.(map[string]any)["correct_answer"]; ok {
			t.Fatal("student API leaks answer")
		}
	}
	a := e.call("POST", "/quizzes/"+id+"/attempts", "/quizzes/{quiz_id}/attempts", map[string]any{}, 201)
	attemptID := a["id"].(string)
	next := e.call("PATCH", "/quizzes/"+id, "/quizzes/{quiz_id}", map[string]any{"version": 1, "title": "修订测验", "questions": []any{question("哪种输入满足查找前提？")}}, 200)
	nextID := next["id"].(string)
	if nextID == id {
		t.Fatal("questions must be immutable")
	}
	original := e.call("GET", "/attempts/"+attemptID, "/attempts/{attempt_id}", nil, 200)
	if original["questions"].([]any)[0].(map[string]any)["stem"] != "二分查找的输入要求是什么？" {
		t.Fatal("historical question overwritten")
	}
	e.call("PATCH", "/quizzes/"+nextID, "/quizzes/{quiz_id}", map[string]any{"version": 1, "title": "只改标题"}, 200)
	e.call("PATCH", "/quizzes/"+nextID, "/quizzes/{quiz_id}", map[string]any{"version": 1, "title": "过期标题"}, 409)
	e.call("DELETE", "/quizzes/"+id, "/quizzes/{quiz_id}", nil, 204)
	e.call("GET", "/attempts/"+attemptID, "/attempts/{attempt_id}", nil, 404)
	e.call("DELETE", "/quizzes/"+nextID, "/quizzes/{quiz_id}", nil, 204)
	e.call("GET", "/quizzes/"+nextID, "/quizzes/{quiz_id}", nil, 404)
	bad := question("错误答案")
	bad["correct_answer"] = "Z"
	e.call("POST", "/lessons/"+e.lesson+"/quizzes", "/lessons/{lesson_id}/quizzes", map[string]any{"title": "非法", "questions": []any{bad}}, 422)
}
func TestContentMaterialSummaryAndWeaknessLifecycle(t *testing.T) {
	e := newContentEnv(t)
	m := e.call("POST", "/lessons/"+e.lesson+"/materials/text", "/lessons/{lesson_id}/materials/text", map[string]any{"text": "数组有序", "title": "原素材"}, 202)["material"].(map[string]any)
	id := m["id"].(string)
	e.call("GET", "/materials/"+id, "/materials/{material_id}", nil, 200)
	e.call("PATCH", "/materials/"+id, "/materials/{material_id}", map[string]any{"version": 1, "text": "新资料"}, 409)
	e.settleJobs()
	out := e.call("PATCH", "/materials/"+id, "/materials/{material_id}", map[string]any{"version": 1, "text": "数组必须有序", "title": "修订素材"}, 202)
	if out["material"].(map[string]any)["version"] != float64(2) {
		t.Fatal("material version did not increment")
	}
	e.settleJobs()
	detail := e.call("GET", "/materials/"+id, "/materials/{material_id}", nil, 200)
	if detail["text"] != "数组必须有序" || detail["title"] != "修订素材" {
		t.Fatal("material edits lost")
	}
	e.call("PATCH", "/materials/"+id, "/materials/{material_id}", map[string]any{"version": 2, "source_url": "https://bilibili.com/video/BV123"}, 422)
	e.call("DELETE", "/materials/"+id, "/materials/{material_id}", nil, 204)
	se := e.call("POST", "/lessons/"+e.lesson+"/summary-evaluations", "/lessons/{lesson_id}/summary-evaluations", map[string]any{"summary": "二分查找需要有序输入"}, 202)["evaluation"].(map[string]any)
	sid := se["id"].(string)
	e.call("DELETE", "/summary-evaluations/"+sid, "/summary-evaluations/{evaluation_id}", nil, 409)
	e.call("DELETE", "/lessons/"+e.lesson, "/lessons/{lesson_id}", nil, 409)
	e.call("DELETE", "/courses/"+e.course, "/courses/{course_id}", nil, 409)
	e.settleJobs()
	edited := e.call("PATCH", "/summary-evaluations/"+sid, "/summary-evaluations/{evaluation_id}", map[string]any{"summary": "区间为空表示不存在"}, 202)["evaluation"].(map[string]any)
	newID := edited["id"].(string)
	if newID == sid {
		t.Fatal("summary revision overwrote history")
	}
	e.settleJobs()
	old := e.call("GET", "/summary-evaluations/"+sid, "/summary-evaluations/{evaluation_id}", nil, 200)
	if old["summary_text"] != "二分查找需要有序输入" {
		t.Fatal("summary history lost")
	}
	page := e.call("GET", "/lessons/"+e.lesson+"/summary-evaluations?limit=1", "/lessons/{lesson_id}/summary-evaluations", nil, 200)
	cursor := page["page_info"].(map[string]any)["next_cursor"].(string)
	page2 := e.call("GET", "/lessons/"+e.lesson+"/summary-evaluations?limit=1&cursor="+cursor, "/lessons/{lesson_id}/summary-evaluations", nil, 200)
	if len(page2["items"].([]any)) != 1 {
		t.Fatal("summary cursor lost sub-second row")
	}
	e.call("DELETE", "/summary-evaluations/"+sid, "/summary-evaluations/{evaluation_id}", nil, 204)
	e.call("DELETE", "/summary-evaluations/"+newID, "/summary-evaluations/{evaluation_id}", nil, 204)
	w := e.call("POST", "/courses/"+e.course+"/weaknesses", "/courses/{course_id}/weaknesses", map[string]any{"lesson_id": e.lesson, "concept_label": "边界条件", "description": "需要复习", "severity": 3}, 201)
	wid := w["id"].(string)
	e.call("PATCH", "/weaknesses/"+wid, "/weaknesses/{weakness_id}", map[string]any{"version": 1, "severity": 2, "mastered": true}, 200)
	e.call("PATCH", "/weaknesses/"+wid, "/weaknesses/{weakness_id}", map[string]any{"version": 1, "severity": 1}, 409)
	e.call("DELETE", "/weaknesses/"+wid, "/weaknesses/{weakness_id}", nil, 204)
	e.call("GET", "/weaknesses/"+wid, "/weaknesses/{weakness_id}", nil, 404)
}
func TestContentMutationsRejectInvalidAndCrossCourseData(t *testing.T) {
	e := newContentEnv(t)
	other := e.call("POST", "/courses", "/courses", map[string]any{"title": "另一课程"}, 201)["id"].(string)
	e.call("POST", "/courses/"+other+"/weaknesses", "/courses/{course_id}/weaknesses", map[string]any{"lesson_id": e.lesson, "concept_label": "错误归属", "severity": 3}, 404)
	e.call("POST", "/lessons/"+e.lesson+"/quizzes", "/lessons/{lesson_id}/quizzes", map[string]any{"title": "空卷", "questions": []any{}}, 422)
	e.call("POST", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", map[string]any{"content_md": "   "}, 422)
	rec := e.request("POST", "/lessons/"+e.lesson+"/quizzes", map[string]any{"title": "缺幂等键", "questions": []any{question("测试")}}, "")
	if rec.Code != 400 {
		t.Fatalf("missing idempotency accepted: %d", rec.Code)
	}
	e.exec(`UPDATE system_state SET maintenance_mode=true,phase='quiesced'`)
	e.call("POST", "/lessons/"+e.lesson+"/documents/raw_text", "/lessons/{lesson_id}/documents/{kind}", map[string]any{"content_md": "维护中"}, 503)
}

func TestContentDeleteGradedAttemptCleansEvidenceAndProgress(t *testing.T) {
	e := newContentEnv(t)
	z := e.call("POST", "/lessons/"+e.lesson+"/quizzes", "/lessons/{lesson_id}/quizzes", map[string]any{"title": "客观测验", "questions": []any{question("输入应满足什么前提？")}}, 201)
	quizID := z["id"].(string)
	a := e.call("POST", "/quizzes/"+quizID+"/attempts", "/quizzes/{quiz_id}/attempts", map[string]any{}, 201)
	aid := a["id"].(string)
	qid := a["questions"].([]any)[0].(map[string]any)["id"].(string)
	saved := e.call("PATCH", "/attempts/"+aid+"/answers/"+qid, "/attempts/{attempt_id}/answers/{question_id}", map[string]any{"answer_version": 0, "answer": map[string]any{"value": "B"}, "client_updated_at": time.Now().UTC().Format(time.RFC3339)}, 200)
	e.call("POST", "/attempts/"+aid+"/submit", "/attempts/{attempt_id}/submit", map[string]any{"version": saved["attempt_version"]}, 200)
	var evidence int
	if err := e.pool.QueryRow(context.Background(), `SELECT jsonb_array_length(evidence) FROM weaknesses WHERE course_id=$1`, e.course).Scan(&evidence); err != nil || evidence != 1 {
		t.Fatalf("missing evidence %d %v", evidence, err)
	}
	e.exec(`INSERT INTO jobs(id,kind,aggregate_type,aggregate_id,payload) VALUES($1,'wrong_answer_analyze','attempt',$2,'{}')`, uuid.New(), aid)
	e.call("DELETE", "/attempts/"+aid, "/attempts/{attempt_id}", nil, 409)
	e.settleJobs()
	e.call("DELETE", "/attempts/"+aid, "/attempts/{attempt_id}", nil, 204)
	if err := e.pool.QueryRow(context.Background(), `SELECT jsonb_array_length(evidence) FROM weaknesses WHERE course_id=$1`, e.course).Scan(&evidence); err != nil || evidence != 0 {
		t.Fatalf("orphan evidence %d %v", evidence, err)
	}
	progress := e.call("GET", "/lessons/"+e.lesson+"/progress", "/lessons/{lesson_id}/progress", nil, 200)
	if progress["completed_attempts"] != float64(0) {
		t.Fatal("deleted grade still counted")
	}
	e.call("DELETE", "/quizzes/"+quizID, "/quizzes/{quiz_id}", nil, 204)
}
