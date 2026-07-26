package aihandlers_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/attempt"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/material"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/aihandlers"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

type env struct {
	pool   *pgxpool.Pool
	store  *filestore.Store
	runner *worker.Runner
	fake   *ai.FakeProvider
}

func newEnv(t *testing.T) *env {
	t.Helper()
	fake := ai.NewFakeProvider()
	return newEnvWithProvider(t, fake, fake)
}

func newEnvWithProvider(t *testing.T, provider ai.Provider, fake *ai.FakeProvider) *env {
	t.Helper()
	pool := databasetest.New(t)
	store, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	reg := worker.NewRegistry()
	aihandlers.Register(reg, aihandlers.Deps{Pool: pool, Provider: provider})
	return &env{
		pool:  pool,
		store: store,
		fake:  fake,
		runner: &worker.Runner{
			Pool: pool, Registry: reg, Files: store, WorkerID: "ai-test-worker",
			Lease: 5 * time.Second, HeartbeatInterval: 200 * time.Millisecond,
			CancelPollInterval: 50 * time.Millisecond, PollInterval: 10 * time.Millisecond,
			Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
	}
}

type targetedFakeProvider struct {
	normal *ai.FakeProvider
	target *ai.FakeProvider

	mu     sync.Mutex
	keyIDs []string
}

func newTargetedFakeProvider() *targetedFakeProvider {
	return &targetedFakeProvider{normal: ai.NewFakeProvider(), target: ai.NewFakeProvider()}
}

func (f *targetedFakeProvider) Generate(ctx context.Context, req ai.Request) (ai.Response, error) {
	return f.normal.Generate(ctx, req)
}

func (f *targetedFakeProvider) GenerateForKey(ctx context.Context, keyID string, req ai.Request) (ai.Response, error) {
	f.mu.Lock()
	f.keyIDs = append(f.keyIDs, keyID)
	f.mu.Unlock()
	return f.target.Generate(ctx, req)
}

func (f *targetedFakeProvider) KeyIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.keyIDs...)
}

