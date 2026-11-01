// Package mastery 提供薄弱点与掌握度快照的 Repository（B2 最小集）。
// 不变量：course+COALESCE(lesson,zero)+concept_key 唯一；掌握度只追加快照，
// 同一 Attempt/公式版本重复消费不生成第二条。
package mastery

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
)

var (
	// ErrNotFound：薄弱点不存在。
	ErrNotFound = errors.New("薄弱点不存在")
	// ErrDuplicateSnapshot：同一 Attempt/公式版本的快照已存在（重复消费）。
	ErrDuplicateSnapshot = errors.New("该作答的掌握度快照已存在")
)

// Weakness 是 weaknesses 表的领域表示。
type Weakness struct {
	ID           uuid.UUID
	CourseID     uuid.UUID
	LessonID     *uuid.UUID
	ConceptKey   string
	ConceptLabel string
	Description  string
	Evidence     []byte
	Severity     int16
	ReviewCount  int32
	Mastered     bool
	Version      int64
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
}

// Snapshot 是 mastery_snapshots 表的领域表示。
type Snapshot struct {
	ID              uuid.UUID
	ScopeType       string // course/lesson
	CourseID        uuid.UUID
	LessonID        *uuid.UUID
	FormulaVersion  string
	Value           float64
	Components      []byte
	SourceAttemptID *uuid.UUID
	RecordedAt      time.Time
}

type Repository struct{}

// CreateWeakness 插入薄弱点；唯一键冲突时更新 last_seen/severity/evidence
// （同一概念重复发现算一次记录的生命周期）。
func (Repository) CreateWeakness(ctx context.Context, q database.Querier, w *Weakness) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	if w.Evidence == nil {
		w.Evidence = []byte(`[]`)
	}
	err := q.QueryRow(ctx, `
		INSERT INTO weaknesses (id, course_id, lesson_id, concept_key, concept_label,
			description, evidence, severity)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (course_id, COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid), concept_key)
		DO UPDATE SET last_seen_at = now(), severity = EXCLUDED.severity,
			evidence = EXCLUDED.evidence, version = weaknesses.version + 1
		RETURNING id, version, first_seen_at, last_seen_at`,
		w.ID, w.CourseID, w.LessonID, w.ConceptKey, w.ConceptLabel,
		w.Description, w.Evidence, w.Severity).
		Scan(&w.ID, &w.Version, &w.FirstSeenAt, &w.LastSeenAt)
	return err
}

// GetWeaknessByConcept 按唯一键读取。
func (Repository) GetWeaknessByConcept(ctx context.Context, q database.Querier,
	courseID uuid.UUID, lessonID *uuid.UUID, conceptKey string) (*Weakness, error) {
	var w Weakness
	err := q.QueryRow(ctx, `
		SELECT id, course_id, lesson_id, concept_key, concept_label, description,
		       evidence, severity, review_count, mastered, version, first_seen_at, last_seen_at
		FROM weaknesses
		WHERE course_id = $1
		  AND COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid)
		      = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND concept_key = $3`, courseID, lessonID, conceptKey).Scan(
		&w.ID, &w.CourseID, &w.LessonID, &w.ConceptKey, &w.ConceptLabel, &w.Description,
		&w.Evidence, &w.Severity, &w.ReviewCount, &w.Mastered, &w.Version, &w.FirstSeenAt, &w.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &w, err
}

// AppendSnapshot 追加掌握度快照；同一 Attempt/公式版本重复消费返回
// ErrDuplicateSnapshot（部分唯一索引 uq_mastery_snapshots_source）。
func (Repository) AppendSnapshot(ctx context.Context, q database.Querier, s *Snapshot) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.Components == nil {
		s.Components = []byte(`{}`)
	}
	err := q.QueryRow(ctx, `
		INSERT INTO mastery_snapshots (id, scope_type, course_id, lesson_id,
			formula_version, value, components, source_attempt_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING recorded_at`,
		s.ID, s.ScopeType, s.CourseID, s.LessonID,
		s.FormulaVersion, s.Value, s.Components, s.SourceAttemptID).
		Scan(&s.RecordedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_mastery_snapshots_source" {
		return ErrDuplicateSnapshot
	}
	return err
}

