// Package attempt_test 是作答链路的 HTTP 集成测试：真实 PostgreSQL +
// 与生产一致的 Gin 引擎，响应体以 api/openapi.yaml 校验。
// 覆盖：创建/自动保存版本语义/提交 200 与 202 拆分/幂等回放/反馈可见性/
// grading_failed 的 Job Retry 与 grading-jobs 重建/掌握度重复消费。
package attempt_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/apitest"
	"github.com/TSY244/ai-study-tutor/internal/attempt"
	"github.com/TSY244/ai-study-tutor/internal/course"
	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/internal/job"
	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/mastery"
	"github.com/TSY244/ai-study-tutor/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
	"github.com/TSY244/ai-study-tutor/internal/quiz"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

const (
	attemptsPath    = "/api/v1/quizzes/{quiz_id}/attempts"
	attemptPath     = "/api/v1/attempts/{attempt_id}"
	savePath        = "/api/v1/attempts/{attempt_id}/answers/{question_id}"
	submitPath      = "/api/v1/attempts/{attempt_id}/submit"
	gradingJobsPath = "/api/v1/attempts/{attempt_id}/grading-jobs"
	retryPath       = "/api/v1/jobs/{job_id}/retry"
)

// newEngine 装配 attempt/quiz/job/mastery 模块（与生产路由同构）。
func newEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	rg := e.Group("/api/v1")
	jsonLimit := httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request")
	idem := &httpx.IdemExecutor{Svc: idempotency.NewService(pool)}
	attempt.NewHandler(attempt.NewService(pool), idem).RegisterRoutes(rg, jsonLimit)
	quiz.NewQueryHandler(quiz.NewQueryService(pool)).RegisterRoutes(rg)
	job.NewHandler(job.NewService(pool), idem).RegisterRoutes(rg, jsonLimit)
	mastery.NewHandler(
		mastery.NewWeaknessService(pool, attempt.Repository{}),
		mastery.NewProgressService(pool, attempt.Repository{}, course.Repository{}, lesson.Repository{}),
		idem).RegisterRoutes(rg, jsonLimit)
	return e
}

// seedLesson 创建课程+课时，返回 (courseID, lessonID)。
func seedLesson(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	courseID, lessonID := uuid.New(), uuid.New()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO courses (id, slug, title) VALUES ($1,$2,'课程')`,
		courseID, "c-"+uuid.NewString()[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,'课时')`,
		lessonID, courseID); err != nil {
		t.Fatal(err)
	}
	return courseID, lessonID
}

// draft 构造一道题的草稿。
func draft(qtype, stem string, options map[string]string, correct json.RawMessage,
	concepts []string, score float64) quiz.QuestionDraft {
	return quiz.QuestionDraft{
		Type:           qtype,
		Stem:           stem,
		Options:        options,
		CorrectAnswer:  correct,
		TestedConcepts: concepts,
		Difficulty:     "medium",
		CognitiveLevel: "recall",
		Score:          score,
	}
}

// assembleQuiz 用生产 AssemblyService 发布 ready 试卷。
func assembleQuiz(t *testing.T, pool *pgxpool.Pool, lessonID uuid.UUID, drafts []quiz.QuestionDraft) *quiz.Quiz {
	t.Helper()
	z, _, err := quiz.NewAssemblyService(pool).Assemble(context.Background(), quiz.AssembleInput{
		LessonID: lessonID,
		Source:   "regular",
		Title:    "极限练习",
		Drafts:   drafts,
	})
	if err != nil {
		t.Fatalf("组装试卷失败: %v", err)
	}
	return z
}

