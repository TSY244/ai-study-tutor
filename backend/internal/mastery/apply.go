// apply.go 是判分结果的幂等应用服务：weakness evidence 合并与
// formula-versioned mastery snapshots 追加。被 Attempt 提交事务（全客观题
// 同步判分）与 B6 Worker 的 AI 批阅结果应用复用。
//
// 幂等不变量（06 阶段 5）：同一 Attempt 的批阅结果重复消费不重复增加
// weakness、成绩或快照——以 lesson 范围快照为消费标记（同事务写入），
// 证据再按 attempt+question 去重兜底。
package mastery

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz/policy"
)

// GradedScore 是一条已批阅成绩（掌握度公式输入；由 attempt.Repository 返回）。
type GradedScore struct {
	Score    float64
	Total    float64
	GradedAt time.Time
}

// GradedScoreReader 提供重新计算掌握度所需的成绩。
type GradedScoreReader interface {
	ListGradedScores(ctx context.Context, q database.Querier, courseID uuid.UUID, lessonID *uuid.UUID) ([]GradedScore, error)
}

// ScoreReader adds the policy inputs to the progress score reader.
type ScoreReader interface {
	GradedScoreReader
	CountGraded(ctx context.Context, q database.Querier, courseID uuid.UUID, lessonID *uuid.UUID) (int, error)
	ConsecutiveAnswers(ctx context.Context, q database.Querier, attemptID uuid.UUID) ([]*bool, error)
	LatestGradedAttemptID(ctx context.Context, q database.Querier, lessonID uuid.UUID) (*uuid.UUID, error)
}

// OutcomeItem 是一道题的判分结果（ Concepts 为题目测试知识点标签）。
type OutcomeItem struct {
	QuestionID uuid.UUID
	Concepts   []string
	// Correct：nil 表示未作答或主观题待批（未答题与错误答案语义不同，
	// 不产生错误证据也不计复习）。
	Correct *bool
	// Note 是证据摘要（脱敏，不含学习材料正文或完整答案）。
	Note string
}

// ApplyInput 是一次批阅结果应用。
type ApplyInput struct {
	CourseID  uuid.UUID
	LessonID  uuid.UUID
	AttemptID uuid.UUID
	QuizID    uuid.UUID
	Items     []OutcomeItem
}

// ApplyService 应用判分结果；Repository 提供 SQL，公式为纯函数。
type ApplyService struct {
	repo   Repository
	scores GradedScoreReader
}

func NewApplyService(scores GradedScoreReader) *ApplyService {
	return &ApplyService{repo: Repository{}, scores: scores}
}

// evidenceItemJSON 是 weakness.evidence 数组元素的脱敏结构。
type evidenceItemJSON struct {
	AttemptID  string `json:"attempt_id"`
	QuestionID string `json:"question_id"`
	QuizID     string `json:"quiz_id"`
	Note       string `json:"note"`
	RecordedAt string `json:"recorded_at"`
}

