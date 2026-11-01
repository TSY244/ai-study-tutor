package contentwrite

import (
	"context"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/google/uuid"
)

// LockLessonDependents closes the race with new attempts and submissions before
// the caller checks active jobs. All deletion paths acquire locks in this order.
func LockLessonDependents(ctx context.Context, q database.Querier, lessonIDs []uuid.UUID) error {
	rows, err := q.Query(ctx, `SELECT id FROM quizzes WHERE lesson_id=ANY($1) ORDER BY id FOR UPDATE`, lessonIDs)
	if err != nil {
		return err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	return LockQuizAttempts(ctx, q, ids)
}

func LockQuizAttempts(ctx context.Context, q database.Querier, quizIDs []uuid.UUID) error {
	rows, err := q.Query(ctx, `SELECT id FROM attempts WHERE quiz_id=ANY($1) ORDER BY id FOR UPDATE`, quizIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() { /* Consume every row so all attempt locks are held. */
	}
	return rows.Err()
}

// DeleteQuizzes removes attempts before their parent quizzes, including derived
// snapshots and evidence outside the deleted lesson. Shared questions survive.
// The caller owns the lesson/quiz/attempt locks and the transaction.
func DeleteQuizzes(ctx context.Context, q database.Querier, quizIDs []uuid.UUID) error {
	rows, err := q.Query(ctx, `SELECT DISTINCT question_id FROM quiz_items WHERE quiz_id=ANY($1)`, quizIDs)
	if err != nil {
		return err
	}
	questionIDs := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		questionIDs = append(questionIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`UPDATE weaknesses w SET evidence=(SELECT COALESCE(jsonb_agg(e),'[]'::jsonb)
     FROM jsonb_array_elements(w.evidence) e
     WHERE NOT (COALESCE(e->>'quiz_id','') IN (SELECT id::text FROM quizzes WHERE id=ANY($1))
       OR COALESCE(e->>'attempt_id','') IN (SELECT id::text FROM attempts WHERE quiz_id=ANY($1)))) ,version=version+1
   WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(w.evidence) e
     WHERE e->>'quiz_id' IN (SELECT id::text FROM quizzes WHERE id=ANY($1))
       OR COALESCE(e->>'attempt_id','') IN (SELECT id::text FROM attempts WHERE quiz_id=ANY($1)))`,
		`DELETE FROM mastery_snapshots WHERE source_attempt_id IN (SELECT id FROM attempts WHERE quiz_id=ANY($1))`,
		`DELETE FROM attempt_answers WHERE attempt_id IN (SELECT id FROM attempts WHERE quiz_id=ANY($1))`,
		`DELETE FROM attempts WHERE quiz_id=ANY($1)`,
		`DELETE FROM quiz_items WHERE quiz_id=ANY($1)`,
		`DELETE FROM quizzes WHERE id=ANY($1)`,
	} {
		if _, err := q.Exec(ctx, stmt, quizIDs); err != nil {
			return err
		}
	}
	_, err = q.Exec(ctx, `DELETE FROM questions q WHERE q.id=ANY($1)
   AND NOT EXISTS(SELECT 1 FROM quiz_items qi WHERE qi.question_id=q.id)
   AND NOT EXISTS(SELECT 1 FROM attempt_answers aa WHERE aa.question_id=q.id)`, questionIDs)
	return err
}