// objectiveQuiz 发布一份全客观题试卷（单选/多选/判断，分值 2/3/1）。
func objectiveQuiz(t *testing.T, pool *pgxpool.Pool, lessonID uuid.UUID) *quiz.Quiz {
	t.Helper()
	return assembleQuiz(t, pool, lessonID, []quiz.QuestionDraft{
		draft("single_choice", "极限的直观含义是？", map[string]string{"A": "趋势", "B": "终点"},
			json.RawMessage(`"A"`), []string{"极限"}, 2),
		draft("multiple_choice", "哪些是无穷小量的性质？", map[string]string{"A": "甲", "B": "乙", "C": "丙"},
			json.RawMessage(`["A","C"]`), []string{"无穷小", "极限"}, 3),
		draft("true_false", "常数列一定收敛。", nil,
			json.RawMessage(`"对"`), []string{"收敛"}, 1),
	})
}

func jsonBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return m
}

func do(t *testing.T, engine *gin.Engine, method, path, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{"Content-Type": "application/json"}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	return apitest.Do(t, engine, method, path, headers, body)
}

// createAttempt 开始作答应答 201，返回 attempt id 与 version。
func createAttempt(t *testing.T, engine *gin.Engine, contract *apitest.Contract, quizID uuid.UUID) (string, float64) {
	t.Helper()
	key := uuid.NewString()
	rec := do(t, engine, http.MethodPost, "/api/v1/quizzes/"+quizID.String()+"/attempts", key, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建作答 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, attemptsPath, http.StatusCreated, rec.Body.Bytes())
	if loc := rec.Header().Get("Location"); loc == "" {
		t.Fatal("创建作答应返回 Location 头")
	}
	m := jsonBody(t, rec.Body.Bytes())
	if m["status"] != "in_progress" || m["questions"] == nil {
		t.Fatalf("初始作答不正确: %v", m)
	}
	// 学生试卷视图不得泄露答案与解析。
	if strings.Contains(rec.Body.String(), "correct_answer") || strings.Contains(rec.Body.String(), "explanation") {
		t.Fatal("创建作答的学生视图泄露了 correct_answer/explanation")
	}
	return m["id"].(string), m["version"].(float64)
}

// quizQuestions 返回试卷题目 id 列表（按 position）。
func quizQuestions(t *testing.T, pool *pgxpool.Pool, quizID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT question_id FROM quiz_items WHERE quiz_id = $1 ORDER BY position`, quizID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func save(t *testing.T, engine *gin.Engine, attemptID string, questionID uuid.UUID, version int64, value string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"answer":{"value":` + strconv.Quote(value) + `},"answer_version":` + strconv.FormatInt(version, 10) +
		`,"client_updated_at":"2026-07-17T08:35:10.123Z"}`
	return do(t, engine, http.MethodPatch,
		"/api/v1/attempts/"+attemptID+"/answers/"+questionID.String(), "", body)
}

func attemptVersion(t *testing.T, engine *gin.Engine, attemptID string) float64 {
	t.Helper()
	rec := do(t, engine, http.MethodGet, "/api/v1/attempts/"+attemptID, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("查询作答 status = %d", rec.Code)
	}
	return jsonBody(t, rec.Body.Bytes())["version"].(float64)
}

// ---- 创建与自动保存 ----

