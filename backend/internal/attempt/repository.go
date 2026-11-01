// Package attempt 提供作答聚合的 Repository 与客观题确定性判分。
// 版本语义：单题 answer_version 首次为 0（插入后为 1），成功后递增；
// 同题串行、不同题可并发；提交只使用 Attempt 全局 version（03 §6.2）。
// 提交事务与批阅结果应用编排见 service.go。
package attempt

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
)

var (
	// ErrNotFound：作答不存在。
	ErrNotFound = errors.New("作答不存在")
	// ErrAnswerVersionConflict：单题 answer_version 不匹配（客户端应重读后重试）。
	ErrAnswerVersionConflict = errors.New("答案版本冲突")
	// ErrNotInProgress：作答已提交，不能再自动保存。
	ErrNotInProgress = errors.New("作答已提交")
	// ErrVersionConflict：提交时 Attempt 全局 version 不匹配。
	ErrVersionConflict = errors.New("作答版本冲突")
	// ErrNotGradingFailed：grading-jobs 重建只接受 grading_failed 状态的作答。
	ErrNotGradingFailed = errors.New("仅批阅失败的作答可以重建批阅任务")
	// ErrAnswersIncomplete：旧隔离数据不完整（缺分数快照），不能重建批阅任务。
	ErrAnswersIncomplete = errors.New("作答数据不完整，无法重建批阅任务")
)

// Attempt 是 attempts 表的领域表示。
type Attempt struct {
	ID                 uuid.UUID
	QuizID             uuid.UUID
	Status             string
	Version            int64
	AutoScore          *float64
	FinalScore         *float64
	MaxScore           *float64
	GradingJobID       *uuid.UUID
	GradingErrorCode   *string
	GradingErrorDetail *string
	StartedAt          time.Time
	SubmittedAt        *time.Time
	GradedAt           *time.Time
}

// Answer 是 attempt_answers 表的领域表示。
type Answer struct {
	AttemptID     uuid.UUID
	QuestionID    uuid.UUID
	Answer        []byte
	AnswerVersion int64
	AutoCorrect   *bool
	AwardedScore  *float64
	Feedback      []byte
	UpdatedAt     time.Time
}

// 作答状态机（04 §10 / AttemptStatus 枚举）。
const (
	StatusInProgress    = "in_progress"
	StatusSubmitted     = "submitted"
	StatusGrading       = "grading"
	StatusGraded        = "graded"
	StatusGradingFailed = "grading_failed"
)

type Repository struct{}

// Create 创建作答（in_progress，version=1，max_score 为试卷总分快照）。
func (Repository) Create(ctx context.Context, q database.Querier, a *Attempt) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return q.QueryRow(ctx, `
		INSERT INTO attempts (id, quiz_id, max_score)
		VALUES ($1, $2, (SELECT COALESCE(SUM(score), 0) FROM quiz_items WHERE quiz_id = $2))
		RETURNING status, version, max_score, started_at`,
		a.ID, a.QuizID).Scan(&a.Status, &a.Version, &a.MaxScore, &a.StartedAt)
}