func (e *env) runOnce(t *testing.T) {
	t.Helper()
	claimed, err := e.runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("应领取到一个任务")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func seedLesson(t *testing.T, pool *pgxpool.Pool) (courseID, lessonID uuid.UUID) {
	t.Helper()
	c := &course.Course{Slug: "course-" + uuid.NewString(), Title: "课程"}
	if err := (course.Repository{}).Create(context.Background(), pool, c); err != nil {
		t.Fatal(err)
	}
	l := &lesson.Lesson{CourseID: c.ID, Ordinal: 1, Title: "课时"}
	if err := (lesson.Repository{}).Create(context.Background(), pool, l); err != nil {
		t.Fatal(err)
	}
	return c.ID, l.ID
}

func publishDoc(t *testing.T, pool *pgxpool.Pool, lessonID uuid.UUID, kind, content string) {
	t.Helper()
	d := &document.Document{LessonID: lessonID, Kind: kind, ContentMD: content}
	if err := (document.Repository{}).CreateVersion(context.Background(), pool, d); err != nil {
		t.Fatal(err)
	}
	if err := (document.Repository{}).Publish(context.Background(), pool, d.ID); err != nil {
		t.Fatal(err)
	}
}

func newJob(t *testing.T, pool *pgxpool.Pool, j *job.Job) uuid.UUID {
	t.Helper()
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	if err := (&job.Repository{}).Create(context.Background(), pool, j); err != nil {
		t.Fatal(err)
	}
	return j.ID
}

func getJob(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *job.Job {
	t.Helper()
	j, err := (&job.Repository{}).GetByID(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func aiRunCount(t *testing.T, pool *pgxpool.Pool, jobID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM ai_runs WHERE job_id = $1 AND redacted`, jobID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRegisterCoversAllAIJobKinds(t *testing.T) {
	reg := worker.NewRegistry()
	aihandlers.Register(reg, aihandlers.Deps{Provider: ai.NotConfiguredProvider{}})
	for _, kind := range []string{
		job.KindClaudeConnectivityTest,
		job.KindLessonRegenerate,
		job.KindQuizGenerate,
		job.KindReviewQuizGenerate,
		job.KindAttemptGrade,
		job.KindWrongAnswerAnalyze,
		job.KindSummaryEvaluate,
	} {
		if _, ok := reg.Lookup(kind); !ok {
			t.Fatalf("未注册 AI handler: %s", kind)
		}
	}
}

func TestConnectivityJobRecordsAIRun(t *testing.T) {
	e := newEnv(t)
	e.fake.Enqueue(ai.Response{JSON: json.RawMessage(`{"ok":true}`), Model: "fake-claude", ProviderRequestID: "req_conn"})
	jobID := newJob(t, e.pool, &job.Job{
		Kind: "claude_connectivity_test", AggregateType: "system", AggregateID: uuid.New(),
		Payload: []byte(`{}`), MaxAttempts: 1,
	})

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusSucceeded {
		t.Fatalf("connectivity 应成功: %s %v", j.Status, j.ErrorCode)
	}
	if aiRunCount(t, e.pool, jobID) != 1 {
		t.Fatal("connectivity 应记录一条脱敏 ai_run")
	}
}

func TestConnectivityJobTargetsOnlyRequestedGatewayKey(t *testing.T) {
	provider := newTargetedFakeProvider()
	provider.target.Enqueue(ai.Response{JSON: json.RawMessage(`{"ok":true}`), Model: "fake-claude", ProviderRequestID: "req_target"})
	e := newEnvWithProvider(t, provider, nil)
	jobID := newJob(t, e.pool, &job.Job{
		Kind: job.KindClaudeConnectivityTest, AggregateType: "system", AggregateID: uuid.New(),
		Payload: mustJSON(t, map[string]string{"key_id": "ck-target"}), MaxAttempts: 1,
	})

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusSucceeded {
		t.Fatalf("targeted connectivity 应成功: %s %v", j.Status, j.ErrorCode)
	}
	if got := provider.KeyIDs(); len(got) != 1 || got[0] != "ck-target" {
		t.Fatalf("应仅调用指定 Key，got=%v", got)
	}
	if got := len(provider.normal.Requests()); got != 0 {
		t.Fatalf("定向测试不应走全局路由，normal calls=%d", got)
	}
	if got := len(provider.target.Requests()); got != 1 {
		t.Fatalf("定向测试应调用一次，target calls=%d", got)
	}
	if aiRunCount(t, e.pool, jobID) != 1 {
		t.Fatal("定向 connectivity 应记录一条脱敏 ai_run")
	}
}

func TestConnectivityJobRejectsInvalidTargetPayload(t *testing.T) {
	provider := newTargetedFakeProvider()
	e := newEnvWithProvider(t, provider, nil)
	jobID := newJob(t, e.pool, &job.Job{
		Kind: job.KindClaudeConnectivityTest, AggregateType: "system", AggregateID: uuid.New(),
		Payload: []byte(`{"key_id":42}`), MaxAttempts: 1,
	})

	e.runOnce(t)

	j := getJob(t, e.pool, jobID)
	if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeClaudeInvalidRequest {
		t.Fatalf("无效 payload 应稳定失败: status=%s code=%v", j.Status, j.ErrorCode)
	}
	if len(provider.KeyIDs()) != 0 || len(provider.normal.Requests()) != 0 {
		t.Fatal("无效 payload 不应调用任何 Provider")
	}
}

func TestQuizAndReviewGenerationPublishReadyQuiz(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		source string
	}{
		{job.KindQuizGenerate, "regular"},
		{job.KindReviewQuizGenerate, "retest"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			e := newEnv(t)
			_, lessonID := seedLesson(t, e.pool)
			publishDoc(t, e.pool, lessonID, document.KindRawText, "学习资料：极限定义。")
			publishDoc(t, e.pool, lessonID, document.KindStudentNotes, "学生笔记：极限描述趋势。")
			e.fake.Enqueue(ai.Response{JSON: json.RawMessage(`{
				"title":"AI 生成试卷",
				"questions":[{
					"type":"single_choice",
					"stem":"极限描述什么？",
					"options":{"A":"趋势","B":"随机数"},
					"correct_answer":"A",
					"explanation":"极限描述趋势。",
					"tested_concepts":["极限"],
					"difficulty":"easy",
					"cognitive_level":"recall",
					"score":5
				}]
			}`), Model: "fake-claude", ProviderRequestID: "req_quiz"})
			family := job.GenerationFamilyLessonGeneration
			jobID := newJob(t, e.pool, &job.Job{
				Kind: tc.kind, AggregateType: "lesson", AggregateID: lessonID, GenerationFamily: &family,
				Payload:     mustJSON(t, map[string]any{"lesson_id": lessonID.String(), "question_count": 1, "title": nil}),
				MaxAttempts: 1,
			})

			e.runOnce(t)

			j := getJob(t, e.pool, jobID)
			if j.Status != job.StatusSucceeded {
				t.Fatalf("任务应成功: %s %v", j.Status, j.ErrorCode)
			}
			var count int
			var source string
			if err := e.pool.QueryRow(context.Background(), `SELECT count(*), max(source) FROM quizzes WHERE lesson_id = $1`, lessonID).Scan(&count, &source); err != nil {
				t.Fatal(err)
			}
			if count != 1 || source != tc.source {
				t.Fatalf("试卷发布错误: count=%d source=%s", count, source)
			}
			if aiRunCount(t, e.pool, jobID) != 1 {
				t.Fatal("quiz 生成应记录一条脱敏 ai_run")
			}
		})
	}
}

func TestProviderFailureMatrixDoesNotPublishQuiz(t *testing.T) {
	for _, code := range []string{
		ai.ErrNotConfigured,
		ai.ErrAuthFailed,
		ai.ErrRateLimited,
		ai.ErrTimeout,
		ai.ErrOverloaded,
		ai.ErrInvalidRequest,
		ai.ErrRefused,
		ai.ErrOutputTruncated,
		ai.ErrInvalidJSON,
		ai.ErrSchemaInvalid,
		ai.ErrDomainInvalid,
	} {
		t.Run(code, func(t *testing.T) {
			e := newEnv(t)
			_, lessonID := seedLesson(t, e.pool)
			publishDoc(t, e.pool, lessonID, document.KindRawText, "学习资料")
			e.fake.EnqueueError(ai.ErrorForCode(code, "脱敏失败摘要"))
			family := job.GenerationFamilyLessonGeneration
			jobID := newJob(t, e.pool, &job.Job{
				Kind: job.KindQuizGenerate, AggregateType: "lesson", AggregateID: lessonID, GenerationFamily: &family,
				Payload:     mustJSON(t, map[string]any{"lesson_id": lessonID.String(), "question_count": 1}),
				MaxAttempts: 1,
			})

			e.runOnce(t)

			j := getJob(t, e.pool, jobID)
			if j.Status != job.StatusFailed || j.ErrorCode == nil || *j.ErrorCode != code {
				t.Fatalf("错误应进入 failed(%s): status=%s code=%v", code, j.Status, j.ErrorCode)
			}
			var quizzes int
			if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM quizzes WHERE lesson_id = $1`, lessonID).Scan(&quizzes); err != nil {
				t.Fatal(err)
			}
			if quizzes != 0 {
				t.Fatalf("Provider 失败不得发布试卷: %d", quizzes)
			}
		})
	}
}