// LatestSnapshot 读取某范围当前掌握度（最新一条快照）。
func (Repository) LatestSnapshot(ctx context.Context, q database.Querier,
	scopeType string, courseID uuid.UUID, lessonID *uuid.UUID) (*Snapshot, error) {
	var s Snapshot
	err := q.QueryRow(ctx, `
		SELECT id, scope_type, course_id, lesson_id, formula_version, value,
		       components, source_attempt_id, recorded_at
		FROM mastery_snapshots
		WHERE scope_type = $1 AND course_id = $2
		  AND COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid)
		      = COALESCE($3, '00000000-0000-0000-0000-000000000000'::uuid)
		ORDER BY recorded_at DESC, id DESC
		LIMIT 1`, scopeType, courseID, lessonID).Scan(
		&s.ID, &s.ScopeType, &s.CourseID, &s.LessonID, &s.FormulaVersion, &s.Value,
		&s.Components, &s.SourceAttemptID, &s.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

// ListSnapshots 返回范围内按时间升序的趋势点（最多 limit 条最近记录，
// 按时间正序输出）。
func (Repository) ListSnapshots(ctx context.Context, q database.Querier,
	scopeType string, courseID uuid.UUID, lessonID *uuid.UUID, limit int) ([]Snapshot, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := q.Query(ctx, `
		SELECT id, scope_type, course_id, lesson_id, formula_version, value,
		       components, source_attempt_id, recorded_at
		FROM (
		    SELECT id, scope_type, course_id, lesson_id, formula_version, value,
		           components, source_attempt_id, recorded_at
		    FROM mastery_snapshots
		    WHERE scope_type = $1 AND course_id = $2
		      AND COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid)
		          = COALESCE($3, '00000000-0000-0000-0000-000000000000'::uuid)
		    ORDER BY recorded_at DESC, id DESC
		    LIMIT $4
		) recent
		ORDER BY recorded_at ASC, id ASC`, scopeType, courseID, lessonID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snapshot{}
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.ID, &s.ScopeType, &s.CourseID, &s.LessonID, &s.FormulaVersion,
			&s.Value, &s.Components, &s.SourceAttemptID, &s.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// HasSnapshotForAttempt 判定某 Attempt 在指定范围/公式版本下已产生快照
// （重复消费的快速判定；并发兜底由 uq_mastery_snapshots_source 完成）。
func (Repository) HasSnapshotForAttempt(ctx context.Context, q database.Querier,
	attemptID uuid.UUID, formulaVersion, scopeType string, courseID uuid.UUID, lessonID *uuid.UUID) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS(
		    SELECT 1 FROM mastery_snapshots
		    WHERE source_attempt_id = $1 AND formula_version = $2
		      AND scope_type = $3 AND course_id = $4
		      AND COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid)
		          = COALESCE($5, '00000000-0000-0000-0000-000000000000'::uuid))`,
		attemptID, formulaVersion, scopeType, courseID, lessonID).Scan(&exists)
	return exists, err
}

// WeaknessFilter 是薄弱点列表的查询过滤。
type WeaknessFilter struct {
	CourseID *uuid.UUID
	LessonID *uuid.UUID
	Mastered *bool
	Severity *int16
	// 游标：first_seen_at ASC, id ASC。
	AfterFirstSeenAt *time.Time
	AfterID          *uuid.UUID
}

// ListWeaknesses 按 first_seen_at ASC, id ASC 游标分页返回薄弱点。
func (Repository) ListWeaknesses(ctx context.Context, q database.Querier,
	f WeaknessFilter, limit int) ([]Weakness, error) {
	rows, err := q.Query(ctx, `
		SELECT id, course_id, lesson_id, concept_key, concept_label, description,
		       evidence, severity, review_count, mastered, version, first_seen_at, last_seen_at
		FROM weaknesses
		WHERE ($1::uuid IS NULL OR course_id = $1)
		  AND ($2::uuid IS NULL OR lesson_id = $2)
		  AND ($3::boolean IS NULL OR mastered = $3)
		  AND ($4::smallint IS NULL OR severity = $4)
		  AND ($5::timestamptz IS NULL OR (first_seen_at, id) > ($5, $6::uuid))
		ORDER BY first_seen_at ASC, id ASC
		LIMIT $7`,
		f.CourseID, f.LessonID, f.Mastered, f.Severity, f.AfterFirstSeenAt, f.AfterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Weakness{}
	for rows.Next() {
		var w Weakness
		if err := rows.Scan(&w.ID, &w.CourseID, &w.LessonID, &w.ConceptKey, &w.ConceptLabel,
			&w.Description, &w.Evidence, &w.Severity, &w.ReviewCount, &w.Mastered,
			&w.Version, &w.FirstSeenAt, &w.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWeaknessByID 按主键读取薄弱点。
func (Repository) GetWeaknessByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Weakness, error) {
	var w Weakness
	err := q.QueryRow(ctx, `
		SELECT id, course_id, lesson_id, concept_key, concept_label, description,
		       evidence, severity, review_count, mastered, version, first_seen_at, last_seen_at
		FROM weaknesses WHERE id = $1`, id).Scan(
		&w.ID, &w.CourseID, &w.LessonID, &w.ConceptKey, &w.ConceptLabel, &w.Description,
		&w.Evidence, &w.Severity, &w.ReviewCount, &w.Mastered, &w.Version, &w.FirstSeenAt, &w.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &w, err
}

// ErrVersionConflict：薄弱点乐观锁冲突，映射 409 version_conflict。
var ErrVersionConflict = errors.New("薄弱点版本冲突")

// ResolveWeakness 手动标记已掌握（mastered=true、severity=1、version+1），
// 携带乐观锁；版本不匹配返回 ErrVersionConflict。
func (Repository) ResolveWeakness(ctx context.Context, q database.Querier,
	id uuid.UUID, expectedVersion int64) (*Weakness, error) {
	return updateWeaknessState(ctx, q, id, expectedVersion, true)
}

// ReopenWeakness 重新激活薄弱点（mastered=false、version+1），携带乐观锁。
func (Repository) ReopenWeakness(ctx context.Context, q database.Querier,
	id uuid.UUID, expectedVersion int64) (*Weakness, error) {
	return updateWeaknessState(ctx, q, id, expectedVersion, false)
}

func updateWeaknessState(ctx context.Context, q database.Querier,
	id uuid.UUID, expectedVersion int64, mastered bool) (*Weakness, error) {
	var w Weakness
	err := q.QueryRow(ctx, `
		UPDATE weaknesses
		SET mastered = $3,
		    severity = CASE WHEN $3 THEN 1 ELSE severity END,
		    last_seen_at = now(), version = version + 1
		WHERE id = $1 AND version = $2
		RETURNING id, course_id, lesson_id, concept_key, concept_label, description,
		          evidence, severity, review_count, mastered, version, first_seen_at, last_seen_at`,
		id, expectedVersion, mastered).Scan(
		&w.ID, &w.CourseID, &w.LessonID, &w.ConceptKey, &w.ConceptLabel, &w.Description,
		&w.Evidence, &w.Severity, &w.ReviewCount, &w.Mastered, &w.Version, &w.FirstSeenAt, &w.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// 区分不存在与版本冲突（同一连接上事务尚未中止，可直接再查）。
		if _, getErr := (Repository{}).GetWeaknessByID(ctx, q, id); errors.Is(getErr, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, ErrVersionConflict
	}
	return &w, err
}

// ListForMastery 返回掌握度计算范围内的薄弱点：课程全部（lessonID 为 nil）
// 或指定课时及其课程级（lesson_id IS NULL）薄弱点。
func (Repository) ListForMastery(ctx context.Context, q database.Querier,
	courseID uuid.UUID, lessonID *uuid.UUID) ([]Weakness, error) {
	rows, err := q.Query(ctx, `
		SELECT id, course_id, lesson_id, concept_key, concept_label, description,
		       evidence, severity, review_count, mastered, version, first_seen_at, last_seen_at
		FROM weaknesses
		WHERE course_id = $1
		  AND ($2::uuid IS NULL OR lesson_id IS NULL OR lesson_id = $2)`,
		courseID, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Weakness{}
	for rows.Next() {
		var w Weakness
		if err := rows.Scan(&w.ID, &w.CourseID, &w.LessonID, &w.ConceptKey, &w.ConceptLabel,
			&w.Description, &w.Evidence, &w.Severity, &w.ReviewCount, &w.Mastered,
			&w.Version, &w.FirstSeenAt, &w.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// CountActive 返回范围内未掌握薄弱点数（范围规则同 ListForMastery）。
func (Repository) CountActive(ctx context.Context, q database.Querier,
	courseID uuid.UUID, lessonID *uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx, `
		SELECT count(*) FROM weaknesses
		WHERE course_id = $1 AND NOT mastered
		  AND ($2::uuid IS NULL OR lesson_id IS NULL OR lesson_id = $2)`,
		courseID, lessonID).Scan(&n)
	return n, err
}

// RecordWrongEvidence 把一次答错证据合并进薄弱点（判分结果应用）：
//   - 新概念：创建（review_count=1，severity 走 NormalizeSeverity 缺省）；
//   - 既有概念：severity 取新旧较大者、mastered 复位、review_count+1；
//   - 证据按 attempt+question 去重（已存在同键证据则不合并、不增加计数，
//     重复消费不重复写入）。
//
// 返回 created=true 表示新建。合并遵循 legacy update_weaknesses 语义
// （formula.go MergeReport 的数据库实现）。
func (r Repository) RecordWrongEvidence(ctx context.Context, q database.Querier,
	w *Weakness, evidenceItem []byte, attemptID, questionID uuid.UUID) (created bool, err error) {
	// 行级锁定既有概念（唯一键）。
	existing, err := r.getWeaknessByConceptForUpdate(ctx, q, w.CourseID, w.LessonID, w.ConceptKey)
	switch {
	case errors.Is(err, ErrNotFound):
		if w.ID == uuid.Nil {
			w.ID = uuid.New()
		}
		if evidenceItem == nil {
			evidenceItem = []byte(`[]`)
		}
		err := q.QueryRow(ctx, `
			INSERT INTO weaknesses (id, course_id, lesson_id, concept_key, concept_label,
				description, evidence, severity, review_count)
			VALUES ($1,$2,$3,$4,$5,$6,jsonb_build_array($7::jsonb),$8,1)
			RETURNING version, first_seen_at, last_seen_at`,
			w.ID, w.CourseID, w.LessonID, w.ConceptKey, w.ConceptLabel,
			w.Description, evidenceItem, int16(NormalizeSeverity(int(w.Severity)))).
			Scan(&w.Version, &w.FirstSeenAt, &w.LastSeenAt)
		if err != nil {
			return false, err
		}
		w.ReviewCount = 1
		w.Mastered = false
		w.Evidence = append([]byte(`[`), append(evidenceItem, ']')...)
		w.Severity = int16(NormalizeSeverity(int(w.Severity)))
		return true, nil
	case err != nil:
		return false, err
	}
	// 证据去重：同一 attempt+question 不重复合并（重复消费防护的第二层；
	// 第一层是掌握度快照的消费标记）。
	var items []map[string]any
	if len(existing.Evidence) > 0 {
		if err := json.Unmarshal(existing.Evidence, &items); err != nil {
			return false, err
		}
	}
	attemptStr, questionStr := attemptID.String(), questionID.String()
	for _, item := range items {
		if item["attempt_id"] == attemptStr && item["question_id"] == questionStr {
			*w = *existing
			return false, nil
		}
	}
	var newItem map[string]any
	if err := json.Unmarshal(evidenceItem, &newItem); err != nil {
		return false, err
	}
	items = append(items, newItem)
	merged, err := json.Marshal(items)
	if err != nil {
		return false, err
	}
	severity := NormalizeSeverity(int(w.Severity))
	err = q.QueryRow(ctx, `
		UPDATE weaknesses
		SET severity = GREATEST(severity, $4), mastered = false,
		    review_count = review_count + 1,
		    evidence = $5,
		    description = CASE WHEN $6 <> '' THEN $6 ELSE description END,
		    last_seen_at = now(), version = version + 1
		WHERE id = $1 AND course_id = $2 AND concept_key = $3
		RETURNING severity, review_count, version, last_seen_at`,
		existing.ID, w.CourseID, w.ConceptKey, severity, merged, w.Description).
		Scan(&w.Severity, &w.ReviewCount, &w.Version, &w.LastSeenAt)
	if err != nil {
		return false, err
	}
	w.ID = existing.ID
	w.Mastered = false
	w.Evidence = merged
	w.FirstSeenAt = existing.FirstSeenAt
	return false, nil
}

// RecordCorrectReview 应用一次答对复习（legacy reduce_weakness_severity 的
// 数据库实现）：severity 降 1（下限 1）、review_count+1、降到 1 且复习 >=2
// 次时标记已掌握。概念不存在或已掌握时不动作（返回 false）。
func (r Repository) RecordCorrectReview(ctx context.Context, q database.Querier,
	courseID uuid.UUID, lessonID *uuid.UUID, conceptKey string) (applied bool, err error) {
	existing, err := r.getWeaknessByConceptForUpdate(ctx, q, courseID, lessonID, conceptKey)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if existing.Mastered {
		return false, nil
	}
	next := ApplyCorrectReview(WeaknessState{
		Severity:    int(existing.Severity),
		ReviewCount: int(existing.ReviewCount),
		Mastered:    existing.Mastered,
	}, 1)
	tag, err := q.Exec(ctx, `
		UPDATE weaknesses
		SET severity = $2, review_count = $3, mastered = $4,
		    last_seen_at = now(), version = version + 1
		WHERE id = $1`,
		existing.ID, next.Severity, next.ReviewCount, next.Mastered)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// getWeaknessByConceptForUpdate 按唯一键锁定读取（合并/复习写路径）。
func (Repository) getWeaknessByConceptForUpdate(ctx context.Context, q database.Querier,
	courseID uuid.UUID, lessonID *uuid.UUID, conceptKey string) (*Weakness, error) {
	var w Weakness
	err := q.QueryRow(ctx, `
		SELECT id, course_id, lesson_id, concept_key, concept_label, description,
		       evidence, severity, review_count, mastered, version, first_seen_at, last_seen_at
		FROM weaknesses
		WHERE course_id = $1
		  AND COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid)
		      = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)
		  AND concept_key = $3
		FOR UPDATE`, courseID, lessonID, conceptKey).Scan(
		&w.ID, &w.CourseID, &w.LessonID, &w.ConceptKey, &w.ConceptLabel, &w.Description,
		&w.Evidence, &w.Severity, &w.ReviewCount, &w.Mastered, &w.Version, &w.FirstSeenAt, &w.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &w, err
}

// ListGradedScores reads the surviving grades used to recompute derived progress.
func (Repository) ListGradedScores(ctx context.Context, q database.Querier, courseID uuid.UUID, lessonID *uuid.UUID) ([]GradedScore, error) {
	rows, err := q.Query(ctx, `
		SELECT a.final_score, a.max_score, a.graded_at
		FROM attempts a
		JOIN quizzes z ON z.id = a.quiz_id
		JOIN lessons l ON l.id = z.lesson_id
		WHERE l.course_id = $1
		  AND ($2::uuid IS NULL OR z.lesson_id = $2)
		  AND a.status = 'graded'
		  AND a.max_score IS NOT NULL AND a.max_score > 0
		ORDER BY a.graded_at ASC, a.id ASC`, courseID, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GradedScore{}
	for rows.Next() {
		var s GradedScore
		if err := rows.Scan(&s.Score, &s.Total, &s.GradedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
