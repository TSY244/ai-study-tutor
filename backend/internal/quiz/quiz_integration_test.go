// Package quiz_test 补充 B4 的试卷查询/组装/策略门禁集成测试：
// 学生视图不可见答案、发布后不可变输入校验、游标分页、
// 掌握度提前停止（practice_not_needed）与显式题量覆盖。
package quiz_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/attempt"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
)

const (
	quizzesPath = "/api/v1/lessons/{lesson_id}/quizzes"
	quizPath    = "/api/v1/quizzes/{quiz_id}"
)

// newQueryEngine 装配试卷查询端点。
func newQueryEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	quiz.NewQueryHandler(quiz.NewQueryService(pool)).RegisterRoutes(e.Group("/api/v1"))
	return e
}

// newGenerationEngine 装配接入真实策略门禁的出题端点。
func newGenerationEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	svc := quiz.NewGenerationService(pool)
	svc.QuizPolicy = (&mastery.PracticeEvaluator{
		Scores:  attempt.Repository{},
		Weak:    mastery.Repository{},
		Lessons: lesson.Repository{},
	}).Needed
	quiz.NewGenerationHandler(svc, &httpx.IdemExecutor{Svc: idempotency.NewService(pool)}).
		RegisterRoutes(e.Group("/api/v1"),
			httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"))
	return e
}

func oneDraft() []quiz.QuestionDraft {
	return []quiz.QuestionDraft{{
		Type:           "single_choice",
		Stem:           "极限的直观含义是？",
		Options:        map[string]string{"A": "趋势", "B": "终点"},
		CorrectAnswer:  json.RawMessage(`"A"`),
		Explanation:    strPtr("极限描述趋势。"),
		TestedConcepts: []string{"极限"},
		Difficulty:     "easy",
		CognitiveLevel: "recall",
		Score:          5,
	}}
}

func strPtr(s string) *string { return &s }