func TestAttemptGradeAppliesFeedbackWeaknessAndMastery(t *testing.T) {
	e := newEnv(t)
	courseID, lessonID := seedLesson(t, e.pool)
	qID := uuid.New()
	if err := (quiz.Repository{}).CreateQuestion(context.Background(), e.pool, &quiz.Question{
		ID: qID, LessonID: lessonID, Type: "short_answer", Stem: "解释极限。",
		CorrectAnswer: []byte(`"标准理解"`), TestedConcepts: []byte(`["极限"]`),
		Difficulty: "medium", CognitiveLevel: "explain", DefaultScore: 10,
	}); err != nil {
		t.Fatal(err)
	}
	z := &quiz.Quiz{
		LessonID: lessonID, Title: "主观题测验", Source: "regular", Status: "ready",
		Items: []quiz.Item{{QuestionID: qID, Position: 1, Score: 10}},
	}
	if err := (quiz.Repository{}).Create(context.Background(), e.pool, z); err != nil {
		t.Fatal(err)
	}
	a := &attempt.Attempt{QuizID: z.ID}
	if err := (attempt.Repository{}).Create(context.Background(), e.pool, a); err != nil {
		t.Fatal(err)
	}
	if _, err := (attempt.Repository{}).SaveAnswer(context.Background(), e.pool, a.ID, qID, []byte(`{"value":"我的解释"}`), 0); err != nil {
		t.Fatal(err)
	}
	jobID := newJob(t, e.pool, &job.Job{
		Kind: job.KindAttemptGrade, AggregateType: "attempt", AggregateID: a.ID,
		Payload:     mustJSON(t, map[string]any{"attempt_id": a.ID.String(), "quiz_id": z.ID.String()}),
		MaxAttempts: 1,
	})
	if err := (attempt.Repository{}).StartGrading(context.Background(), e.pool, a.ID, jobID, 0); err != nil {
		t.Fatal(err)
	}
	e.fake.Enqueue(ai.Response{JSON: json.RawMessage(`{
		"question_feedback":[{
			"question_id":"` + qID.String() + `",
			"awarded_score":7,
			"is_correct":false,
			"feedback":"解释不完整",
			"correct_understanding":"极限描述趋近过程",
			"weak_concepts":["极限"]
		}],
		"review_plan":["复习定义"]
	}`), Model: "fake-claude", ProviderRequestID: "req_grade"})

	e.runOnce(t)

	graded, err := (attempt.Repository{}).GetByID(context.Background(), e.pool, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if graded.Status != attempt.StatusGraded || graded.FinalScore == nil || *graded.FinalScore != 7 {
		t.Fatalf("批阅结果未应用: %+v", graded)
	}
	var weaknessCount, snapshotCount int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM weaknesses WHERE course_id = $1`, courseID).Scan(&weaknessCount); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM mastery_snapshots WHERE source_attempt_id = $1`, a.ID).Scan(&snapshotCount); err != nil {
		t.Fatal(err)
	}
	if weaknessCount == 0 || snapshotCount == 0 {
		t.Fatalf("应产生 weakness 与 mastery snapshot: weaknesses=%d snapshots=%d", weaknessCount, snapshotCount)
	}
}

