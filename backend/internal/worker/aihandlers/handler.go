package aihandlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/attempt"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
	"github.com/TSY244/ai-study-tutor/backend/internal/summary"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/piruntime"
)

type Deps struct {
	Pool     *pgxpool.Pool
	Provider aipkg.Provider
	// QuizCLI 非 nil 时测验/复习测验生成走 Pi SDK 文件契约；
	// nil 保持 Messages API 路径。
	QuizCLI QuizGenerator
}

func Register(reg *worker.Registry, deps Deps) {
	if deps.Provider == nil {
		deps.Provider = aipkg.NotConfiguredProvider{}
	}
	b := base{Pool: deps.Pool, Provider: deps.Provider}
	reg.Register(job.KindClaudeConnectivityTest, connectivityHandler{base: b})
	reg.Register(job.KindLessonRegenerate, lessonRegenerateHandler{base: b})
	reg.Register(job.KindQuizGenerate, quizGenerateHandler{base: b, Review: false, QuizCLI: deps.QuizCLI})
	reg.Register(job.KindReviewQuizGenerate, quizGenerateHandler{base: b, Review: true, QuizCLI: deps.QuizCLI})
	reg.Register(job.KindAttemptGrade, attemptGradeHandler{base: b})
	reg.Register(job.KindWrongAnswerAnalyze, wrongAnswerAnalyzeHandler{base: b})
	reg.Register(job.KindSummaryEvaluate, summaryEvaluateHandler{base: b})
}

type base struct {
	Pool     *pgxpool.Pool
	Provider aipkg.Provider
	runs     aipkg.Repository
}

func (b base) call(ctx context.Context, ex *worker.Execution, req aipkg.Request) (aipkg.Response, uuid.UUID, error) {
	return b.callWith(ctx, ex, req, b.Provider.Generate)
}

func (b base) callWith(
	ctx context.Context,
	ex *worker.Execution,
	req aipkg.Request,
	generate func(context.Context, aipkg.Request) (aipkg.Response, error),
) (aipkg.Response, uuid.UUID, error) {
	resp, err := generate(ctx, req)
	run := aipkg.NewRun(ex.Job.ID, req, resp, err)
	if recErr := b.runs.Create(ctx, ex.Pool, run); recErr != nil {
		return resp, uuid.Nil, worker.WrapError(worker.ErrCodeInternal, "记录 AI 调用审计失败，将自动重试。", true, recErr)
	}
	if err != nil {
		perr := aipkg.AsProviderError(err)
		return resp, run.ID, worker.NewError(perr.Code, perr.Detail, perr.Retryable)
	}
	return resp, run.ID, nil
}

func (b base) markRunFailure(ctx context.Context, ex *worker.Execution, runID uuid.UUID, err error) error {
	code := worker.Classify(err).Code
	if runID != uuid.Nil {
		if recErr := b.runs.MarkError(ctx, ex.Pool, runID, code); recErr != nil {
			return worker.WrapError(worker.ErrCodeInternal, "更新 AI 调用审计失败，将自动重试。", true, recErr)
		}
	}
	return err
}

func noopFail(context.Context, pgx.Tx, *job.Job, string) error { return nil }
func noopCancel(context.Context, pgx.Tx, *job.Job) error       { return nil }

// ---------------------------------------------------------------------------
// Connectivity
// ---------------------------------------------------------------------------

type connectivityHandler struct{ base }

type connectivityPayload struct {
	KeyID string `json:"key_id"`
}

type keyTargetingProvider interface {
	GenerateForKey(context.Context, string, aipkg.Request) (aipkg.Response, error)
}

