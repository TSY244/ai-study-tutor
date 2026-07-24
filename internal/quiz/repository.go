// Package quiz 提供题目/试卷的 Repository（B2 最小集）。
// Question 是不可变版本实体；Quiz 发布为 ready 后 quiz_items 不可修改
// （B4 应用层保证，本包只提供读取与创建）。
package quiz

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

var (
	// ErrNotFound：试卷或题目不存在。
	ErrNotFound = errors.New("试卷或题目不存在")
)

// Question 是 questions 表的领域表示。
type Question struct {
	ID             uuid.UUID
	LessonID       uuid.UUID
	SourceJobID    *uuid.UUID
	Type           string
	Stem           string
	Options        []byte
	CorrectAnswer  []byte
	Explanation    *string
	TestedConcepts []byte
	Difficulty     string
	CognitiveLevel string
	DefaultScore   float64
	ContentHash    *string
	CreatedAt      time.Time
}

// Item 是 quiz_items 表的领域表示。
type Item struct {
	QuizID     uuid.UUID
	QuestionID uuid.UUID
	Position   int32
	Score      float64
}

// Quiz 是 quizzes 表的领域表示。
type Quiz struct {
	ID             uuid.UUID
	LessonID       uuid.UUID
	Title          string
	Source         string
	PolicySnapshot []byte
	Status         string
	CreatedAt      time.Time
	Items          []Item
}

type Repository struct{}

// CreateQuestion 插入不可变题目。
func (Repository) CreateQuestion(ctx context.Context, q database.Querier, question *Question) error {
	if question.ID == uuid.Nil {
		question.ID = uuid.New()
	}
	if question.TestedConcepts == nil {
		question.TestedConcepts = []byte(`[]`)
	}
	return q.QueryRow(ctx, `
		INSERT INTO questions (id, lesson_id, source_job_id, type, stem, options,
			correct_answer, explanation, tested_concepts, difficulty,
			cognitive_level, default_score, content_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING created_at`,
		question.ID, question.LessonID, question.SourceJobID, question.Type,
		question.Stem, question.Options, question.CorrectAnswer, question.Explanation,
		question.TestedConcepts, question.Difficulty, question.CognitiveLevel,
		question.DefaultScore, question.ContentHash).Scan(&question.CreatedAt)
}

// Create 在调用方事务中插入试卷与全部 quiz_items。
func (Repository) Create(ctx context.Context, q database.Querier, quiz *Quiz) error {
	if quiz.ID == uuid.Nil {
		quiz.ID = uuid.New()
	}
	if quiz.PolicySnapshot == nil {
		quiz.PolicySnapshot = []byte(`{}`)
	}
	if quiz.Status == "" {
		quiz.Status = "draft"
	}
	err := q.QueryRow(ctx, `
		INSERT INTO quizzes (id, lesson_id, title, source, policy_snapshot, status)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING created_at`,
		quiz.ID, quiz.LessonID, quiz.Title, quiz.Source, quiz.PolicySnapshot, quiz.Status).
		Scan(&quiz.CreatedAt)
	if err != nil {
		return err
	}
	for i := range quiz.Items {
		quiz.Items[i].QuizID = quiz.ID
		if _, err := q.Exec(ctx, `
			INSERT INTO quiz_items (quiz_id, question_id, position, score)
			VALUES ($1,$2,$3,$4)`,
			quiz.ID, quiz.Items[i].QuestionID, quiz.Items[i].Position, quiz.Items[i].Score); err != nil {
			return err
		}
	}
	return nil
}

