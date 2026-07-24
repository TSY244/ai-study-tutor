package attempt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/mastery"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/internal/quiz"
)

// GradingFeedbackInput is the provider-independent subjective grading draft.
type GradingFeedbackInput struct {
	QuestionID           uuid.UUID
	AwardedScore         float64
	IsCorrect            *bool
	Feedback             string
	CorrectUnderstanding string
	WeakConcepts         []string
}

type GradingApplyInput struct {
	AttemptID uuid.UUID
	Items     []GradingFeedbackInput
}

type GradingApplyService struct {
	repo    Repository
	quizzes quiz.Repository
	lessons lesson.Repository
	apply   *mastery.ApplyService
}

func NewGradingApplyService() *GradingApplyService {
	repo := Repository{}
	return &GradingApplyService{
		repo: repo, quizzes: quiz.Repository{}, lessons: lesson.Repository{},
		apply: mastery.NewApplyService(repo),
	}
}

// ApplyInTx validates and publishes AI grading in the caller transaction.
func (s *GradingApplyService) ApplyInTx(ctx context.Context, q database.Querier, in GradingApplyInput) (float64, error) {
	a, err := s.repo.LockForUpdate(ctx, q, in.AttemptID)
	if err != nil {
		return 0, err
	}
	if a.Status != StatusGrading && a.Status != StatusSubmitted {
		return 0, fmt.Errorf("作答状态不允许批阅发布: %s", a.Status)
	}
	z, questions, err := s.quizzes.GetWithQuestions(ctx, q, a.QuizID)
	if err != nil {
		return 0, err
	}
	answers, err := s.repo.ListAnswers(ctx, q, a.ID)
	if err != nil {
		return 0, err
	}
	answersByQuestion := map[uuid.UUID]Answer{}
	for _, ans := range answers {
		answersByQuestion[ans.QuestionID] = ans
	}
	inputByQuestion := map[uuid.UUID]GradingFeedbackInput{}
	for _, item := range in.Items {
		inputByQuestion[item.QuestionID] = item
	}
	auto := 0.0
	if a.AutoScore != nil {
		auto = *a.AutoScore
	}
	finalScore := auto
	outcomes := make([]mastery.OutcomeItem, 0, len(questions))
	for _, qq := range questions {
		concepts := conceptsOf(qq.Question.TestedConcepts)
		if ObjectiveType(qq.Question.Type) {
			ans := answersByQuestion[qq.Question.ID]
			outcomes = append(outcomes, mastery.OutcomeItem{
				QuestionID: qq.Question.ID, Concepts: concepts, Correct: ans.AutoCorrect, Note: "客观题作答判分",
			})
			continue
		}
		item, ok := inputByQuestion[qq.Question.ID]
		if !ok {
			return 0, fmt.Errorf("缺少主观题批阅结果: %s", qq.Question.ID)
		}
		if item.AwardedScore < 0 || item.AwardedScore > qq.Score {
			return 0, fmt.Errorf("题目 %s 得分越界", qq.Question.ID)
		}
		correct := item.IsCorrect
		if correct == nil {
			v := item.AwardedScore >= qq.Score
			correct = &v
		}
		feedback, err := buildAIFeedback(qq, item, correct)
		if err != nil {
			return 0, err
		}
		if err := s.repo.ApplyObjectiveResult(ctx, q, a.ID, qq.Question.ID, correct, item.AwardedScore, feedback); err != nil {
			return 0, err
		}
		finalScore += item.AwardedScore
		note := "主观题批阅"
		if item.Feedback != "" {
			note = item.Feedback
		}
		outcomes = append(outcomes, mastery.OutcomeItem{
			QuestionID: qq.Question.ID, Concepts: append(concepts, item.WeakConcepts...), Correct: correct, Note: note,
		})
	}
	if a.MaxScore != nil && finalScore > *a.MaxScore {
		return 0, errors.New("最终得分超过试卷总分")
	}
	if err := s.repo.CompleteGrading(ctx, q, a.ID, finalScore); err != nil {
		return 0, err
	}
	les, err := s.lessons.GetByID(ctx, q, z.LessonID)
	if err != nil {
		return 0, err
	}
	if _, err := s.apply.ApplyAttemptOutcome(ctx, q, mastery.ApplyInput{
		CourseID: les.CourseID, LessonID: z.LessonID, AttemptID: a.ID, QuizID: z.ID, Items: outcomes,
	}); err != nil {
		return 0, err
	}
	return finalScore, nil
}

func buildAIFeedback(qq quiz.QuizQuestion, item GradingFeedbackInput, correct *bool) ([]byte, error) {
	var correctValue json.RawMessage = json.RawMessage(`null`)
	if len(qq.Question.CorrectAnswer) > 0 {
		correctValue = qq.Question.CorrectAnswer
	}
	correctAnswer, err := json.Marshal(map[string]json.RawMessage{"value": correctValue})
	if err != nil {
		return nil, err
	}
	explanation := qq.Question.Explanation
	return json.Marshal(map[string]any{
		"question_id":           qq.Question.ID.String(),
		"awarded_score":         item.AwardedScore,
		"max_score":             qq.Score,
		"is_correct":            correct,
		"feedback":              item.Feedback,
		"correct_understanding": item.CorrectUnderstanding,
		"correct_answer":        json.RawMessage(correctAnswer),
		"explanation":           explanation,
		"weak_concepts":         item.WeakConcepts,
	})
}