func (h connectivityHandler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	var payload connectivityPayload
	if err := json.Unmarshal(ex.Job.Payload, &payload); err != nil {
		return nil, worker.NewError(worker.ErrCodeClaudeInvalidRequest, "Claude 连通性测试配置无效。", false)
	}
	if err := ex.Stage(ctx, "connecting", 30, "连接 Claude"); err != nil {
		return nil, err
	}
	generate := h.Provider.Generate
	if payload.KeyID != "" {
		targeted, ok := h.Provider.(keyTargetingProvider)
		if !ok {
			generate = func(context.Context, aipkg.Request) (aipkg.Response, error) {
				return aipkg.Response{}, aipkg.ErrorForCode(aipkg.ErrNotConfigured, "Claude 单 Key 测试仅在多 Key 网关启用时可用。")
			}
		} else {
			generate = func(ctx context.Context, req aipkg.Request) (aipkg.Response, error) {
				return targeted.GenerateForKey(ctx, payload.KeyID, req)
			}
		}
	}
	resp, runID, err := h.callWith(ctx, ex, aipkg.BuildConnectivityRequest(), generate)
	if err != nil {
		return nil, err
	}
	if err := ex.Stage(ctx, "validating", 80, "校验 Claude 连通性结果"); err != nil {
		return nil, err
	}
	var out struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resp.JSON, &out); err != nil || !out.OK {
		return nil, h.markRunFailure(ctx, ex, runID,
			worker.NewError(worker.ErrCodeClaudeSchemaInvalid, "Claude 连通性测试返回无效结果。", true))
	}
	return &worker.Outcome{Publish: func(context.Context, pgx.Tx) ([]byte, error) {
		return json.Marshal(map[string]any{"status": "ok", "model": resp.Model, "request_id": resp.ProviderRequestID})
	}}, nil
}

func (h connectivityHandler) FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error {
	return noopFail(ctx, tx, j, code)
}
func (h connectivityHandler) FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error {
	return noopCancel(ctx, tx, j)
}

// ---------------------------------------------------------------------------
// Lesson regenerate
// ---------------------------------------------------------------------------

type lessonRegenerateHandler struct {
	base
	lessons lesson.Repository
	docs    document.Repository
}

type regenerationPayload struct {
	LessonID uuid.UUID `json:"lesson_id"`
	Kinds    []string  `json:"kinds"`
}

func (h lessonRegenerateHandler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	var p regenerationPayload
	if err := json.Unmarshal(ex.Job.Payload, &p); err != nil || p.LessonID == uuid.Nil {
		return nil, worker.NewError(worker.ErrCodeInternal, "任务输入快照无效，请重新创建任务。", false)
	}
	if len(p.Kinds) == 0 {
		p.Kinds = []string{document.KindTeacherOutline, document.KindStudentNotes}
	}
	if err := ex.Stage(ctx, "loading_context", 10, "读取课时上下文"); err != nil {
		return nil, err
	}
	les, err := h.lessons.GetByID(ctx, ex.Pool, p.LessonID)
	if err != nil {
		return nil, err
	}
	raw, err := h.docs.GetActive(ctx, ex.Pool, p.LessonID, document.KindRawText)
	if err != nil {
		return nil, worker.NewError(worker.ErrCodeClaudeDomainInvalid, "课时缺少 raw_text，无法重生成笔记。", false)
	}
	generated := map[string]string{}
	for _, kind := range p.Kinds {
		stage := "generating_student_notes"
		progress := int16(55)
		if kind == document.KindTeacherOutline {
			stage, progress = "generating_teacher_outline", 40
		}
		if err := ex.Stage(ctx, stage, progress, "调用 Claude 生成文档"); err != nil {
			return nil, err
		}
		req, err := aipkg.BuildLessonDocumentRequest(aipkg.LessonDocumentPromptInput{
			LessonTitle: les.Title, RawText: raw.ContentMD, Kind: kind,
		})
		if err != nil {
			return nil, err
		}
		resp, runID, err := h.call(ctx, ex, req)
		if err != nil {
			return nil, err
		}
		md, err := parseMarkdownResult(resp.JSON)
		if err != nil {
			return nil, h.markRunFailure(ctx, ex, runID, err)
		}
		generated[kind] = md
	}
	if err := ex.Stage(ctx, "validating", 85, "校验生成文档"); err != nil {
		return nil, err
	}
	if len(generated) == 0 {
		return nil, worker.NewError(worker.ErrCodeClaudeSchemaInvalid, "Claude 未生成任何文档。", true)
	}
	if err := ex.Stage(ctx, "publishing", 95, "发布文档版本"); err != nil {
		return nil, err
	}
	return &worker.Outcome{Publish: h.publishDocs(ex.Job.ID, p.LessonID, generated)}, nil
}

