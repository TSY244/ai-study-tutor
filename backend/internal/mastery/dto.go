package mastery

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
)

// WeaknessDTO 是薄弱点视图（契约 Weakness）；evidence 为证据对象数组。
type WeaknessDTO struct {
	ID           string           `json:"id"`
	CourseID     string           `json:"course_id"`
	LessonID     *string          `json:"lesson_id"`
	ConceptKey   string           `json:"concept_key"`
	ConceptLabel string           `json:"concept_label"`
	Description  string           `json:"description"`
	Evidence     []map[string]any `json:"evidence"`
	Severity     int16            `json:"severity"`
	ReviewCount  int32            `json:"review_count"`
	Mastered     bool             `json:"mastered"`
	Version      int64            `json:"version"`
	FirstSeenAt  string           `json:"first_seen_at"`
	LastSeenAt   string           `json:"last_seen_at"`
}

// ToWeaknessDTO 映射薄弱点（evidence JSONB → 对象数组，缺省空数组）。
func ToWeaknessDTO(w *Weakness) WeaknessDTO {
	var lessonID *string
	if w.LessonID != nil {
		s := w.LessonID.String()
		lessonID = &s
	}
	evidence := []map[string]any{}
	if len(w.Evidence) > 0 {
		if err := json.Unmarshal(w.Evidence, &evidence); err != nil {
			evidence = []map[string]any{}
		}
	}
	return WeaknessDTO{
		ID:           w.ID.String(),
		CourseID:     w.CourseID.String(),
		LessonID:     lessonID,
		ConceptKey:   w.ConceptKey,
		ConceptLabel: w.ConceptLabel,
		Description:  w.Description,
		Evidence:     evidence,
		Severity:     w.Severity,
		ReviewCount:  w.ReviewCount,
		Mastered:     w.Mastered,
		Version:      w.Version,
		FirstSeenAt:  httpx.Time(w.FirstSeenAt),
		LastSeenAt:   httpx.Time(w.LastSeenAt),
	}
}

// ProgressDTO 是进度聚合视图（契约 Progress）。
type ProgressDTO struct {
	ScopeType         string             `json:"scope_type"`
	ScopeID           string             `json:"scope_id"`
	Mastery           *float64           `json:"mastery"`
	FormulaVersion    *string            `json:"formula_version"`
	CompletedAttempts int32              `json:"completed_attempts"`
	ActiveWeaknesses  int32              `json:"active_weaknesses"`
	Trend             []ProgressPointDTO `json:"trend"`
}

// ProgressPointDTO 是趋势点（契约 ProgressPoint）。
type ProgressPointDTO struct {
	RecordedAt      string     `json:"recorded_at"`
	Mastery         float64    `json:"mastery"`
	FormulaVersion  string     `json:"formula_version"`
	SourceAttemptID *uuid.UUID `json:"source_attempt_id"`
}

// ToProgressPointDTO 映射快照为趋势点（value 0-100 → 0..1）。
func ToProgressPointDTO(s Snapshot) ProgressPointDTO {
	return ProgressPointDTO{
		RecordedAt:      httpx.Time(s.RecordedAt),
		Mastery:         s.Value / 100,
		FormulaVersion:  s.FormulaVersion,
		SourceAttemptID: s.SourceAttemptID,
	}
}
