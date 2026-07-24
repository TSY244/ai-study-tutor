package legacyimport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository owns all importer-specific SQL. Application orchestration in
// Service never interpolates SQL or chooses transaction boundaries.
type Repository struct{}

type sourceRecord struct {
	SourcePath string
	SourceHash string
	SourceType string
	TargetType string
	TargetID   *uuid.UUID
	Status     string
	ReasonCode *string
}

type importedAnswer struct {
	QuestionID uuid.UUID
	Answer     []byte
	Correct    *bool
	Awarded    *float64
}

type importedAttempt struct {
	ID          uuid.UUID
	QuizID      uuid.UUID
	Status      string
	AutoScore   float64
	FinalScore  *float64
	MaxScore    float64
	ErrorCode   *string
	ErrorDetail *string
	SubmittedAt time.Time
	GradedAt    *time.Time
	Answers     []importedAnswer
}

func (Repository) LockManifest(ctx context.Context, tx pgx.Tx, manifestHash string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 734001))`, manifestHash)
	return err
}

func (Repository) FindBatch(ctx context.Context, tx pgx.Tx, manifestHash, resolutionHash string) (uuid.UUID, BatchStatus, json.RawMessage, bool, error) {
	var id uuid.UUID
	var status BatchStatus
	var report json.RawMessage
	err := tx.QueryRow(ctx, `
		SELECT id, status, COALESCE(report, '{}'::jsonb)
		FROM legacy_import_batches
		WHERE source_manifest_hash = $1 AND resolution_sha256 = $2
		ORDER BY created_at DESC
		LIMIT 1`, manifestHash, resolutionHash).Scan(&id, &status, &report)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", nil, false, nil
	}
	return id, status, report, err == nil, err
}

func (Repository) CreateBatch(ctx context.Context, tx pgx.Tx, id uuid.UUID, m Manifest, resolutionHash string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO legacy_import_batches
			(id, source_manifest_hash, source_root_fingerprint, resolution_sha256, importer_version, status, started_at)
		VALUES ($1,$2,$3,$4,$5,'importing',now())`,
		id, m.ManifestSHA256, m.SourceRootFingerprint, resolutionHash, m.ImporterVersion)
	return err
}

func (Repository) FinishBatch(ctx context.Context, tx pgx.Tx, id uuid.UUID, status BatchStatus, report ImportReport) error {
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE legacy_import_batches
		SET status=$2, report=$3, completed_at=now()
		WHERE id=$1`, id, status, body)
	return err
}

func (Repository) Record(ctx context.Context, tx pgx.Tx, batchID uuid.UUID, record sourceRecord) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO legacy_import_records
			(id,batch_id,source_path,source_sha256,source_type,target_type,target_id,status,reason_code)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		uuid.New(), batchID, record.SourcePath, record.SourceHash, record.SourceType,
		record.TargetType, record.TargetID, record.Status, record.ReasonCode)
	return err
}

// SourceTarget resolves a stable source path/hash mapping from any completed
// or review batch. This makes a changed manifest deduplicate unchanged files
// without treating unrelated business rows as equivalent.
func (Repository) SourceTarget(ctx context.Context, tx pgx.Tx, path, hash, targetType string) (*uuid.UUID, bool, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT r.target_id
		FROM legacy_import_records r
		JOIN legacy_import_batches b ON b.id=r.batch_id
		WHERE r.source_path=$1 AND r.source_sha256=$2 AND r.target_type=$3
		  AND r.target_id IS NOT NULL
		  AND r.status IN ('imported','deduplicated')
		  AND b.status IN ('needs_review','completed')
		ORDER BY b.created_at ASC
		LIMIT 1`, path, hash, targetType).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &id, true, nil
}

// SourcePathWasImported detects a source path whose bytes changed. A changed
// legacy file is never permitted to overwrite an imported business entity.
func (Repository) SourcePathWasImported(ctx context.Context, tx pgx.Tx, path, targetType string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM legacy_import_records
			WHERE source_path=$1 AND target_type=$2 AND status IN ('imported','deduplicated')
		)`, path, targetType).Scan(&exists)
	return exists, err
}

func (Repository) SetLessonReady(ctx context.Context, tx pgx.Tx, lessonID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE lessons SET status='ready', version=version+1, updated_at=now()
		WHERE id=$1 AND status IN ('draft','failed','ready_with_warning')`, lessonID)
	return err
}

func (Repository) CreateLegacyMaterial(ctx context.Context, tx pgx.Tx, lessonID uuid.UUID, content string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO materials (id,lesson_id,kind,inline_text,metadata,status)
		VALUES ($1,$2,'text',$3,'{"importer":"legacy-import-v1"}'::jsonb,'ready')`,
		uuid.New(), lessonID, content)
	return err
}

func (Repository) CreateAttempt(ctx context.Context, tx pgx.Tx, value importedAttempt) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO attempts
			(id,quiz_id,status,auto_score,final_score,max_score,grading_error_code,grading_error_detail,
			 started_at,submitted_at,graded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$10)`,
		value.ID, value.QuizID, value.Status, value.AutoScore, value.FinalScore, value.MaxScore,
		value.ErrorCode, value.ErrorDetail, value.SubmittedAt, value.GradedAt)
	if err != nil {
		return err
	}
	for _, answer := range value.Answers {
		if _, err := tx.Exec(ctx, `
			INSERT INTO attempt_answers
				(attempt_id,question_id,answer,answer_version,auto_correct,awarded_score)
			VALUES ($1,$2,$3,1,$4,$5)`,
			value.ID, answer.QuestionID, answer.Answer, answer.Correct, answer.Awarded); err != nil {
			return err
		}
	}
	return nil
}

func (Repository) CompleteFeedback(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, total float64, gradedAt time.Time, feedback map[uuid.UUID]json.RawMessage) (bool, error) {
	for questionID, body := range feedback {
		tag, err := tx.Exec(ctx, `
			UPDATE attempt_answers SET feedback=$3, updated_at=now()
			WHERE attempt_id=$1 AND question_id=$2`, attemptID, questionID, body)
		if err != nil {
			return false, err
		}
		if tag.RowsAffected() != 1 {
			return false, errors.New("feedback answer mapping missing")
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE attempts SET status='graded', final_score=$2, graded_at=$3,
			grading_error_code=NULL, grading_error_detail=NULL, version=version+1
		WHERE id=$1 AND status='submitted'`, attemptID, total, gradedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (Repository) AttemptStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM attempts WHERE id=$1`, id).Scan(&status)
	return status, err
}

func (Repository) MarkSubmittedAttemptsNotResumable(ctx context.Context, tx pgx.Tx, attemptIDs []uuid.UUID) error {
	for _, id := range attemptIDs {
		if _, err := tx.Exec(ctx, `
			UPDATE attempts SET status='grading_failed', grading_error_code='legacy_task_not_resumable',
			grading_error_detail='历史批阅任务不可恢复。', version=version+1
			WHERE id=$1 AND status='submitted'`, id); err != nil {
			return err
		}
	}
	return nil
}