func (h lessonRegenerateHandler) publishDocs(jobID, lessonID uuid.UUID, generated map[string]string) worker.PublishFunc {
	return func(ctx context.Context, tx pgx.Tx) ([]byte, error) {
		docIDs := map[string]string{}
		md := generated[document.KindTeacherOutline]
		if md == "" {
			md = generated[document.KindStudentNotes]
		}
		title, err := aipkg.StudySummaryTitle(md)
		if err != nil {
			return nil, err
		}
		if err := (course.Repository{}).ApplyGeneratedTitle(ctx, tx, lessonID, title); err != nil {
			return nil, err
		}
		for kind, md := range generated {
			d := &document.Document{LessonID: lessonID, Kind: kind, ContentMD: md, SourceJobID: &jobID}
			if err := h.docs.CreateVersion(ctx, tx, d); err != nil {
				return nil, err
			}
			if err := h.docs.Publish(ctx, tx, d.ID); err != nil {
				return nil, err
			}
			docIDs[kind] = d.ID.String()
		}
		les, err := h.lessons.GetByID(ctx, tx, lessonID)
		if err != nil {
			return nil, err
		}
		if err := h.lessons.UpdateStatus(ctx, tx, lessonID, "ready", les.Version); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"documents": docIDs})
	}
}

func (h lessonRegenerateHandler) FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error {
	var p regenerationPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.LessonID == uuid.Nil {
		return nil
	}
	les, err := h.lessons.GetByID(ctx, tx, p.LessonID)
	if err != nil {
		return nil
	}
	docs, err := h.docs.ListActiveByLesson(ctx, tx, p.LessonID)
	if err != nil {
		return err
	}
	status := "failed"
	if len(docs) > 0 {
		status = "ready_with_warning"
	}
	return h.lessons.UpdateStatus(ctx, tx, p.LessonID, status, les.Version)
}
func (h lessonRegenerateHandler) FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error {
	return h.FinalizeFail(ctx, tx, j, "cancelled")
}

// ---------------------------------------------------------------------------
// Quiz generation
// ---------------------------------------------------------------------------

type quizGenerateHandler struct {
	base
	Review  bool
	QuizCLI QuizGenerator
	lessons lesson.Repository
	docs    document.Repository
	quiz    *quiz.AssemblyService
}

type quizPayload struct {
	LessonID        uuid.UUID `json:"lesson_id"`
	QuestionCount   *int32    `json:"question_count"`
	Title           *string   `json:"title"`
	GenerationNotes string    `json:"generation_notes"`
}

type quizResult struct {
	Title     string               `json:"title"`
	Questions []questionResultItem `json:"questions"`
}

type questionResultItem struct {
	Type           string            `json:"type"`
	Stem           string            `json:"stem"`
	Options        map[string]string `json:"options"`
	CorrectAnswer  json.RawMessage   `json:"correct_answer"`
	Explanation    *string           `json:"explanation"`
	TestedConcepts []string          `json:"tested_concepts"`
	Difficulty     string            `json:"difficulty"`
	CognitiveLevel string            `json:"cognitive_level"`
	Score          float64           `json:"score"`
}