// GetByID 读取试卷与按 position 排序的 items。
func (Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Quiz, error) {
	var quiz Quiz
	err := q.QueryRow(ctx, `
		SELECT id, lesson_id, title, source, policy_snapshot, status, created_at
		FROM quizzes WHERE id = $1`, id).Scan(
		&quiz.ID, &quiz.LessonID, &quiz.Title, &quiz.Source, &quiz.PolicySnapshot,
		&quiz.Status, &quiz.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `
		SELECT question_id, position, score FROM quiz_items
		WHERE quiz_id = $1 ORDER BY position ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it Item
		it.QuizID = id
		if err := rows.Scan(&it.QuestionID, &it.Position, &it.Score); err != nil {
			return nil, err
		}
		quiz.Items = append(quiz.Items, it)
	}
	return &quiz, rows.Err()
}

// UpdateStatus 更新试卷状态（draft/ready/archived）。
func (Repository) UpdateStatus(ctx context.Context, q database.Querier, id uuid.UUID, status string) error {
	tag, err := q.Exec(ctx, `UPDATE quizzes SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateQuestions 在调用方事务中批量插入不可变题目。
func (r Repository) CreateQuestions(ctx context.Context, q database.Querier, questions []Question) error {
	for i := range questions {
		if err := r.CreateQuestion(ctx, q, &questions[i]); err != nil {
			return err
		}
	}
	return nil
}

// QuizQuestion 是 quiz_items 与 questions 的连接结果（题目顺序与当次分值
// 固定在试卷层）。
type QuizQuestion struct {
	Position int32
	Score    float64
	Question Question
}

// GetWithQuestions 读取试卷及其按 position 排序的题目详情。
// 学生可见性裁剪在 service/DTO 层完成，本方法返回全部列。
func (r Repository) GetWithQuestions(ctx context.Context, q database.Querier, id uuid.UUID) (*Quiz, []QuizQuestion, error) {
	quiz, err := r.GetByID(ctx, q, id)
	if err != nil {
		return nil, nil, err
	}
	rows, err := q.Query(ctx, `
		SELECT qi.position, qi.score, q.id, q.lesson_id, q.source_job_id, q.type,
		       q.stem, q.options, q.correct_answer, q.explanation, q.tested_concepts,
		       q.difficulty, q.cognitive_level, q.default_score, q.content_hash, q.created_at
		FROM quiz_items qi
		JOIN questions q ON q.id = qi.question_id
		WHERE qi.quiz_id = $1
		ORDER BY qi.position ASC`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []QuizQuestion{}
	for rows.Next() {
		var qq QuizQuestion
		if err := rows.Scan(&qq.Position, &qq.Score, &qq.Question.ID, &qq.Question.LessonID,
			&qq.Question.SourceJobID, &qq.Question.Type, &qq.Question.Stem, &qq.Question.Options,
			&qq.Question.CorrectAnswer, &qq.Question.Explanation, &qq.Question.TestedConcepts,
			&qq.Question.Difficulty, &qq.Question.CognitiveLevel, &qq.Question.DefaultScore,
			&qq.Question.ContentHash, &qq.Question.CreatedAt); err != nil {
			return nil, nil, err
		}
		out = append(out, qq)
	}
	return quiz, out, rows.Err()
}

// SummaryRow 是试卷列表行（QuizSummary DTO 的仓库视图）。
type SummaryRow struct {
	Quiz
	QuestionCount int32
}

// ListByLesson 按 created_at ASC, id ASC 游标分页返回课时试卷（含题目数）。
func (Repository) ListByLesson(ctx context.Context, q database.Querier, lessonID uuid.UUID,
	afterCreatedAt *time.Time, afterID *uuid.UUID, limit int) ([]SummaryRow, error) {
	rows, err := q.Query(ctx, `
		SELECT z.id, z.lesson_id, z.title, z.source, z.policy_snapshot, z.status, z.created_at,
		       (SELECT count(*) FROM quiz_items qi WHERE qi.quiz_id = z.id) AS question_count
		FROM quizzes z
		WHERE z.lesson_id = $1
		  AND ($2::timestamptz IS NULL OR (z.created_at, z.id) > ($2, $3::uuid))
		ORDER BY z.created_at ASC, z.id ASC
		LIMIT $4`, lessonID, afterCreatedAt, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SummaryRow{}
	for rows.Next() {
		var r SummaryRow
		if err := rows.Scan(&r.ID, &r.LessonID, &r.Title, &r.Source, &r.PolicySnapshot,
			&r.Status, &r.CreatedAt, &r.QuestionCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