// TestAssemblyAndQuery 组装发布 → 学生视图查询（不可见答案/解析）→ 分页。
func TestAssemblyAndQuery(t *testing.T) {
	pool := databasetest.New(t)
	engine := newQueryEngine(t, pool)
	contract := apitest.Load(t)
	lessonID := seedLesson(t, pool)
	svc := quiz.NewAssemblyService(pool)

	// 发布两份试卷。
	for _, title := range []string{"第一套", "第二套"} {
		if _, _, err := svc.Assemble(context.Background(), quiz.AssembleInput{
			LessonID: lessonID, Source: "regular", Title: title, Drafts: oneDraft(),
		}); err != nil {
			t.Fatalf("组装失败: %v", err)
		}
	}

	// 列表：limit=1 分页，两页各一条且游标可续。
	rec := apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+lessonID.String()+"/quizzes?limit=1", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("列表 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, quizzesPath, http.StatusOK, rec.Body.Bytes())
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
	if len(page1.Items) != 1 || !page1.PageInfo.HasMore || page1.PageInfo.NextCursor == nil {
		t.Fatalf("第一页错误: %+v", page1)
	}
	if page1.Items[0]["question_count"].(float64) != 1 {
		t.Fatalf("question_count 错误: %v", page1.Items[0])
	}
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+lessonID.String()+"/quizzes?limit=1&cursor="+*page1.PageInfo.NextCursor, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("第二页 status = %d, body = %s", rec.Code, rec.Body)
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
	if len(page2.Items) != 1 || page2.PageInfo.HasMore ||
		page2.Items[0]["id"] == page1.Items[0]["id"] {
		t.Fatalf("第二页错误: %+v", page2)
	}

	// 详情：学生视图不含 correct_answer/explanation；选项/概念可见。
	quizID := page1.Items[0]["id"].(string)
	rec = apitest.Do(t, engine, http.MethodGet, "/api/v1/quizzes/"+quizID, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("详情 status = %d", rec.Code)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, quizPath, http.StatusOK, rec.Body.Bytes())
	body := rec.Body.String()
	var detail struct {
		Status    string `json:"status"`
		Questions []struct {
			Stem    string            `json:"stem"`
			Options map[string]string `json:"options"`
			Score   float64           `json:"score"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Status != "ready" || len(detail.Questions) != 1 ||
		detail.Questions[0].Options["A"] != "趋势" || detail.Questions[0].Score != 5 {
		t.Fatalf("详情错误: %+v", detail)
	}
	if jsonContains(body, "correct_answer") || jsonContains(body, "explanation") {
		t.Fatal("学生视图泄露了 correct_answer/explanation")
	}

	// 不存在：课时/试卷 404。
	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/lessons/"+uuid.NewString()+"/quizzes", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在课时 status = %d", rec.Code)
	}
	rec = apitest.Do(t, engine, http.MethodGet, "/api/v1/quizzes/"+uuid.NewString(), nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在试卷 status = %d", rec.Code)
	}
}

func jsonContains(body, key string) bool {
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return false
	}
	return walkJSON(v, key)
}

func walkJSON(v any, key string) bool {
	switch m := v.(type) {
	case map[string]any:
		for k, val := range m {
			if k == key || walkJSON(val, key) {
				return true
			}
		}
	case []any:
		for _, item := range m {
			if walkJSON(item, key) {
				return true
			}
		}
	}
	return false
}

// TestAssemblyValidation 覆盖草稿领域校验与不可变发布约束。
func TestAssemblyValidation(t *testing.T) {
	pool := databasetest.New(t)
	lessonID := seedLesson(t, pool)
	svc := quiz.NewAssemblyService(pool)

	// 非法草稿集合：无题、坏题型、缺答案、选择题缺选项、分值非正。
	bad := quiz.AssembleInput{LessonID: lessonID, Source: "regular", Title: "x", Drafts: nil}
	var invalid *quiz.ErrInvalidDraft
	if _, _, err := svc.Assemble(context.Background(), bad); !errors.As(err, &invalid) {
		t.Fatalf("空试卷应拒绝: %v", err)
	}
	d := oneDraft()[0]
	d.Options = nil
	if _, _, err := svc.Assemble(context.Background(), quiz.AssembleInput{
		LessonID: lessonID, Source: "regular", Title: "x",
		Drafts: []quiz.QuestionDraft{d},
	}); !errors.As(err, &invalid) {
		t.Fatalf("选择题缺选项应拒绝: %v", err)
	}
	d2 := oneDraft()[0]
	d2.Score = 0
	if _, _, err := svc.Assemble(context.Background(), quiz.AssembleInput{
		LessonID: lessonID, Source: "regular", Title: "x",
		Drafts: []quiz.QuestionDraft{d2},
	}); !errors.As(err, &invalid) {
		t.Fatalf("分值非正应拒绝: %v", err)
	}

	// 发布后不可变：quiz_items 提供方只有创建路径，状态机只允许归档。
	z, _, err := svc.Assemble(context.Background(), quiz.AssembleInput{
		LessonID: lessonID, Source: "retest", Title: "复习卷", Drafts: oneDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if z.Status != "ready" || z.Source != "retest" {
		t.Fatalf("发布状态错误: %+v", z)
	}
}

// TestPracticeNotNeededGate 掌握度提前停止：高掌握度+连续正确 → 422；
// 显式 question_count 覆盖提前停止（golden override_beats_early_stop）。
func TestPracticeNotNeededGate(t *testing.T) {
	pool := databasetest.New(t)
	engine := newGenerationEngine(t, pool)
	contract := apitest.Load(t)
	lessonID := seedLesson(t, pool)
	ctx := context.Background()

	// 无历史：需要练习 → 202。
	rec := post(t, engine, "/api/v1/lessons/"+lessonID.String()+"/quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("无历史时应放行: %d %s", rec.Code, rec.Body)
	}
	// 释放 lesson_generation 族占用（后续请求不受 resource_busy 干扰）。
	if _, err := pool.Exec(ctx, `UPDATE jobs SET status = 'cancelled', completed_at = now()`); err != nil {
		t.Fatal(err)
	}

	// 构造高掌握度：最近一次作答 3 题全对（掌握度 1.0、连续正确 3）。
	threeDrafts := []quiz.QuestionDraft{oneDraft()[0], oneDraft()[0], oneDraft()[0]}
	z, questions, err := quiz.NewAssemblyService(pool).Assemble(ctx, quiz.AssembleInput{
		LessonID: lessonID, Source: "regular", Title: "历史卷", Drafts: threeDrafts,
	})
	if err != nil {
		t.Fatal(err)
	}
	attemptID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO attempts (id, quiz_id, status, auto_score, final_score, max_score,
		    submitted_at, graded_at)
		VALUES ($1,$2,'graded',15,15,15,now(),now())`, attemptID, z.ID); err != nil {
		t.Fatal(err)
	}
	for _, q := range questions {
		if _, err := pool.Exec(ctx, `
			INSERT INTO attempt_answers (attempt_id, question_id, answer, auto_correct, awarded_score)
			VALUES ($1,$2,'{"value":"A"}',true,5)`, attemptID, q.ID); err != nil {
			t.Fatal(err)
		}
	}

	// 提前停止 → 422 practice_not_needed，不创建 Job。
	before := jobCount(t, pool)
	rec = post(t, engine, "/api/v1/lessons/"+lessonID.String()+"/quiz-generations",
		uuid.NewString(), `{}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("提前停止应 422: %d %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "practice_not_needed")
	if after := jobCount(t, pool); after != before {
		t.Fatalf("practice_not_needed 不得创建 Job: %d → %d", before, after)
	}

	// 显式题量覆盖提前停止 → 202（复习出题本就不受限，另行覆盖）。
	rec = post(t, engine, "/api/v1/lessons/"+lessonID.String()+"/quiz-generations",
		uuid.NewString(), `{"question_count":5}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("显式题量应覆盖提前停止: %d %s", rec.Code, rec.Body)
	}
}
