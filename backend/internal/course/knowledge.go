package course

import (
	"context"
	"strings"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/google/uuid"
)

type KnowledgeDraft struct {
	Label             string `json:"label"`
	Explanation       string `json:"explanation"`
	LearningObjective string `json:"learning_objective"`
	SourceExcerpt     string `json:"source_excerpt"`
	Importance        string `json:"importance"`
}

type KnowledgePoint struct {
	KnowledgeDraft
	Version     int64     `json:"version"`
	ID          uuid.UUID `json:"id"`
	CourseID    uuid.UUID `json:"course_id"`
	LessonID    uuid.UUID `json:"lesson_id"`
	LessonTitle string    `json:"lesson_title"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Repository) ListKnowledge(ctx context.Context, q database.Querier, courseID uuid.UUID, lessonID *uuid.UUID) ([]KnowledgePoint, error) {
	rows, err := q.Query(ctx, `SELECT k.id,k.course_id,k.lesson_id,l.title,k.label,k.explanation,k.learning_objective,k.source_excerpt,k.importance,k.updated_at,k.version FROM course_knowledge_points k JOIN lessons l ON l.id=k.lesson_id WHERE k.course_id=$1 AND ($2::uuid IS NULL OR k.lesson_id=$2) AND k.active ORDER BY l.ordinal,k.label`, courseID, lessonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KnowledgePoint{}
	for rows.Next() {
		var p KnowledgePoint
		if err := rows.Scan(&p.ID, &p.CourseID, &p.LessonID, &p.LessonTitle, &p.Label, &p.Explanation, &p.LearningObjective, &p.SourceExcerpt, &p.Importance, &p.UpdatedAt, &p.Version); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReplaceKnowledge retains stable IDs and old labels for historical quizzes;
// only the latest successful analysis is visible in the course catalogue.
func (Repository) ReplaceKnowledge(ctx context.Context, q database.Querier, lessonID, jobID uuid.UUID, points []KnowledgeDraft) error {
	if _, err := q.Exec(ctx, `UPDATE course_knowledge_points SET active=false WHERE lesson_id=$1`, lessonID); err != nil {
		return err
	}
	for _, p := range points {
		_, err := q.Exec(ctx, `INSERT INTO course_knowledge_points(id,course_id,lesson_id,concept_key,label,explanation,learning_objective,source_excerpt,importance,source_job_id)
  SELECT $1,course_id,id,$3,$4,$5,$6,$7,$8,$9 FROM lessons WHERE id=$2
  ON CONFLICT (lesson_id,concept_key) DO UPDATE SET label=excluded.label,explanation=excluded.explanation,learning_objective=excluded.learning_objective,source_excerpt=excluded.source_excerpt,importance=excluded.importance,source_job_id=excluded.source_job_id,active=true,updated_at=now(),version=course_knowledge_points.version+1`, uuid.New(), lessonID, strings.ToLower(strings.TrimSpace(p.Label)), p.Label, p.Explanation, p.LearningObjective, p.SourceExcerpt, p.Importance, jobID)
		if err != nil {
			return err
		}
	}
	return nil
}
