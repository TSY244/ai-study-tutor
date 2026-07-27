package attempt

import (
	"encoding/json"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
)

// 时间格式化（契约 UTC RFC 3339）。本包不能依赖 httpx：httpx 的稳定错误码
// 映射已依赖本包，反向引用会造成包循环。
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

// AnswerDTO 是作答答案视图（契约 AttemptAnswer）。
// is_correct/awarded_score/feedback 按 Attempt 状态裁剪（见 BuildAttemptDTO）。
type AnswerDTO struct {
	QuestionID    string          `json:"question_id"`
	Answer        json.RawMessage `json:"answer"`
	AnswerVersion int64           `json:"answer_version"`
	AwardedScore  *float64        `json:"awarded_score,omitempty"`
	IsCorrect     *bool           `json:"is_correct,omitempty"`
	Feedback      json.RawMessage `json:"feedback,omitempty"`
	UpdatedAt     string          `json:"updated_at"`
}

// AttemptDTO 是作答视图（契约 Attempt）：questions 为不可变 Quiz 的学生可见
// 题目快照；answers 按试卷 position 排序。
type AttemptDTO struct {
	ID                 string             `json:"id"`
	QuizID             string             `json:"quiz_id"`
	Status             string             `json:"status"`
	Version            int64              `json:"version"`
	AutoScore          *float64           `json:"auto_score"`
	FinalScore         *float64           `json:"final_score"`
	MaxScore           *float64           `json:"max_score"`
	GradingJobID       *string            `json:"grading_job_id"`
	GradingErrorCode   *string            `json:"grading_error_code"`
	GradingErrorDetail *string            `json:"grading_error_detail"`
	Questions          []quiz.QuestionDTO `json:"questions"`
	Answers            []AnswerDTO        `json:"answers"`
	StartedAt          string             `json:"started_at"`
	SubmittedAt        *string            `json:"submitted_at"`
	GradedAt           *string            `json:"graded_at"`
}

// SubmitCompletedBody 是 200 AttemptSubmitCompleted（全客观题，无 Job 字段）。
type SubmitCompletedBody struct {
	Attempt AttemptDTO `json:"attempt"`
}

// SubmitAcceptedBody 是 202 AttemptSubmitAccepted（必含 attempt_grade Job）。
type SubmitAcceptedBody struct {
	Attempt AttemptDTO `json:"attempt"`
	Job     job.DTO    `json:"job"`
}

// JobAcceptedBody 是 grading-jobs 重建的 202 AsyncAccepted 响应。
type JobAcceptedBody struct {
	Job job.DTO `json:"job"`
}

// BuildAttemptDTO 按状态映射作答视图（04 §10.2 反馈可见性矩阵）：
//   - in_progress：只有学生答案，不返回判分与反馈；
//   - submitted/grading/grading_failed：返回客观题判分（is_correct、
//     awarded_score），不返回标准答案与解析（feedback 不出现）；
//   - graded：返回完整 QuestionFeedback（含 correct_answer、explanation）。
func BuildAttemptDTO(a *Attempt, z *quiz.Quiz, questions []quiz.QuizQuestion, answers []Answer) AttemptDTO {
	byQuestion := make(map[string]Answer, len(answers))
	for _, ans := range answers {
		byQuestion[ans.QuestionID.String()] = ans
	}
	graded := a.Status == StatusGraded
	showObjective := a.Status == StatusSubmitted || a.Status == StatusGrading ||
		a.Status == StatusGradingFailed || graded

	qs := make([]quiz.QuestionDTO, 0, len(questions))
	as := make([]AnswerDTO, 0, len(questions))
	for _, qq := range questions {
		qs = append(qs, quiz.ToQuestionDTO(qq))
		ans, ok := byQuestion[qq.Question.ID.String()]
		if !ok {
			continue // 未保存答案（仅 in_progress 时可能）
		}
		dto := AnswerDTO{
			QuestionID:    ans.QuestionID.String(),
			Answer:        answerValueOr(ans.Answer),
			AnswerVersion: ans.AnswerVersion,
			UpdatedAt:     formatTime(ans.UpdatedAt),
		}
		switch {
		case graded:
			dto.IsCorrect = ans.AutoCorrect
			dto.AwardedScore = ans.AwardedScore
			if len(ans.Feedback) > 0 {
				dto.Feedback = ans.Feedback
			}
		case showObjective:
			// 只暴露确定性客观判分，不暴露标准答案/解析。
			dto.IsCorrect = ans.AutoCorrect
			dto.AwardedScore = ans.AwardedScore
		}
		as = append(as, dto)
	}
	var jobID *string
	if a.GradingJobID != nil {
		s := a.GradingJobID.String()
		jobID = &s
	}
	return AttemptDTO{
		ID:                 a.ID.String(),
		QuizID:             a.QuizID.String(),
		Status:             a.Status,
		Version:            a.Version,
		AutoScore:          a.AutoScore,
		FinalScore:         a.FinalScore,
		MaxScore:           a.MaxScore,
		GradingJobID:       jobID,
		GradingErrorCode:   a.GradingErrorCode,
		GradingErrorDetail: a.GradingErrorDetail,
		Questions:          qs,
		Answers:            as,
		StartedAt:          formatTime(a.StartedAt),
		SubmittedAt:        formatTimePtr(a.SubmittedAt),
		GradedAt:           formatTimePtr(a.GradedAt),
	}
}

// answerValueOr 把 answer JSONB 规范化为 AnswerValue 对象；NULL（未作答）
// 输出 {"value": null}。
func answerValueOr(raw []byte) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{"value":null}`)
	}
	return raw
}

// SavedAnswerDTO 是自动保存响应（契约 SavedAnswer）。
type SavedAnswerDTO struct {
	AttemptID      string          `json:"attempt_id"`
	QuestionID     string          `json:"question_id"`
	Answer         json.RawMessage `json:"answer"`
	AnswerVersion  int64           `json:"answer_version"`
	AttemptVersion int64           `json:"attempt_version"`
	SavedAt        string          `json:"saved_at"`
}

// AttemptSummaryDTO 是作答历史列表行（契约 AttemptSummary）。
type AttemptSummaryDTO struct {
	ID          string   `json:"id"`
	QuizID      string   `json:"quiz_id"`
	QuizTitle   string   `json:"quiz_title"`
	Status      string   `json:"status"`
	AutoScore   *float64 `json:"auto_score"`
	FinalScore  *float64 `json:"final_score"`
	MaxScore    *float64 `json:"max_score"`
	StartedAt   string   `json:"started_at"`
	SubmittedAt *string  `json:"submitted_at"`
	GradedAt    *string  `json:"graded_at"`
}

// ToAttemptSummaryDTO 映射列表行。
func ToAttemptSummaryDTO(r SummaryRow) AttemptSummaryDTO {
	return AttemptSummaryDTO{
		ID:          r.ID.String(),
		QuizID:      r.QuizID.String(),
		QuizTitle:   r.QuizTitle,
		Status:      r.Status,
		AutoScore:   r.AutoScore,
		FinalScore:  r.FinalScore,
		MaxScore:    r.MaxScore,
		StartedAt:   formatTime(r.StartedAt),
		SubmittedAt: formatTimePtr(r.SubmittedAt),
		GradedAt:    formatTimePtr(r.GradedAt),
	}
}