func (h quizGenerateHandler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	var p quizPayload
	if err := json.Unmarshal(ex.Job.Payload, &p); err != nil || p.LessonID == uuid.Nil {
		return nil, worker.NewError(worker.ErrCodeInternal, "任务输入快照无效，请重新创建任务。", false)
	}
	if err := ex.Stage(ctx, "loading_context", 10, "读取出题上下文"); err != nil {
		return nil, err
	}
	les, raw, student, err := h.lessonContext(ctx, ex.Pool, p.LessonID)
	if err != nil {
		return nil, err
	}
	count := int32(8)
	if p.QuestionCount != nil {
		count = *p.QuestionCount
	}
	if err := ex.Stage(ctx, choose(h.Review, "planning_review", "planning"), 25, "生成出题计划"); err != nil {
		return nil, err
	}
	points, err := h.analyzeKnowledge(ctx, ex, les, raw, student, int(count))
	if err != nil {
		return nil, err
	}
	knowledgeJSON, _ := json.Marshal(map[string]any{"points": points})
	var req aipkg.Request
	generate := h.Provider.Generate
	if h.QuizCLI != nil {
		req = buildQuizCLIRequest(ex.WorkDir, quizCLIPromptInput{
			LessonTitle: les.Title, QuestionCount: int(count), Review: h.Review, GenerationNotes: p.GenerationNotes,
		})
		quizCLI := h.QuizCLI
		workDir := ex.WorkDir
		generate = func(ctx context.Context, r aipkg.Request) (aipkg.Response, error) {
			return quizCLI.Generate(ctx, workDir, r)
		}
	} else {
		req, err = aipkg.BuildQuizRequest(aipkg.QuizPromptInput{
			LessonTitle: les.Title, RawText: raw, StudentNotes: student, QuestionCount: int(count), Review: h.Review, KnowledgeJSON: string(knowledgeJSON), GenerationNotes: p.GenerationNotes,
		})
		if err != nil {
			return nil, err
		}
	}
	if err := ex.Stage(ctx, "generating_questions", 55, "AI 正在针对知识点生成题目"); err != nil {
		return nil, err
	}
	var resp aipkg.Response
	var runID uuid.UUID
	err = piruntime.WithProgress(ctx, func(ctx context.Context, message string) error {
		return ex.Stage(ctx, "generating_questions", 55, message)
	}, func(ctx context.Context) error {
		var callErr error
		resp, runID, callErr = h.callWith(ctx, ex, req, generate)
		return callErr
	})
	if err != nil {
		return nil, err
	}
	var result quizResult
	if err := json.Unmarshal(resp.JSON, &result); err != nil {
		return nil, h.markRunFailure(ctx, ex, runID,
			worker.NewError(worker.ErrCodeClaudeInvalidJSON, "Claude 题目结果不是有效 JSON。", true))
	}
	if err := ex.Stage(ctx, "validating", 85, "校验题目结果"); err != nil {
		return nil, err
	}
	if err := validateKnowledgeQuestions(result, points, int(count), p.GenerationNotes); err != nil {
		return nil, h.markRunFailure(ctx, ex, runID, err)
	}
	input, err := quizAssembleInput(p, h.Review, ex.Job.ID, result)
	if err != nil {
		return nil, h.markRunFailure(ctx, ex, runID, err)
	}
	if err := input.Validate(); err != nil {
		return nil, h.markRunFailure(ctx, ex, runID,
			worker.WrapError(worker.ErrCodeClaudeDomainInvalid, "Claude 题目未通过领域校验，将自动重试。", true, err))
	}
	if err := ex.Stage(ctx, "publishing", 95, "发布试卷"); err != nil {
		return nil, err
	}
	var policy map[string]any
	_ = json.Unmarshal(input.PolicySnapshot, &policy)
	policy["knowledge_points"] = points
	policy["generation_notes"] = p.GenerationNotes
	policy["knowledge_schema_version"] = knowledgeSchemaVersion
	input.PolicySnapshot, _ = json.Marshal(policy)
	return &worker.Outcome{Publish: func(ctx context.Context, tx pgx.Tx) ([]byte, error) {
		if err := (course.Repository{}).ReplaceKnowledge(ctx, tx, p.LessonID, ex.Job.ID, points); err != nil {
			return nil, err
		}
		service := h.quiz
		if service == nil {
			service = quiz.NewAssemblyService(h.Pool)
		}
		z, questions, err := service.AssembleInTx(ctx, tx, input)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"quiz_id": z.ID.String(), "question_count": len(questions), "source": z.Source})
	}}, nil
}

func (h quizGenerateHandler) lessonContext(ctx context.Context, q database.Querier, lessonID uuid.UUID) (*lesson.Lesson, string, string, error) {
	les, err := h.lessons.GetByID(ctx, q, lessonID)
	if err != nil {
		return nil, "", "", err
	}
	raw, err := h.docs.GetActive(ctx, q, lessonID, document.KindRawText)
	if err != nil {
		return nil, "", "", worker.NewError(worker.ErrCodeClaudeDomainInvalid, "课时缺少 raw_text，无法生成测验。", false)
	}
	student := ""
	if d, err := h.docs.GetActive(ctx, q, lessonID, document.KindStudentNotes); err == nil {
		student = d.ContentMD
	}
	return les, raw.ContentMD, student, nil
}

func quizAssembleInput(p quizPayload, review bool, jobID uuid.UUID, result quizResult) (quiz.AssembleInput, error) {
	source := "regular"
	if review {
		source = "retest"
	}
	title := result.Title
	if p.Title != nil && *p.Title != "" {
		title = *p.Title
	}
	drafts := make([]quiz.QuestionDraft, 0, len(result.Questions))
	for _, q := range result.Questions {
		drafts = append(drafts, quiz.QuestionDraft{
			Type: q.Type, Stem: q.Stem, Options: q.Options, CorrectAnswer: q.CorrectAnswer,
			Explanation: q.Explanation, TestedConcepts: q.TestedConcepts,
			Difficulty: q.Difficulty, CognitiveLevel: q.CognitiveLevel, Score: q.Score,
		})
	}
	policy, err := json.Marshal(map[string]any{"prompt_schema": aipkg.SchemaQuizV1, "review": review})
	if err != nil {
		return quiz.AssembleInput{}, err
	}
	return quiz.AssembleInput{LessonID: p.LessonID, Source: source, Title: title, PolicySnapshot: policy, Drafts: drafts, SourceJobID: &jobID}, nil
}

