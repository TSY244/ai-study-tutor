package quiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

// QueryService 提供试卷查询用例；事务边界：只读短事务。
type QueryService struct {
	pool    *pgxpool.Pool
	repo    Repository
	lessons lesson.Repository
}

func NewQueryService(pool *pgxpool.Pool) *QueryService {
	return &QueryService{pool: pool, repo: Repository{}, lessons: lesson.Repository{}}
}

// ListQuizzes 按 created_at ASC, id ASC 游标分页返回课时试卷；课时不存在返回 404。
func (s *QueryService) ListQuizzes(ctx context.Context, lessonID uuid.UUID,
	afterCreatedAt *time.Time, afterID *uuid.UUID, limit int) ([]SummaryRow, error) {
	if _, err := s.lessons.GetByID(ctx, s.pool, lessonID); err != nil {
		if errors.Is(err, lesson.ErrNotFound) {
			return nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return nil, err
	}
	return s.repo.ListByLesson(ctx, s.pool, lessonID, afterCreatedAt, afterID, limit+1)
}

// GetQuiz 读取试卷及学生可见题目快照；不存在返回 404。
func (s *QueryService) GetQuiz(ctx context.Context, id uuid.UUID) (*Quiz, []QuizQuestion, error) {
	z, questions, err := s.repo.GetWithQuestions(ctx, s.pool, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return nil, nil, err
	}
	return z, questions, nil
}

// QuestionDraft 是模型无关的题目草稿（06 阶段 5：输入可由 fixture/fake/Claude
// 提供；本服务只做领域校验与不可变发布）。
type QuestionDraft struct {
	Type           string
	Stem           string
	Options        map[string]string // 选择题选项；其他题型为 nil
	CorrectAnswer  json.RawMessage   // 必填，JSON 字符串或字符串数组
	Explanation    *string
	TestedConcepts []string
	Difficulty     string
	CognitiveLevel string
	Score          float64
}

// AssembleInput 是一次试卷组装（常规 regular / 复习 retest / 拼卷 assembled）。
type AssembleInput struct {
	LessonID       uuid.UUID
	Source         string // regular/retest/assembled
	Title          string
	PolicySnapshot []byte // 当次策略快照；nil 时存 {}
	Drafts         []QuestionDraft
	SourceJobID    *uuid.UUID
}

// AssemblyService 把题目草稿确定性地发布为 ready 试卷（Quiz 发布后
// 题目/顺序/分值不可修改，编辑只能创建新版本——06 阶段 5 关键不变量）。
// B6 Worker 的 quiz_generate/review_quiz_generate 处理器复用本服务。
type AssemblyService struct {
	pool    *pgxpool.Pool
	repo    Repository
	lessons lesson.Repository
}

func NewAssemblyService(pool *pgxpool.Pool) *AssemblyService {
	return &AssemblyService{pool: pool, repo: Repository{}, lessons: lesson.Repository{}}
}

var (
	validQuestionTypes = map[string]bool{
		"single_choice": true, "multiple_choice": true, "true_false": true,
		"short_answer": true, "case_analysis": true,
	}
	validDifficulties = map[string]bool{
		"easy": true, "medium": true, "hard": true, "challenge": true,
	}
	validCognitiveLevels = map[string]bool{
		"recall": true, "explain": true, "apply": true, "transfer": true, "reflect": true,
	}
	validQuizSources = map[string]bool{
		"regular": true, "retest": true, "assembled": true,
	}
)

// ErrInvalidDraft：题目草稿未通过领域校验（detail 含全部字段错误）。
type ErrInvalidDraft struct {
	Problems []string
}

func (e *ErrInvalidDraft) Error() string {
	return fmt.Sprintf("题目草稿校验失败: %v", e.Problems)
}

// Validate 校验草稿集合；返回 nil 表示可发布。
func (in AssembleInput) Validate() error {
	var problems []string
	if !validQuizSources[in.Source] {
		problems = append(problems, "source 必须是 regular/retest/assembled")
	}
	if len(in.Drafts) == 0 {
		problems = append(problems, "试卷至少包含一道题")
	}
	if len(in.Drafts) > 100 {
		problems = append(problems, "试卷题目数不能超过 100")
	}
	for i, d := range in.Drafts {
		prefix := fmt.Sprintf("第 %d 题", i+1)
		if !validQuestionTypes[d.Type] {
			problems = append(problems, prefix+": type 非法")
		}
		if len(d.Stem) == 0 {
			problems = append(problems, prefix+": stem 不能为空")
		}
		if !validDifficulties[d.Difficulty] {
			problems = append(problems, prefix+": difficulty 非法")
		}
		if !validCognitiveLevels[d.CognitiveLevel] {
			problems = append(problems, prefix+": cognitive_level 非法")
		}
		if d.Score <= 0 {
			problems = append(problems, prefix+": score 必须大于 0")
		}
		if len(d.CorrectAnswer) == 0 || string(d.CorrectAnswer) == "null" {
			problems = append(problems, prefix+": correct_answer 不能为空")
		}
		if (d.Type == "single_choice" || d.Type == "multiple_choice") && len(d.Options) == 0 {
			problems = append(problems, prefix+": 选择题必须提供 options")
		}
	}
	if len(problems) > 0 {
		return &ErrInvalidDraft{Problems: problems}
	}
	return nil
}

// Assemble 在单个事务内发布试卷：业务校验 → 课时存在 → 插入不可变
// Question → 创建 ready Quiz 与固定 quiz_items（position 从 1 递增）。
func (s *AssemblyService) Assemble(ctx context.Context, in AssembleInput) (*Quiz, []Question, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	z, questions, err := s.AssembleInTx(ctx, tx, in)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return z, questions, nil
}

// AssembleInTx 在调用方事务内发布试卷；Worker 成功发布时使用，确保
// 领域结果与 Job 终态同事务提交。
func (s *AssemblyService) AssembleInTx(ctx context.Context, q database.Querier, in AssembleInput) (*Quiz, []Question, error) {
	if err := in.Validate(); err != nil {
		return nil, nil, err
	}
	if _, err := s.lessons.GetByID(ctx, q, in.LessonID); err != nil {
		if errors.Is(err, lesson.ErrNotFound) {
			return nil, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "试卷所属课时不存在。")
		}
		return nil, nil, err
	}
	questions := make([]Question, 0, len(in.Drafts))
	items := make([]Item, 0, len(in.Drafts))
	quizID := uuid.New()
	for i, d := range in.Drafts {
		options := []byte("null")
		if len(d.Options) > 0 {
			var err error
			options, err = json.Marshal(d.Options)
			if err != nil {
				return nil, nil, err
			}
		}
		concepts, err := json.Marshal(d.TestedConcepts)
		if err != nil {
			return nil, nil, err
		}
		q := Question{
			ID:             uuid.New(),
			LessonID:       in.LessonID,
			SourceJobID:    in.SourceJobID,
			Type:           d.Type,
			Stem:           d.Stem,
			Options:        options,
			CorrectAnswer:  d.CorrectAnswer,
			Explanation:    d.Explanation,
			TestedConcepts: concepts,
			Difficulty:     d.Difficulty,
			CognitiveLevel: d.CognitiveLevel,
			DefaultScore:   d.Score,
		}
		questions = append(questions, q)
		items = append(items, Item{
			QuizID:     quizID,
			QuestionID: q.ID,
			Position:   int32(i + 1),
			Score:      d.Score,
		})
	}
	if err := s.repo.CreateQuestions(ctx, q, questions); err != nil {
		return nil, nil, err
	}
	z := &Quiz{
		ID:             quizID,
		LessonID:       in.LessonID,
		Title:          in.Title,
		Source:         in.Source,
		PolicySnapshot: in.PolicySnapshot,
		Status:         "ready",
		Items:          items,
	}
	if z.Title == "" {
		z.Title = "练习试卷"
	}
	if err := s.repo.Create(ctx, q, z); err != nil {
		return nil, nil, err
	}
	return z, questions, nil
}