// ApplyAttemptOutcome 在调用方事务中应用一次批阅结果：
//  1. 消费标记：lesson 范围已存在该 Attempt 的快照 → 直接返回（重放安全）；
//  2. 错误答案：按知识点合并/新建薄弱点并追加证据（dedup attempt+question）；
//     正确答案：对既有未掌握薄弱点应用复习降严重度；
//  3. 追加 lesson 与 course 两个范围的掌握度快照（含公式版本与组成）。
//
// 返回 applied=false 表示该结果此前已消费（无重复写入）。
func (s *ApplyService) ApplyAttemptOutcome(ctx context.Context, q database.Querier, in ApplyInput) (applied bool, err error) {
	exists, err := s.repo.HasSnapshotForAttempt(ctx, q, in.AttemptID, FormulaV1, "lesson", in.CourseID, &in.LessonID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	recordedAt := time.Now().UTC().Format(time.RFC3339)
	for _, item := range in.Items {
		if item.Correct == nil {
			continue
		}
		for _, concept := range item.Concepts {
			key := NormalizeConceptKey(concept)
			if key == "" {
				continue
			}
			if *item.Correct {
				if _, err := s.repo.RecordCorrectReview(ctx, q, in.CourseID, &in.LessonID, key); err != nil {
					return false, err
				}
				continue
			}
			evidence, err := json.Marshal(evidenceItemJSON{
				AttemptID:  in.AttemptID.String(),
				QuestionID: item.QuestionID.String(),
				QuizID:     in.QuizID.String(),
				Note:       item.Note,
				RecordedAt: recordedAt,
			})
			if err != nil {
				return false, err
			}
			w := &Weakness{
				CourseID:     in.CourseID,
				LessonID:     &in.LessonID,
				ConceptKey:   key,
				ConceptLabel: concept,
				Description:  "需要复习该概念",
				Severity:     3,
			}
			if _, err := s.repo.RecordWrongEvidence(ctx, q, w, evidence, in.AttemptID, item.QuestionID); err != nil {
				return false, err
			}
		}
	}
	if err := s.appendScopeSnapshot(ctx, q, "lesson", in.CourseID, &in.LessonID, &in.AttemptID); err != nil {
		return false, err
	}
	if err := s.appendScopeSnapshot(ctx, q, "course", in.CourseID, nil, &in.AttemptID); err != nil {
		return false, err
	}
	return true, nil
}

// appendScopeSnapshot 计算并追加一个范围的掌握度快照；sourceAttemptID 为 nil
// 表示人工调整来源（不参与重复消费去重）。重复消费
// （uq_mastery_snapshots_source 冲突）视为已写入，不视为失败。
func (s *ApplyService) appendScopeSnapshot(ctx context.Context, q database.Querier,
	scopeType string, courseID uuid.UUID, lessonID *uuid.UUID, sourceAttemptID *uuid.UUID) error {
	scores, err := s.scores.ListGradedScores(ctx, q, courseID, lessonID)
	if err != nil {
		return err
	}
	weaknesses, err := s.repo.ListForMastery(ctx, q, courseID, lessonID)
	if err != nil {
		return err
	}
	records := make([]policy.ScoreRecord, 0, len(scores))
	for _, sc := range scores {
		records = append(records, policy.ScoreRecord{
			Score: sc.Score, Total: sc.Total, Timestamp: sc.GradedAt.UTC().Format(time.RFC3339)})
	}
	inputs := make([]WeaknessInput, 0, len(weaknesses))
	for _, w := range weaknesses {
		inputs = append(inputs, WeaknessInput{Severity: int(w.Severity), Mastered: w.Mastered})
	}
	value := ComputeMastery(records, inputs)
	components, err := json.Marshal(map[string]any{
		"score_mastery":  policy.ComputeWeightedMastery(records),
		"scores_count":   len(records),
		"weakness_count": len(inputs),
		"mastered_count": countMastered(inputs),
		"weakness_scope": scopeType,
	})
	if err != nil {
		return err
	}
	snap := &Snapshot{
		ScopeType:       scopeType,
		CourseID:        courseID,
		LessonID:        lessonID,
		FormulaVersion:  FormulaV1,
		Value:           math.Round(value*10000) / 100, // 百分比，两位小数（NUMERIC(5,2)）
		Components:      components,
		SourceAttemptID: sourceAttemptID,
	}
	if err := s.repo.AppendSnapshot(ctx, q, snap); err != nil {
		if errors.Is(err, ErrDuplicateSnapshot) {
			return nil
		}
		return err
	}
	return nil
}

func countMastered(inputs []WeaknessInput) int {
	var n int
	for _, w := range inputs {
		if w.Mastered {
			n++
		}
	}
	return n
}

// PracticeEvaluator 是常规练习出题策略门禁（quiz.GenerationService.QuizPolicy
// 的生产实现）：题量策略返回 0（掌握度与连续正确达到提前停止线）时判定
// 不需要继续练习，映射 422 practice_not_needed。
type PracticeEvaluator struct {
	Scores  ScoreReader
	Weak    Repository
	Lessons LessonCourseReader
}

// LessonCourseReader 解析课时所属课程（lesson.Repository 实现）。
type LessonCourseReader interface {
	// GetCourseID 返回课时所属课程；课时不存在时 found=false（不跨模块
	// 传递哨兵错误，由调用方映射）。
	GetCourseID(ctx context.Context, q database.Querier, lessonID uuid.UUID) (courseID uuid.UUID, found bool, err error)
}

// Needed 判定当前课时是否需要继续生成普通练习。
func (e *PracticeEvaluator) Needed(ctx context.Context, q database.Querier, lessonID uuid.UUID) (bool, error) {
	courseID, found, err := e.Lessons.GetCourseID(ctx, q, lessonID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, errors.New("课时不存在")
	}
	scores, err := e.Scores.ListGradedScores(ctx, q, courseID, &lessonID)
	if err != nil {
		return false, err
	}
	weaknesses, err := e.Weak.ListForMastery(ctx, q, courseID, &lessonID)
	if err != nil {
		return false, err
	}
	records := make([]policy.ScoreRecord, 0, len(scores))
	for _, sc := range scores {
		records = append(records, policy.ScoreRecord{
			Score: sc.Score, Total: sc.Total, Timestamp: sc.GradedAt.UTC().Format(time.RFC3339)})
	}
	inputs := make([]WeaknessInput, 0, len(weaknesses))
	for _, w := range weaknesses {
		inputs = append(inputs, WeaknessInput{Severity: int(w.Severity), Mastered: w.Mastered})
	}
	masteryValue := ComputeMastery(records, inputs)
	var flags []*bool
	if latest, err := e.Scores.LatestGradedAttemptID(ctx, q, lessonID); err != nil {
		return false, err
	} else if latest != nil {
		flags, err = e.Scores.ConsecutiveAnswers(ctx, q, *latest)
		if err != nil {
			return false, err
		}
	}
	correct, wrong, _ := ConsecutiveStats(flags)
	return policy.DetermineQuestionCount("normal", wrong, correct, masteryValue, nil) > 0, nil
}

// Refresh appends current derived progress after a manual content change.
func (s *ApplyService) Refresh(ctx context.Context, q database.Querier, courseID uuid.UUID, lessonID *uuid.UUID) error {
	if lessonID != nil {
		if err := s.appendScopeSnapshot(ctx, q, "lesson", courseID, lessonID, nil); err != nil {
			return err
		}
	}
	return s.appendScopeSnapshot(ctx, q, "course", courseID, nil, nil)
}