func (h quizGenerateHandler) FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error {
	return noopFail(ctx, tx, j, code)
}
func (h quizGenerateHandler) FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error {
	return noopCancel(ctx, tx, j)
}

// ---------------------------------------------------------------------------
// Attempt grade
// ---------------------------------------------------------------------------

type attemptGradeHandler struct {
	base
	attempts attempt.Repository
	quizzes  quiz.Repository
	apply    *attempt.GradingApplyService
}

type attemptPayload struct {
	AttemptID uuid.UUID `json:"attempt_id"`
	QuizID    uuid.UUID `json:"quiz_id"`
}

type gradeResult struct {
	QuestionFeedback []gradeFeedback `json:"question_feedback"`
	ReviewPlan       []string        `json:"review_plan"`
}

type gradeFeedback struct {
	QuestionID           string   `json:"question_id"`
	AwardedScore         float64  `json:"awarded_score"`
	IsCorrect            *bool    `json:"is_correct"`
	Feedback             string   `json:"feedback"`
	CorrectUnderstanding string   `json:"correct_understanding"`
	WeakConcepts         []string `json:"weak_concepts"`
}

func (h attemptGradeHandler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	p, err := parseAttemptPayload(ex.Job)
	if err != nil {
		return nil, err
	}
	if err := ex.Stage(ctx, "loading_attempt", 10, "读取作答上下文"); err != nil {
		return nil, err
	}
	quizJSON, answersJSON, err := h.gradeContext(ctx, ex.Pool, p.AttemptID)
	if err != nil {
		return nil, err
	}
	if err := ex.Stage(ctx, "scoring_objective", 30, "确认客观题暂定分"); err != nil {
		return nil, err
	}
	req := aipkg.BuildGradeRequest(aipkg.GradePromptInput{QuizJSON: quizJSON, AnswersJSON: answersJSON})
	if err := ex.Stage(ctx, "grading_subjective", 60, "调用 Claude 批阅主观题"); err != nil {
		return nil, err
	}
	resp, runID, err := h.call(ctx, ex, req)
	if err != nil {
		return nil, err
	}
	var result gradeResult
	if err := json.Unmarshal(resp.JSON, &result); err != nil {
		return nil, h.markRunFailure(ctx, ex, runID,
			worker.NewError(worker.ErrCodeClaudeInvalidJSON, "Claude 批阅结果不是有效 JSON。", true))
	}
	items := make([]attempt.GradingFeedbackInput, 0, len(result.QuestionFeedback))
	for _, item := range result.QuestionFeedback {
		qid, err := uuid.Parse(item.QuestionID)
		if err != nil {
			return nil, h.markRunFailure(ctx, ex, runID,
				worker.NewError(worker.ErrCodeClaudeSchemaInvalid, "Claude 批阅结果包含无效 question_id。", true))
		}
		items = append(items, attempt.GradingFeedbackInput{
			QuestionID: qid, AwardedScore: item.AwardedScore, IsCorrect: item.IsCorrect,
			Feedback: item.Feedback, CorrectUnderstanding: item.CorrectUnderstanding, WeakConcepts: item.WeakConcepts,
		})
	}
	if err := ex.Stage(ctx, "validating", 85, "校验批阅结果"); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, h.markRunFailure(ctx, ex, runID,
			worker.NewError(worker.ErrCodeClaudeSchemaInvalid, "Claude 未返回逐题批阅结果。", true))
	}
	if err := ex.Stage(ctx, "publishing", 95, "发布批阅结果"); err != nil {
		return nil, err
	}
	return &worker.Outcome{Publish: func(ctx context.Context, tx pgx.Tx) ([]byte, error) {
		service := h.apply
		if service == nil {
			service = attempt.NewGradingApplyService()
		}
		final, err := service.ApplyInTx(ctx, tx, attempt.GradingApplyInput{AttemptID: p.AttemptID, Items: items})
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"attempt_id": p.AttemptID.String(), "final_score": final, "review_plan_count": len(result.ReviewPlan)})
	}}, nil
}

