package quiz

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/contentwrite"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
)

// QuestionInput is the editor's answer-bearing representation, never used by student GETs.
type QuestionInput struct {
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
type ManualQuizCreate struct {
	Title           string          `json:"title"`
	GenerationNotes string          `json:"generation_notes"`
	Questions       []QuestionInput `json:"questions"`
}
type ManualQuizEdit struct {
	Version         int64            `json:"version"`
	Title           *string          `json:"title,omitempty"`
	GenerationNotes *string          `json:"generation_notes,omitempty"`
	Questions       *[]QuestionInput `json:"questions,omitempty"`
}
type QuizEditor struct {
	ID              uuid.UUID       `json:"id"`
	LessonID        uuid.UUID       `json:"lesson_id"`
	Version         int64           `json:"version"`
	Title           string          `json:"title"`
	GenerationNotes string          `json:"generation_notes"`
	Questions       []QuestionInput `json:"questions"`
}

func editor(z *Quiz, questions []QuizQuestion) *QuizEditor {
	out := &QuizEditor{ID: z.ID, LessonID: z.LessonID, Version: z.Version, Title: z.Title, GenerationNotes: generationNotes(z.PolicySnapshot), Questions: []QuestionInput{}}
	for _, q := range questions {
		var options map[string]string
		_ = json.Unmarshal(q.Question.Options, &options)
		concepts := []string{}
		_ = json.Unmarshal(q.Question.TestedConcepts, &concepts)
		out.Questions = append(out.Questions, QuestionInput{q.Question.Type, q.Question.Stem, options, q.Question.CorrectAnswer, q.Question.Explanation, concepts, q.Question.Difficulty, q.Question.CognitiveLevel, q.Score})
	}
	return out
}
func manualDrafts(in ManualQuizCreate) ([]QuestionDraft, error) {
	if strings.TrimSpace(in.Title) == "" || utf8.RuneCountInString(in.Title) > 300 {
		return nil, contentwrite.Invalid("title", "标题须为 1 至 300 字。")
	}
	if utf8.RuneCountInString(in.GenerationNotes) > 3000 {
		return nil, contentwrite.Invalid("generation_notes", "出题备注最多 3000 字。")
	}
	drafts := []QuestionDraft{}
	for _, q := range in.Questions {
		if strings.TrimSpace(q.Stem) == "" || utf8.RuneCountInString(q.Stem) > 20000 || math.IsNaN(q.Score) || math.IsInf(q.Score, 0) || q.Score > 999999.99 {
			return nil, contentwrite.Invalid("questions", "题干为空、过长或分值不合法。")
		}
		switch q.Type {
		case "single_choice":
			var a string
			if json.Unmarshal(q.CorrectAnswer, &a) != nil || strings.TrimSpace(q.Options[a]) == "" {
				return nil, contentwrite.Invalid("questions", "单选答案必须是有效选项。")
			}
		case "multiple_choice":
			var a []string
			if json.Unmarshal(q.CorrectAnswer, &a) != nil || len(a) == 0 {
				return nil, contentwrite.Invalid("questions", "多选答案须为非空选项数组。")
			}
			seen := map[string]bool{}
			for _, k := range a {
				if seen[k] || strings.TrimSpace(q.Options[k]) == "" {
					return nil, contentwrite.Invalid("questions", "多选答案包含重复或无效选项。")
				}
				seen[k] = true
			}
		case "true_false":
			var a bool
			if string(q.CorrectAnswer) == "null" || json.Unmarshal(q.CorrectAnswer, &a) != nil {
				return nil, contentwrite.Invalid("questions", "判断题答案必须是布尔值。")
			}
		case "short_answer", "case_analysis":
			var a string
			if json.Unmarshal(q.CorrectAnswer, &a) != nil || strings.TrimSpace(a) == "" {
				return nil, contentwrite.Invalid("questions", "主观题需要文本参考答案。")
			}
		}
		if q.Type == "single_choice" || q.Type == "multiple_choice" {
			if len(q.Options) < 2 || len(q.Options) > 10 {
				return nil, contentwrite.Invalid("questions", "选择题需要 2 至 10 个选项。")
			}
			for k, v := range q.Options {
				if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
					return nil, contentwrite.Invalid("questions", "选项标签和内容不能为空。")
				}
			}
		}
		drafts = append(drafts, QuestionDraft{q.Type, q.Stem, q.Options, q.CorrectAnswer, q.Explanation, q.TestedConcepts, q.Difficulty, q.CognitiveLevel, q.Score})
	}
	if err := (AssembleInput{Source: "assembled", Drafts: drafts}).Validate(); err != nil {
		return nil, contentwrite.Invalid("questions", err.Error())
	}
	return drafts, nil
}
func (s *QueryService) Editor(ctx context.Context, id uuid.UUID) (*QuizEditor, error) {
	z, qs, err := s.GetQuiz(ctx, id)
	if err != nil {
		return nil, err
	}
	return editor(z, qs), nil
}
func (s *QueryService) CreateManual(ctx context.Context, lessonID uuid.UUID, in ManualQuizCreate, complete func(pgx.Tx, *QuizEditor) error) (*QuizEditor, error) {
	drafts, err := manualDrafts(in)
	if err != nil {
		return nil, err
	}
	var out *QuizEditor
	err = contentwrite.Run(ctx, s.pool, lessonID, func(tx pgx.Tx) error {
		policy, _ := json.Marshal(map[string]any{"generation_notes": in.GenerationNotes, "origin": "manual"})
		z, _, err := NewAssemblyService(s.pool).AssembleInTx(ctx, tx, AssembleInput{LessonID: lessonID, Title: in.Title, Source: "assembled", PolicySnapshot: policy, Drafts: drafts})
		if err != nil {
			return err
		}
		z, qs, err := s.repo.GetWithQuestions(ctx, tx, z.ID)
		if err != nil {
			return err
		}
		out = editor(z, qs)
		if complete != nil {
			return complete(tx, out)
		}
		return nil
	})
	return out, err
}
func (s *QueryService) EditManual(ctx context.Context, id uuid.UUID, in ManualQuizEdit) (*QuizEditor, error) {
	z, _, err := s.GetQuiz(ctx, id)
	if err != nil {
		return nil, err
	}
	var out *QuizEditor
	err = contentwrite.Run(ctx, s.pool, z.LessonID, func(tx pgx.Tx) error {
		// The lesson lock serializes manual edits; the quiz lock also serializes attempt creation.
		var version int64
		err := tx.QueryRow(ctx, `SELECT version FROM quizzes WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			return contentwrite.NotFound()
		}
		if err != nil {
			return err
		}
		if version != in.Version {
			return contentwrite.Conflict()
		}
		current, qs, err := s.repo.GetWithQuestions(ctx, tx, id)
		if err != nil {
			return err
		}
		view := editor(current, qs)
		if in.Title != nil {
			view.Title = *in.Title
		}
		if in.GenerationNotes != nil {
			view.GenerationNotes = *in.GenerationNotes
		}
		questions := view.Questions
		if in.Questions != nil {
			questions = *in.Questions
		}
		drafts, err := manualDrafts(ManualQuizCreate{view.Title, view.GenerationNotes, questions})
		if err != nil {
			return err
		}
		var policy map[string]any
		_ = json.Unmarshal(current.PolicySnapshot, &policy)
		if policy == nil {
			policy = map[string]any{}
		}
		policy["generation_notes"] = view.GenerationNotes
		if in.Questions != nil {
			policy["previous_quiz_id"] = id.String()
			b, _ := json.Marshal(policy)
			next, _, err := NewAssemblyService(s.pool).AssembleInTx(ctx, tx, AssembleInput{LessonID: current.LessonID, Title: view.Title, Source: "assembled", PolicySnapshot: b, Drafts: drafts})
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE quizzes SET status='archived',deleted_at=now(),version=version+1 WHERE id=$1`, id); err != nil {
				return err
			}
			next, qs, err = s.repo.GetWithQuestions(ctx, tx, next.ID)
			if err != nil {
				return err
			}
			out = editor(next, qs)
			return nil
		}
		b, _ := json.Marshal(policy)
		if _, err = tx.Exec(ctx, `UPDATE quizzes SET title=$2,policy_snapshot=$3,version=version+1 WHERE id=$1`, id, view.Title, b); err != nil {
			return err
		}
		current, qs, err = s.repo.GetWithQuestions(ctx, tx, id)
		if err != nil {
			return err
		}
		out = editor(current, qs)
		return nil
	})
	return out, err
}
func (s *QueryService) DeleteManual(ctx context.Context, id uuid.UUID) error {
	z, err := s.repo.GetByID(ctx, s.pool, id)
	if errors.Is(err, ErrNotFound) {
		return contentwrite.NotFound()
	}
	if err != nil {
		return err
	}
	return contentwrite.Run(ctx, s.pool, z.LessonID, func(tx pgx.Tx) error {
		var locked uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM quizzes WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return contentwrite.NotFound()
			}
			return err
		}
		if err := contentwrite.LockQuizAttempts(ctx, tx, []uuid.UUID{id}); err != nil {
			return err
		}
		// A submission may have created a job while deletion waited for its lock.
		if err := contentwrite.Lesson(ctx, tx, z.LessonID); err != nil {
			return err
		}
		var courseID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT course_id FROM lessons WHERE id=$1`, z.LessonID).Scan(&courseID); err != nil {
			return err
		}
		if err := contentwrite.DeleteQuizzes(ctx, tx, []uuid.UUID{id}); err != nil {
			return err
		}
		return mastery.NewApplyService(mastery.Repository{}).Refresh(ctx, tx, courseID, &z.LessonID)
	})
}

type MutationHandler struct {
	svc  *QueryService
	idem *httpx.IdemExecutor
}

func NewMutationHandler(pool *pgxpool.Pool, idem *httpx.IdemExecutor) *MutationHandler {
	return &MutationHandler{NewQueryService(pool), idem}
}
func (h *MutationHandler) RegisterRoutes(rg *gin.RouterGroup, limit gin.HandlerFunc) {
	rg.POST("/lessons/:lesson_id/quizzes", limit, h.create)
	rg.GET("/quizzes/:quiz_id/editor", h.get)
	rg.PATCH("/quizzes/:quiz_id", limit, h.edit)
	rg.DELETE("/quizzes/:quiz_id", h.delete)
}
func (h *MutationHandler) create(c *gin.Context) {
	id, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	var in ManualQuizCreate
	if !httpx.BindJSON(c, &in) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	h.idem.Run(c, "createManualQuiz", httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(in), id.String()), func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		out, err := h.svc.CreateManual(ctx, id, in, func(tx pgx.Tx, q *QuizEditor) error {
			return httpx.CompleteJSON(ctx, tx, h.idem.Svc, rec.ID, 201, q, nil, nil, nil, nil)
		})
		return 201, out, err
	})
}
func (h *MutationHandler) get(c *gin.Context) {
	id, ok := pathUUID(c, "quiz_id")
	if !ok {
		return
	}
	out, err := h.svc.Editor(c.Request.Context(), id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *MutationHandler) edit(c *gin.Context) {
	id, ok := pathUUID(c, "quiz_id")
	if !ok {
		return
	}
	var in ManualQuizEdit
	if !httpx.BindJSON(c, &in) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	if in.Version < 1 {
		httpx.WriteError(c, contentwrite.Invalid("version", "请提供当前版本。"))
		return
	}
	out, err := h.svc.EditManual(c.Request.Context(), id, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *MutationHandler) delete(c *gin.Context) {
	id, ok := pathUUID(c, "quiz_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteManual(c.Request.Context(), id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(204)
}