func TestCreateAttemptAndAutosave(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	_, lessonID := seedLesson(t, pool)
	z := objectiveQuiz(t, pool, lessonID)
	questions := quizQuestions(t, pool, z.ID)

	attemptID, version := createAttempt(t, engine, contract, z.ID)
	if version != 1 {
		t.Fatalf("初始 version = %v，期望 1", version)
	}

	// 幂等回放：同 Key 返回同一响应（含 Location 头回放）。
	key := uuid.NewString()
	rec1 := do(t, engine, http.MethodPost, "/api/v1/quizzes/"+z.ID.String()+"/attempts", key, "")
	rec2 := do(t, engine, http.MethodPost, "/api/v1/quizzes/"+z.ID.String()+"/attempts", key, "")
	if rec1.Code != http.StatusCreated || rec2.Code != http.StatusCreated {
		t.Fatalf("回放状态码 = %d/%d", rec1.Code, rec2.Code)
	}
	// JSONB 存储的响应回放为语义等价 JSON（键序/空白由 jsonb 规范化）。
	if jsonBody(t, rec1.Body.Bytes())["id"] != jsonBody(t, rec2.Body.Bytes())["id"] {
		t.Fatal("幂等回放应返回同一作答")
	}
	if rec2.Header().Get("Location") == "" {
		t.Fatal("幂等回放应携带 Location 头")
	}

	// 未发布试卷不能作答：draft 试卷 422。
	draftQuiz := assembleQuiz(t, pool, lessonID, []quiz.QuestionDraft{
		draft("single_choice", "q", map[string]string{"A": "a"}, json.RawMessage(`"A"`), nil, 1),
	})
	if _, err := pool.Exec(context.Background(),
		`UPDATE quizzes SET status = 'draft' WHERE id = $1`, draftQuiz.ID); err != nil {
		t.Fatal(err)
	}
	rec := do(t, engine, http.MethodPost, "/api/v1/quizzes/"+draftQuiz.ID.String()+"/attempts", uuid.NewString(), "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("draft 试卷作答 status = %d", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusUnprocessableEntity, "validation_error")

	// 首次自动保存 answer_version=0 → 1，Attempt 全局版本递增。
	rec = save(t, engine, attemptID, questions[0], 0, "A")
	if rec.Code != http.StatusOK {
		t.Fatalf("首次保存 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPatch, savePath, http.StatusOK, rec.Body.Bytes())
	m := jsonBody(t, rec.Body.Bytes())
	if m["answer_version"].(float64) != 1 || m["attempt_version"].(float64) != 2 {
		t.Fatalf("首次保存版本错误: %v", m)
	}

	// 同题过期版本 → 409 version_conflict + 服务端当前状态扩展。
	rec = save(t, engine, attemptID, questions[0], 0, "B")
	if rec.Code != http.StatusConflict {
		t.Fatalf("过期版本 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "version_conflict")
	m = jsonBody(t, rec.Body.Bytes())
	if m["current_answer_version"].(float64) != 1 || m["current_attempt_version"].(float64) != 2 {
		t.Fatalf("冲突扩展错误: %v", m)
	}
	if cur, ok := m["current_answer"].(map[string]any); !ok || cur["value"] != "A" {
		t.Fatalf("current_answer 扩展错误: %v", m["current_answer"])
	}

	// 正确版本继续保存 → 2。
	rec = save(t, engine, attemptID, questions[0], 1, "B")
	if rec.Code != http.StatusOK || jsonBody(t, rec.Body.Bytes())["answer_version"].(float64) != 2 {
		t.Fatalf("二次保存失败: %d %s", rec.Code, rec.Body)
	}
	// 不同题互不阻塞（各自版本独立）。
	rec = save(t, engine, attemptID, questions[1], 0, "c,a")
	if rec.Code != http.StatusOK || jsonBody(t, rec.Body.Bytes())["answer_version"].(float64) != 1 {
		t.Fatalf("不同题保存失败: %d %s", rec.Code, rec.Body)
	}

	// 不属于本试卷的题目 → 404。
	rec = save(t, engine, attemptID, uuid.New(), 0, "A")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("越权题目 status = %d", rec.Code)
	}
}

// ---- 提交：全客观题 200 ----

func TestSubmitAllObjectiveCompleted(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID, lessonID := seedLesson(t, pool)
	z := objectiveQuiz(t, pool, lessonID)
	questions := quizQuestions(t, pool, z.ID)
	attemptID, _ := createAttempt(t, engine, contract, z.ID)

	// q1 答对、q2 集合乱序答对、q3 未作答。
	save(t, engine, attemptID, questions[0], 0, "a")
	save(t, engine, attemptID, questions[1], 0, "C,A")

	// 错误的全局版本 → 409 version_conflict。
	key := uuid.NewString()
	rec := do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit", key, `{"version":99}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("错误版本提交 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "version_conflict")

	// 正确版本提交 → 200 AttemptSubmitCompleted（不含 job）。
	version := attemptVersion(t, engine, attemptID)
	body := `{"version":` + strconv.FormatInt(int64(version), 10) + `}`
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit", key, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("提交 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, submitPath, http.StatusOK, rec.Body.Bytes())
	m := jsonBody(t, rec.Body.Bytes())
	if _, hasJob := m["job"]; hasJob {
		t.Fatal("AttemptSubmitCompleted 不得包含 job 字段")
	}
	att := m["attempt"].(map[string]any)
	if att["status"] != "graded" {
		t.Fatalf("全客观题应同步 graded: %v", att["status"])
	}
	// q1 2 分 + q2 3 分 + q3 未答 0 分 = 5 / 6。
	if att["auto_score"].(float64) != 5 || att["final_score"].(float64) != 5 ||
		att["max_score"].(float64) != 6 {
		t.Fatalf("分数错误: auto=%v final=%v max=%v", att["auto_score"], att["final_score"], att["max_score"])
	}
	// graded 后反馈可见：correct_answer/explanation 经 feedback 返回。
	answers := att["answers"].([]any)
	if len(answers) != 3 {
		t.Fatalf("answers 数量 = %d", len(answers))
	}
	var q1fb, q3fb map[string]any
	for _, a := range answers {
		am := a.(map[string]any)
		fb, _ := am["feedback"].(map[string]any)
		if am["question_id"] == questions[0].String() {
			q1fb = fb
			if am["is_correct"] != true || am["awarded_score"].(float64) != 2 {
				t.Fatalf("q1 判分错误: %v", am)
			}
		}
		if am["question_id"] == questions[2].String() {
			q3fb = fb
			if am["is_correct"] != nil {
				t.Fatalf("未作答题 is_correct 应为 null: %v", am)
			}
		}
	}
	if q1fb == nil || q1fb["correct_answer"] == nil {
		t.Fatal("graded 后应返回 correct_answer 反馈")
	}
	if q3fb == nil || q3fb["feedback"] != "未作答" {
		t.Fatalf("未作答反馈错误: %v", q3fb)
	}

	// 重复提交：同 Key 回放一致；不同 Key → 409 attempt_already_submitted；不产生 Job。
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit", key, body)
	if rec.Code != http.StatusOK || rec.Body.String() == "" {
		t.Fatalf("同 Key 回放失败: %d", rec.Code)
	}
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit", uuid.NewString(), body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("重复提交 status = %d", rec.Code)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "attempt_already_submitted")
	var jobCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs`).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if jobCount != 0 {
		t.Fatalf("全客观题不得产生 Job，实际 %d", jobCount)
	}

	// 提交后自动保存被拒绝。
	rec = save(t, engine, attemptID, questions[0], 2, "A")
	if rec.Code != http.StatusConflict {
		t.Fatalf("提交后保存 status = %d", rec.Code)
	}

	// 掌握度快照已追加（lesson+course），进度可见。
	var snapCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM mastery_snapshots WHERE source_attempt_id = $1`, attemptID).Scan(&snapCount); err != nil {
		t.Fatal(err)
	}
	if snapCount != 2 {
		t.Fatalf("快照数 = %d，期望 2（lesson+course）", snapCount)
	}
	rec = do(t, engine, http.MethodGet, "/api/v1/courses/"+courseID.String()+"/progress", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("课程进度 status = %d", rec.Code)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, "/api/v1/courses/{course_id}/progress", http.StatusOK, rec.Body.Bytes())
	prog := jsonBody(t, rec.Body.Bytes())
	if prog["mastery"] == nil || prog["formula_version"] != "mastery/v1" ||
		prog["completed_attempts"].(float64) != 1 {
		t.Fatalf("进度聚合错误: %v", prog)
	}
}

// ---- 提交：含主观题 202 与反馈可见性 ----

func TestSubmitWithSubjectiveAccepted(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	_, lessonID := seedLesson(t, pool)
	z := assembleQuiz(t, pool, lessonID, []quiz.QuestionDraft{
		draft("single_choice", "极限的直观含义是？", map[string]string{"A": "趋势", "B": "终点"},
			json.RawMessage(`"A"`), []string{"极限"}, 2),
		draft("short_answer", "用自己的话解释 ε-δ 定义。", nil,
			json.RawMessage(`"见解析"`), []string{"极限"}, 8),
	})
	questions := quizQuestions(t, pool, z.ID)
	attemptID, _ := createAttempt(t, engine, contract, z.ID)
	save(t, engine, attemptID, questions[0], 0, "A")
	save(t, engine, attemptID, questions[1], 0, "ε-δ 是严格化表述")

	version := attemptVersion(t, engine, attemptID)
	key := uuid.NewString()
	body := `{"version":` + strconv.FormatInt(int64(version), 10) + `}`
	rec := do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit", key, body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("提交 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, submitPath, http.StatusAccepted, rec.Body.Bytes())
	m := jsonBody(t, rec.Body.Bytes())
	j, ok := m["job"].(map[string]any)
	if !ok || j["kind"] != "attempt_grade" || j["status"] != "queued" {
		t.Fatalf("202 必含 attempt_grade Job: %v", m)
	}
	att := m["attempt"].(map[string]any)
	if att["status"] != "grading" || att["final_score"] != nil {
		t.Fatalf("主观题未批阅前不得有 final_score: %v", att)
	}
	if att["auto_score"].(float64) != 2 {
		t.Fatalf("客观题暂定分错误: %v", att["auto_score"])
	}

	// 同一 Attempt 重复提交只产生一个批阅 Job：同 Key 回放 + 新 Key 409。
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit", key, body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("同 Key 回放 status = %d", rec.Code)
	}
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit", uuid.NewString(), body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("重复提交 status = %d", rec.Code)
	}
	var jobCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM jobs WHERE kind = 'attempt_grade'`).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if jobCount != 1 {
		t.Fatalf("批阅 Job 数 = %d，期望 1", jobCount)
	}

	// grading 中：客观题结果可见，但不得泄露 correct_answer/explanation。
	rec = do(t, engine, http.MethodGet, "/api/v1/attempts/"+attemptID, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("查询作答 status = %d", rec.Code)
	}
	contract.ValidateJSONResponse(t, http.MethodGet, attemptPath, http.StatusOK, rec.Body.Bytes())
	att = jsonBody(t, rec.Body.Bytes())
	for _, a := range att["answers"].([]any) {
		am := a.(map[string]any)
		if _, hasFeedback := am["feedback"]; hasFeedback {
			t.Fatalf("grading 中不得返回反馈: %v", am)
		}
		if am["question_id"] == questions[0].String() && am["is_correct"] != true {
			t.Fatalf("grading 中客观题结果应可见: %v", am)
		}
	}
	if strings.Contains(rec.Body.String(), "correct_answer") {
		t.Fatal("grading 中泄露了 correct_answer")
	}
}

// ---- grading_failed：Job Retry 与 grading-jobs 重建 ----

// failGrading 把作答与其批阅 Job 置为失败态（模拟 Worker 批阅失败）。
func failGrading(t *testing.T, pool *pgxpool.Pool, attemptID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		UPDATE jobs SET status = 'failed', error_code = 'claude_error', error_detail = '上游错误',
		    completed_at = now(), lease_owner = NULL, lease_expires_at = NULL
		WHERE aggregate_id = $1 AND kind = 'attempt_grade'`, attemptID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		UPDATE attempts SET status = 'grading_failed',
		    grading_error_code = 'claude_error', grading_error_detail = '上游错误'
		WHERE id = $1`, attemptID); err != nil {
		t.Fatal(err)
	}
}

func TestGradingFailedRetryAndRebuild(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	_, lessonID := seedLesson(t, pool)
	z := assembleQuiz(t, pool, lessonID, []quiz.QuestionDraft{
		draft("short_answer", "解释 ε-δ 定义。", nil, json.RawMessage(`"见解析"`), []string{"极限"}, 10),
	})
	attemptID, _ := createAttempt(t, engine, contract, z.ID)
	qs := quizQuestions(t, pool, z.ID)
	save(t, engine, attemptID, qs[0], 0, "严格化表述")
	version := attemptVersion(t, engine, attemptID)
	rec := do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit",
		uuid.NewString(), `{"version":`+strconv.FormatInt(int64(version), 10)+`}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("提交 status = %d", rec.Code)
	}
	oldJobID := jsonBody(t, rec.Body.Bytes())["job"].(map[string]any)["id"].(string)
	failGrading(t, pool, attemptID)

	// grading_failed 可见错误信息；反馈仍不泄露。
	rec = do(t, engine, http.MethodGet, "/api/v1/attempts/"+attemptID, "", "")
	att := jsonBody(t, rec.Body.Bytes())
	if att["status"] != "grading_failed" || att["grading_error_code"] != "claude_error" {
		t.Fatalf("grading_failed 状态错误: %v", att)
	}

	// Job Retry：创建新批阅 Job（retry_of 指向原任务），作答回到 grading。
	key := uuid.NewString()
	rec = do(t, engine, http.MethodPost, "/api/v1/jobs/"+oldJobID+"/retry", key, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("Job Retry status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, retryPath, http.StatusAccepted, rec.Body.Bytes())
	newJob := jsonBody(t, rec.Body.Bytes())["job"].(map[string]any)
	if newJob["retry_of_job_id"] != oldJobID || newJob["kind"] != "attempt_grade" {
		t.Fatalf("重试 Job 错误: %v", newJob)
	}
	rec = do(t, engine, http.MethodGet, "/api/v1/attempts/"+attemptID, "", "")
	att = jsonBody(t, rec.Body.Bytes())
	if att["status"] != "grading" || att["grading_job_id"] != newJob["id"] ||
		att["grading_error_code"] != nil {
		t.Fatalf("重试后作答状态错误: %v", att)
	}

	// 同 Key 回放一致；活动中的 Job 不能重试（409 resource_busy）。
	rec = do(t, engine, http.MethodPost, "/api/v1/jobs/"+oldJobID+"/retry", key, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("Retry 回放 status = %d", rec.Code)
	}
	rec = do(t, engine, http.MethodPost, "/api/v1/jobs/"+newJob["id"].(string)+"/retry", uuid.NewString(), "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("活动任务重试 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")

	// 存在活动批阅 Job 时 grading-jobs → 409 resource_busy + active_job_id。
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/grading-jobs", uuid.NewString(), "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("活动任务下重建 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusConflict, "resource_busy")
	if jsonBody(t, rec.Body.Bytes())["active_job_id"] == nil {
		t.Fatal("resource_busy 应携带 active_job_id 扩展")
	}

	// 新批阅 Job 再次失败 → grading-jobs 重建成功（retry_of 指向第二个任务）。
	if _, err := pool.Exec(context.Background(), `
		UPDATE jobs SET status = 'failed', error_code = 'claude_error', error_detail = 'x',
		    completed_at = now(), lease_owner = NULL, lease_expires_at = NULL WHERE id = $1`,
		newJob["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		UPDATE attempts SET status = 'grading_failed', grading_error_code = 'claude_error'
		WHERE id = $1`, attemptID); err != nil {
		t.Fatal(err)
	}
	key = uuid.NewString()
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/grading-jobs", key, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("重建 status = %d, body = %s", rec.Code, rec.Body)
	}
	contract.ValidateJSONResponse(t, http.MethodPost, gradingJobsPath, http.StatusAccepted, rec.Body.Bytes())
	rebuilt := jsonBody(t, rec.Body.Bytes())["job"].(map[string]any)
	if rebuilt["retry_of_job_id"] != newJob["id"] {
		t.Fatalf("重建 Job retry_of 错误: %v", rebuilt)
	}
	rec = do(t, engine, http.MethodGet, "/api/v1/attempts/"+attemptID, "", "")
	if jsonBody(t, rec.Body.Bytes())["status"] != "grading" {
		t.Fatal("重建后作答应回到 grading")
	}
	// 同 Key 回放返回同一 Job。
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/grading-jobs", key, "")
	if rec.Code != http.StatusAccepted ||
		jsonBody(t, rec.Body.Bytes())["job"].(map[string]any)["id"] != rebuilt["id"] {
		t.Fatalf("重建回放应返回同一 Job: %d %s", rec.Code, rec.Body)
	}

	// grading-jobs 只接受 grading_failed：in_progress → 422；graded → 409。
	otherID, _ := createAttempt(t, engine, contract, z.ID)
	rec = do(t, engine, http.MethodPost, "/api/v1/attempts/"+otherID+"/grading-jobs", uuid.NewString(), "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("in_progress 重建 status = %d", rec.Code)
	}
}

// ---- 掌握度重复消费 ----

func TestMasteryReplayNoDuplicate(t *testing.T) {
	pool := databasetest.New(t)
	engine := newEngine(t, pool)
	contract := apitest.Load(t)
	courseID, lessonID := seedLesson(t, pool)
	z := objectiveQuiz(t, pool, lessonID)
	questions := quizQuestions(t, pool, z.ID)
	attemptID, _ := createAttempt(t, engine, contract, z.ID)

	// q1 答错（正确为 A，答 B）→ 产生“极限”薄弱点；q2 答对触发复习；q3 未答。
	save(t, engine, attemptID, questions[0], 0, "B")
	save(t, engine, attemptID, questions[1], 0, "A,C")
	version := attemptVersion(t, engine, attemptID)
	rec := do(t, engine, http.MethodPost, "/api/v1/attempts/"+attemptID+"/submit",
		uuid.NewString(), `{"version":`+strconv.FormatInt(int64(version), 10)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("提交 status = %d, body = %s", rec.Code, rec.Body)
	}

	// 薄弱点已创建。q1 答错（创建 severity=3、review_count=1），q2 答对同概念
	// 触发复习（legacy reduce 语义：severity-1、review_count+1），
	// 最终 severity=2、review_count=2，证据一条。
	var wID string
	var severity, reviewCount int
	var evidence []byte
	if err := pool.QueryRow(context.Background(), `
		SELECT id, severity, review_count, evidence FROM weaknesses
		WHERE course_id = $1 AND lesson_id = $2 AND concept_key = '极限'`,
		courseID, lessonID).Scan(&wID, &severity, &reviewCount, &evidence); err != nil {
		t.Fatalf("答错应产生薄弱点: %v", err)
	}
	if severity != 2 || reviewCount != 2 {
		t.Fatalf("薄弱点合并/复习状态错误: severity=%d review_count=%d", severity, reviewCount)
	}
	var items []map[string]any
	if err := json.Unmarshal(evidence, &items); err != nil || len(items) != 1 {
		t.Fatalf("证据数组错误: %s", evidence)
	}

	// 重复消费同一批阅结果：直接再调 ApplyService（模拟 Worker 重放）。
	apply := mastery.NewApplyService(attempt.Repository{})
	attemptUUID, _ := uuid.Parse(attemptID)
	truth := false
	in := mastery.ApplyInput{
		CourseID: courseID, LessonID: lessonID, AttemptID: attemptUUID, QuizID: z.ID,
		Items: []mastery.OutcomeItem{
			{QuestionID: questions[0], Concepts: []string{"极限"}, Correct: &truth, Note: "客观题作答判分"},
		},
	}
	applied, err := apply.ApplyAttemptOutcome(context.Background(), pool, in)
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("重复消费应返回 applied=false")
	}
	// 无重复写入：快照数、review_count、证据条数不变。
	var snapCount, rc2, evidenceCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM mastery_snapshots WHERE source_attempt_id = $1`, attemptID).Scan(&snapCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(),
		`SELECT review_count, jsonb_array_length(evidence) FROM weaknesses WHERE id = $1`, wID).Scan(&rc2, &evidenceCount); err != nil {
		t.Fatal(err)
	}
	if snapCount != 2 || rc2 != 2 || evidenceCount != 1 {
		t.Fatalf("重复消费产生了重复写入: snapshots=%d review_count=%d evidence=%d",
			snapCount, rc2, evidenceCount)
	}
}