func parseAttemptPayload(j *job.Job) (attemptPayload, error) {
	var p attemptPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.AttemptID == uuid.Nil {
		return p, worker.NewError(worker.ErrCodeInternal, "任务输入快照无效，请重新创建任务。", false)
	}
	return p, nil
}

func (h attemptGradeHandler) gradeContext(ctx context.Context, q database.Querier, attemptID uuid.UUID) (string, string, error) {
	a, err := h.attempts.GetByID(ctx, q, attemptID)
	if err != nil {
		return "", "", err
	}
	_, questions, err := h.quizzes.GetWithQuestions(ctx, q, a.QuizID)
	if err != nil {
		return "", "", err
	}
	answers, err := h.attempts.ListAnswers(ctx, q, attemptID)
	if err != nil {
		return "", "", err
	}
	quizCtx := make([]map[string]any, 0, len(questions))
	for _, qq := range questions {
		quizCtx = append(quizCtx, map[string]any{
			"question_id": qq.Question.ID.String(), "type": qq.Question.Type, "stem": qq.Question.Stem,
			"score": qq.Score, "correct_answer": json.RawMessage(qq.Question.CorrectAnswer),
			"explanation": qq.Question.Explanation, "tested_concepts": json.RawMessage(qq.Question.TestedConcepts),
		})
	}
	answerCtx := make([]map[string]any, 0, len(answers))
	for _, ans := range answers {
		answerCtx = append(answerCtx, map[string]any{"question_id": ans.QuestionID.String(), "answer": json.RawMessage(ans.Answer)})
	}
	qb, _ := json.Marshal(quizCtx)
	ab, _ := json.Marshal(answerCtx)
	return string(qb), string(ab), nil
}