func TestSummaryEvaluateAndWrongAnswerAnalyzePublishSanitizedResults(t *testing.T) {
	e := newEnv(t)
	_, lessonID := seedLesson(t, e.pool)
	publishDoc(t, e.pool, lessonID, document.KindRawText, "学习资料：极限。")

	evalID := uuid.New()
	summaryJobID := uuid.New()
	if _, err := e.pool.Exec(context.Background(), `
		INSERT INTO jobs (id, kind, aggregate_type, aggregate_id, payload, max_attempts)
		VALUES ($1,'summary_evaluate','summary_evaluation',$2,$3,1)`,
		summaryJobID, evalID, mustJSON(t, map[string]any{
			"evaluation_id": evalID.String(), "lesson_id": lessonID.String(), "summary": "极限是趋势",
		})); err != nil {
		t.Fatal(err)
	}
	if err := (&job.Repository{}).AddEvent(context.Background(), e.pool, summaryJobID, job.EventQueued, job.StatusQueued, 0, "任务已入队", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(context.Background(), `
		INSERT INTO summary_evaluations (id, lesson_id, summary_text, status, job_id)
		VALUES ($1,$2,'极限是趋势','evaluating',$3)`, evalID, lessonID, summaryJobID); err != nil {
		t.Fatal(err)
	}
	e.fake.Enqueue(ai.Response{JSON: json.RawMessage(`{
		"coverage_score":0.8,
		"correct_points":["趋势"],
		"misconceptions":[],
		"missing_points":["严谨定义"],
		"suggestions":["补充 epsilon-delta"]
	}`), Model: "fake-claude", ProviderRequestID: "req_summary"})

	e.runOnce(t)

	var status string
	var result []byte
	if err := e.pool.QueryRow(context.Background(), `SELECT status, result FROM summary_evaluations WHERE id = $1`, evalID).Scan(&status, &result); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || !strings.Contains(string(result), "coverage_score") {
		t.Fatalf("summary_evaluation 未发布: status=%s result=%s", status, result)
	}

	wrongJobID := newJob(t, e.pool, &job.Job{
		Kind: job.KindWrongAnswerAnalyze, AggregateType: "lesson", AggregateID: lessonID,
		Payload:     mustJSON(t, map[string]any{"lesson_id": lessonID.String(), "items": []any{map[string]any{"question_id": uuid.NewString()}}}),
		MaxAttempts: 1,
	})
	e.fake.Enqueue(ai.Response{JSON: json.RawMessage(`{"items":[{"analysis":"概念混淆","suggestions":["复习定义"]}]}`), Model: "fake-claude", ProviderRequestID: "req_wrong"})

	e.runOnce(t)

	j := getJob(t, e.pool, wrongJobID)
	if j.Status != job.StatusSucceeded || !strings.Contains(string(j.Result), "analysis") {
		t.Fatalf("wrong_answer_analyze 应发布脱敏 job.result: status=%s result=%s", j.Status, j.Result)
	}
}

func TestLessonRegeneratePublishesRequestedDocuments(t *testing.T) {
	e := newEnv(t)
	_, lessonID := seedLesson(t, e.pool)
	publishDoc(t, e.pool, lessonID, document.KindRawText, "学习资料：极限。")
	e.fake.Enqueue(ai.Response{JSON: json.RawMessage(`{"markdown":"# 教师教纲\n讲解极限"}`), Model: "fake-claude", ProviderRequestID: "req_teacher"})
	e.fake.Enqueue(ai.Response{JSON: json.RawMessage(`{"markdown":"# 学生笔记\n理解极限"}`), Model: "fake-claude", ProviderRequestID: "req_student"})
	family := job.GenerationFamilyLessonGeneration
	jobID := newJob(t, e.pool, &job.Job{
		Kind: job.KindLessonRegenerate, AggregateType: "lesson", AggregateID: lessonID, GenerationFamily: &family,
		Payload: mustJSON(t, map[string]any{
			"lesson_id": lessonID.String(), "kinds": []string{document.KindTeacherOutline, document.KindStudentNotes},
		}),
		MaxAttempts: 1,
	})

	e.runOnce(t)

	for _, kind := range []string{document.KindTeacherOutline, document.KindStudentNotes} {
		d, err := (document.Repository{}).GetActive(context.Background(), e.pool, lessonID, kind)
		if err != nil {
			t.Fatalf("%s 未发布: %v", kind, err)
		}
		if d.SourceJobID == nil || *d.SourceJobID != jobID {
			t.Fatalf("%s source_job_id 错误: %+v", kind, d)
		}
	}
	if aiRunCount(t, e.pool, jobID) != 2 {
		t.Fatal("lesson_regenerate 应记录两条脱敏 ai_run")
	}
}

func TestTextMaterialExistsOnlyForImportFixture(t *testing.T) {
	// 防止未使用的 material import 在本测试包退化；lesson_import 的 AI
	// 扩展测试位于 internal/worker 包，复用 B5 fixture。
	_ = material.KindText
}