// GetByID 读作答。
func (Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Attempt, error) {
	var a Attempt
	err := q.QueryRow(ctx, `
		SELECT id, quiz_id, status, version, auto_score, final_score, max_score,
		       grading_job_id, grading_error_code, grading_error_detail,
		       started_at, submitted_at, graded_at
		FROM attempts WHERE id = $1`, id).Scan(
		&a.ID, &a.QuizID, &a.Status, &a.Version, &a.AutoScore, &a.FinalScore, &a.MaxScore,
		&a.GradingJobID, &a.GradingErrorCode, &a.GradingErrorDetail,
		&a.StartedAt, &a.SubmittedAt, &a.GradedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

// LockForUpdate 锁定并读取作答（提交/重建批阅任务等写路径使用）。
func (Repository) LockForUpdate(ctx context.Context, q database.Querier, id uuid.UUID) (*Attempt, error) {
	var a Attempt
	err := q.QueryRow(ctx, `
		SELECT id, quiz_id, status, version, auto_score, final_score, max_score,
		       grading_job_id, grading_error_code, grading_error_detail,
		       started_at, submitted_at, graded_at
		FROM attempts WHERE id = $1 FOR UPDATE`, id).Scan(
		&a.ID, &a.QuizID, &a.Status, &a.Version, &a.AutoScore, &a.FinalScore, &a.MaxScore,
		&a.GradingJobID, &a.GradingErrorCode, &a.GradingErrorDetail,
		&a.StartedAt, &a.SubmittedAt, &a.GradedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

// SaveResult 是一次成功自动保存的结果。
type SaveResult struct {
	AnswerVersion  int64
	AttemptVersion int64
	SavedAt        time.Time
}

// SaveAnswer 单题自动保存（03 §6.2）：
//   - 首次保存 expectedVersion=0，插入 answer_version=1；
//   - 后续保存必须携带当前 answer_version，成功则 +1；
//   - 同事务锁定 Attempt 并递增全局 version；
//   - 不同题互不阻塞（等待 Attempt 行锁后各自成功）。
func (r Repository) SaveAnswer(ctx context.Context, q database.Querier,
	attemptID, questionID uuid.UUID, answer []byte, expectedVersion int64) (*SaveResult, error) {
	if expectedVersion < 0 {
		return nil, ErrAnswerVersionConflict
	}
	// 锁定 Attempt 行并校验状态。
	var status string
	err := q.QueryRow(ctx,
		`SELECT status FROM attempts WHERE id = $1 FOR UPDATE`, attemptID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != StatusInProgress {
		return nil, ErrNotInProgress
	}
	var res SaveResult
	if expectedVersion == 0 {
		err = q.QueryRow(ctx, `
			INSERT INTO attempt_answers (attempt_id, question_id, answer, answer_version)
			VALUES ($1,$2,$3,1)
			ON CONFLICT (attempt_id, question_id) DO NOTHING
			RETURNING answer_version`, attemptID, questionID, answer).Scan(&res.AnswerVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAnswerVersionConflict
		}
	} else {
		err = q.QueryRow(ctx, `
			UPDATE attempt_answers
			SET answer = $3, answer_version = answer_version + 1, updated_at = now()
			WHERE attempt_id = $1 AND question_id = $2 AND answer_version = $4
			RETURNING answer_version`, attemptID, questionID, answer, expectedVersion).Scan(&res.AnswerVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAnswerVersionConflict
		}
	}
	if err != nil {
		return nil, err
	}
	err = q.QueryRow(ctx, `
		UPDATE attempts SET version = version + 1 WHERE id = $1
		RETURNING version, (SELECT updated_at FROM attempt_answers
		                    WHERE attempt_id = $1 AND question_id = $2)`,
		attemptID, questionID).Scan(&res.AttemptVersion, &res.SavedAt)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// GetAnswer 读取单题答案。
func (Repository) GetAnswer(ctx context.Context, q database.Querier, attemptID, questionID uuid.UUID) (*Answer, error) {
	var a Answer
	a.AttemptID, a.QuestionID = attemptID, questionID
	err := q.QueryRow(ctx, `
		SELECT answer, answer_version, auto_correct, awarded_score, feedback, updated_at
		FROM attempt_answers WHERE attempt_id = $1 AND question_id = $2`,
		attemptID, questionID).Scan(
		&a.Answer, &a.AnswerVersion, &a.AutoCorrect, &a.AwardedScore, &a.Feedback, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

// ListAnswers 读取作答的全部已保存答案（按 question_id 稳定排序；
// 展示层按试卷 position 重排）。
func (Repository) ListAnswers(ctx context.Context, q database.Querier, attemptID uuid.UUID) ([]Answer, error) {
	rows, err := q.Query(ctx, `
		SELECT question_id, answer, answer_version, auto_correct, awarded_score, feedback, updated_at
		FROM attempt_answers WHERE attempt_id = $1 ORDER BY question_id`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Answer{}
	for rows.Next() {
		var a Answer
		a.AttemptID = attemptID
		if err := rows.Scan(&a.QuestionID, &a.Answer, &a.AnswerVersion,
			&a.AutoCorrect, &a.AwardedScore, &a.Feedback, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// EnsureAnswerRows 为试卷全部题目补建缺失的答案行（提交时调用）：
// 未作答题目以 answer=NULL 落行，保证响应与判分覆盖整卷。
func (Repository) EnsureAnswerRows(ctx context.Context, q database.Querier, attemptID, quizID uuid.UUID) error {
	_, err := q.Exec(ctx, `
		INSERT INTO attempt_answers (attempt_id, question_id, answer)
		SELECT $1, qi.question_id, NULL
		FROM quiz_items qi
		WHERE qi.quiz_id = $2
		ON CONFLICT (attempt_id, question_id) DO NOTHING`, attemptID, quizID)
	return err
}

// ApplyObjectiveResult 写入单题客观判分结果与反馈快照。
func (Repository) ApplyObjectiveResult(ctx context.Context, q database.Querier,
	attemptID, questionID uuid.UUID, autoCorrect *bool, awarded float64, feedback []byte) error {
	_, err := q.Exec(ctx, `
		UPDATE attempt_answers
		SET auto_correct = $3, awarded_score = $4, feedback = $5, updated_at = now()
		WHERE attempt_id = $1 AND question_id = $2`,
		attemptID, questionID, autoCorrect, awarded, feedback)
	return err
}

// CompleteObjectiveSubmit 全客观题提交：同步判分完成，attempt 进入 graded。
// 必须在锁定 Attempt（LockForUpdate）且校验 version 后调用。
func (Repository) CompleteObjectiveSubmit(ctx context.Context, q database.Querier,
	id uuid.UUID, autoScore float64) error {
	tag, err := q.Exec(ctx, `
		UPDATE attempts
		SET status = 'graded', auto_score = $2, final_score = $2,
		    submitted_at = now(), graded_at = now(), version = version + 1
		WHERE id = $1`, id, autoScore)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// StartGrading 含主观题提交：记录客观题暂定分并进入 grading（等待 attempt_grade Job）。
func (Repository) StartGrading(ctx context.Context, q database.Querier,
	id, jobID uuid.UUID, autoScore float64) error {
	tag, err := q.Exec(ctx, `
		UPDATE attempts
		SET status = 'grading', auto_score = $2, grading_job_id = $3,
		    submitted_at = now(), version = version + 1
		WHERE id = $1`, id, autoScore, jobID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkGradingFailed 批阅失败终态（Worker 批阅处理器使用；错误码稳定）。
func (Repository) MarkGradingFailed(ctx context.Context, q database.Querier,
	id uuid.UUID, code, detail string) error {
	tag, err := q.Exec(ctx, `
		UPDATE attempts
		SET status = 'grading_failed', grading_error_code = $2, grading_error_detail = $3,
		    version = version + 1
		WHERE id = $1 AND status IN ('submitted', 'grading')`, id, code, detail)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CompleteGrading 写入 AI 批阅终态。调用方负责逐题反馈和 mastery 应用
// 已在同一事务内完成。
func (Repository) CompleteGrading(ctx context.Context, q database.Querier,
	id uuid.UUID, finalScore float64) error {
	tag, err := q.Exec(ctx, `
		UPDATE attempts
		SET status = 'graded', final_score = $2, graded_at = now(),
		    grading_error_code = NULL, grading_error_detail = NULL,
		    version = version + 1
		WHERE id = $1 AND status IN ('submitted', 'grading')`, id, finalScore)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RestartGrading 从 grading_failed 重新进入 grading：绑定新批阅 Job 并清除
// 上次失败信息（grading-jobs 重建与 Job Retry 共用）。
func (Repository) RestartGrading(ctx context.Context, q database.Querier, id, jobID uuid.UUID) error {
	tag, err := q.Exec(ctx, `
		UPDATE attempts
		SET status = 'grading', grading_job_id = $2,
		    grading_error_code = NULL, grading_error_detail = NULL,
		    version = version + 1
		WHERE id = $1 AND status = 'grading_failed'`, id, jobID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListGradedScores 返回范围内全部已批阅成绩（lessonID 为 nil 表示课程范围；
// 行类型 mastery.GradedScore 是掌握度公式的直接输入）。
// 按 graded_at ASC, id ASC 稳定排序；掌握度公式内部再按时间戳加权。
func (Repository) ListGradedScores(ctx context.Context, q database.Querier,
	courseID uuid.UUID, lessonID *uuid.UUID) ([]mastery.GradedScore, error) {
	return (mastery.Repository{}).ListGradedScores(ctx, q, courseID, lessonID)
}

// CountGraded 返回范围内已批阅作答数。
func (Repository) CountGraded(ctx context.Context, q database.Querier,
	courseID uuid.UUID, lessonID *uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx, `
		SELECT count(*)
		FROM attempts a
		JOIN quizzes z ON z.id = a.quiz_id
		JOIN lessons l ON l.id = z.lesson_id
		WHERE l.course_id = $1
		  AND ($2::uuid IS NULL OR z.lesson_id = $2)
		  AND a.status = 'graded'`, courseID, lessonID).Scan(&n)
	return n, err
}

// ConsecutiveAnswers 返回指定作答按试卷 position 排序的逐题正误
// （nil 表示未作答或未判分）。出题策略的连续统计输入。
func (Repository) ConsecutiveAnswers(ctx context.Context, q database.Querier, attemptID uuid.UUID) ([]*bool, error) {
	rows, err := q.Query(ctx, `
		SELECT aa.auto_correct
		FROM attempt_answers aa
		JOIN attempts a ON a.id = aa.attempt_id
		JOIN quiz_items qi ON qi.quiz_id = a.quiz_id AND qi.question_id = aa.question_id
		WHERE aa.attempt_id = $1
		ORDER BY qi.position ASC`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*bool{}
	for rows.Next() {
		var c *bool
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// LatestGradedAttemptID 返回课时最近一次已批阅作答；不存在返回 nil, nil。
func (Repository) LatestGradedAttemptID(ctx context.Context, q database.Querier, lessonID uuid.UUID) (*uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		SELECT a.id
		FROM attempts a
		JOIN quizzes z ON z.id = a.quiz_id
		WHERE z.lesson_id = $1 AND a.status = 'graded'
		ORDER BY a.graded_at DESC, a.id DESC
		LIMIT 1`, lessonID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &id, err
}

// SummaryRow 是作答历史列表行（AttemptSummary DTO 的仓库视图）。
type SummaryRow struct {
	ID          uuid.UUID
	QuizID      uuid.UUID
	QuizTitle   string
	Status      string
	AutoScore   *float64
	FinalScore  *float64
	MaxScore    *float64
	StartedAt   time.Time
	SubmittedAt *time.Time
	GradedAt    *time.Time
}

// ListByLesson 按 started_at DESC, id DESC 游标分页返回课时作答历史
// （跨该课时全部试卷，含试卷标题）。
func (Repository) ListByLesson(ctx context.Context, q database.Querier, lessonID uuid.UUID,
	afterStartedAt *time.Time, afterID *uuid.UUID, limit int) ([]SummaryRow, error) {
	rows, err := q.Query(ctx, `
		SELECT a.id, a.quiz_id, z.title, a.status, a.auto_score, a.final_score, a.max_score,
		       a.started_at, a.submitted_at, a.graded_at
		FROM attempts a
		JOIN quizzes z ON z.id = a.quiz_id
		WHERE z.lesson_id = $1
		  AND ($2::timestamptz IS NULL OR (a.started_at, a.id) < ($2, $3::uuid))
		ORDER BY a.started_at DESC, a.id DESC
		LIMIT $4`, lessonID, afterStartedAt, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SummaryRow{}
	for rows.Next() {
		var r SummaryRow
		if err := rows.Scan(&r.ID, &r.QuizID, &r.QuizTitle, &r.Status,
			&r.AutoScore, &r.FinalScore, &r.MaxScore,
			&r.StartedAt, &r.SubmittedAt, &r.GradedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LockSubmissionParents keeps deletion from taking the parent locks while a
// submission holds the attempt and is about to write progress or create a job.
// Parent-first ordering matches course/lesson/quiz cascade deletion.
func (Repository) LockSubmissionParents(ctx context.Context, q database.Querier, attemptID uuid.UUID) error {
	var locked uuid.UUID
	for _, stmt := range []string{
		`SELECT c.id FROM courses c JOIN lessons l ON l.course_id=c.id JOIN quizzes z ON z.lesson_id=l.id JOIN attempts a ON a.quiz_id=z.id WHERE a.id=$1 FOR KEY SHARE OF c`,
		`SELECT l.id FROM lessons l JOIN quizzes z ON z.lesson_id=l.id JOIN attempts a ON a.quiz_id=z.id WHERE a.id=$1 FOR KEY SHARE OF l`,
	} {
		if err := q.QueryRow(ctx, stmt, attemptID).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
	}
	return nil
}