func (h attemptGradeHandler) FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error {
	p, err := parseAttemptPayload(j)
	if err != nil {
		return nil
	}
	detail := code
	if j.ErrorDetail != nil {
		detail = *j.ErrorDetail
	}
	if err := h.attempts.MarkGradingFailed(ctx, tx, p.AttemptID, code, detail); err != nil {
		if errors.Is(err, attempt.ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}
func (h attemptGradeHandler) FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error {
	return h.FinalizeFail(ctx, tx, j, "cancelled")
}

// ---------------------------------------------------------------------------
// Wrong answer analyze
// ---------------------------------------------------------------------------

type wrongAnswerAnalyzeHandler struct{ base }

func (h wrongAnswerAnalyzeHandler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	if err := ex.Stage(ctx, "loading_attempt", 15, "读取错题上下文"); err != nil {
		return nil, err
	}
	if err := ex.Stage(ctx, "analyzing_wrong_answers", 55, "调用 Claude 分析错题"); err != nil {
		return nil, err
	}
	req := aipkg.BuildWrongAnalyzeRequest(string(ex.Job.Payload))
	resp, runID, err := h.call(ctx, ex, req)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(resp.JSON, &result); err != nil {
		return nil, h.markRunFailure(ctx, ex, runID,
			worker.NewError(worker.ErrCodeClaudeInvalidJSON, "Claude 错题分析不是有效 JSON。", true))
	}
	if _, ok := result["items"].([]any); !ok {
		return nil, h.markRunFailure(ctx, ex, runID,
			worker.NewError(worker.ErrCodeClaudeSchemaInvalid, "Claude 错题分析缺少 items。", true))
	}
	if err := ex.Stage(ctx, "validating", 85, "校验错题分析"); err != nil {
		return nil, err
	}
	if err := ex.Stage(ctx, "publishing", 95, "发布错题分析"); err != nil {
		return nil, err
	}
	return &worker.Outcome{Publish: func(context.Context, pgx.Tx) ([]byte, error) { return resp.JSON, nil }}, nil
}
func (h wrongAnswerAnalyzeHandler) FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error {
	return noopFail(ctx, tx, j, code)
}
func (h wrongAnswerAnalyzeHandler) FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error {
	return noopCancel(ctx, tx, j)
}

// ---------------------------------------------------------------------------
// Summary evaluate
// ---------------------------------------------------------------------------

type summaryEvaluateHandler struct {
	base
	docs    document.Repository
	summary summary.Repository
}

type summaryPayload struct {
	EvaluationID uuid.UUID `json:"evaluation_id"`
	LessonID     uuid.UUID `json:"lesson_id"`
	Summary      string    `json:"summary"`
}

func (h summaryEvaluateHandler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	var p summaryPayload
	if err := json.Unmarshal(ex.Job.Payload, &p); err != nil {
		return nil, worker.NewError(worker.ErrCodeInternal, "任务输入快照无效，请重新创建任务。", false)
	}
	if p.EvaluationID == uuid.Nil && ex.Job.AggregateType == "summary_evaluation" {
		p.EvaluationID = ex.Job.AggregateID
	}
	if err := ex.Stage(ctx, "loading_context", 20, "读取总结评估上下文"); err != nil {
		return nil, err
	}
	if p.Summary == "" && p.EvaluationID != uuid.Nil {
		lessonID, text, err := h.summary.GetSummaryText(ctx, ex.Pool, p.EvaluationID)
		if err != nil {
			return nil, err
		}
		p.LessonID, p.Summary = lessonID, text
	}
	raw, err := h.docs.GetActive(ctx, ex.Pool, p.LessonID, document.KindRawText)
	if err != nil {
		return nil, worker.NewError(worker.ErrCodeClaudeDomainInvalid, "课时缺少 raw_text，无法评估总结。", false)
	}
	if err := ex.Stage(ctx, "evaluating_summary", 60, "调用 Claude 评估总结"); err != nil {
		return nil, err
	}
	resp, runID, err := h.call(ctx, ex, aipkg.BuildSummaryRequest(raw.ContentMD, p.Summary))
	if err != nil {
		return nil, err
	}
	if err := validateSummary(resp.JSON); err != nil {
		return nil, h.markRunFailure(ctx, ex, runID, err)
	}
	if err := ex.Stage(ctx, "validating", 85, "校验总结评估"); err != nil {
		return nil, err
	}
	if err := ex.Stage(ctx, "publishing", 95, "发布总结评估"); err != nil {
		return nil, err
	}
	return &worker.Outcome{Publish: func(ctx context.Context, tx pgx.Tx) ([]byte, error) {
		if p.EvaluationID != uuid.Nil {
			if err := h.summary.Complete(ctx, tx, p.EvaluationID, resp.JSON); err != nil {
				return nil, err
			}
		}
		return resp.JSON, nil
	}}, nil
}

func validateSummary(raw json.RawMessage) error {
	var out struct {
		CoverageScore  float64  `json:"coverage_score"`
		CorrectPoints  []string `json:"correct_points"`
		Misconceptions []string `json:"misconceptions"`
		MissingPoints  []string `json:"missing_points"`
		Suggestions    []string `json:"suggestions"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return worker.NewError(worker.ErrCodeClaudeInvalidJSON, "Claude 总结评估不是有效 JSON。", true)
	}
	if out.CoverageScore < 0 || out.CoverageScore > 1 {
		return worker.NewError(worker.ErrCodeClaudeSchemaInvalid, "Claude 总结评估 coverage_score 越界。", true)
	}
	return nil
}

func (h summaryEvaluateHandler) FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error {
	var p summaryPayload
	_ = json.Unmarshal(j.Payload, &p)
	if p.EvaluationID == uuid.Nil && j.AggregateType == "summary_evaluation" {
		p.EvaluationID = j.AggregateID
	}
	if p.EvaluationID == uuid.Nil {
		return nil
	}
	if err := h.summary.Fail(ctx, tx, p.EvaluationID, code); err != nil {
		if errors.Is(err, summary.ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}
func (h summaryEvaluateHandler) FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error {
	return h.FinalizeFail(ctx, tx, j, "cancelled")
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func parseMarkdownResult(raw json.RawMessage) (string, error) {
	var out struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", worker.NewError(worker.ErrCodeClaudeInvalidJSON, "Claude 文档结果不是有效 JSON。", true)
	}
	if out.Markdown == "" {
		return "", worker.NewError(worker.ErrCodeClaudeSchemaInvalid, "Claude 文档结果缺少 markdown。", true)
	}
	if _, err := aipkg.StudySummaryTitle(out.Markdown); err != nil {
		return "", worker.NewError(worker.ErrCodeClaudeSchemaInvalid, aipkg.AsProviderError(err).Detail, true)
	}
	return out.Markdown, nil
}

func choose(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func errf(code string, retryable bool, format string, args ...any) error {
	return worker.NewError(code, fmt.Sprintf(format, args...), retryable)
}
