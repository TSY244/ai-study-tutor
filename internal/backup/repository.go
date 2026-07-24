package backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

// Repository keeps backup metadata SQL out of command orchestration.
type Repository struct{}

func (Repository) Create(ctx context.Context, q database.Querier, id uuid.UUID, owner, appVersion, displayName string) error {
	_, err := q.Exec(ctx, `
		INSERT INTO backups (id, status, maintenance_owner, app_version, display_name)
		VALUES ($1, 'creating', $2, $3, $4)`, id, owner, appVersion, displayName)
	return err
}

func (Repository) MarkReady(ctx context.Context, q database.Querier, id uuid.UUID, dbVersion string, size int64, manifest Manifest, checksum string) error {
	body, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("编码备份元数据失败: %w", err)
	}
	tag, err := q.Exec(ctx, `
		UPDATE backups
		SET status = 'ready', db_version = $2, size_bytes = $3,
		    manifest = $4::jsonb, checksum = $5, completed_at = now(),
		    error_code = NULL, error_detail = NULL
		WHERE id = $1 AND status = 'creating'`, id, dbVersion, size, body, checksum)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("备份状态更新冲突")
	}
	return nil
}

func (Repository) MarkFailed(ctx context.Context, q database.Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `
		UPDATE backups
		SET status = 'failed', error_code = 'backup_failed',
		    error_detail = '备份未完成；请查看 Tools 容器日志后重试', completed_at = now()
		WHERE id = $1 AND status = 'creating'`, id)
	return err
}

// MarkRestored turns the creating record embedded in a pg_dump back into a
// ready backup record after the bundle files have been published locally.
func (Repository) MarkRestored(ctx context.Context, q database.Querier, id uuid.UUID, size int64, manifest Manifest, checksum string) error {
	body, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("编码恢复备份元数据失败: %w", err)
	}
	tag, err := q.Exec(ctx, `
		UPDATE backups
		SET status = 'ready', db_version = $2, size_bytes = $3,
		    manifest = $4::jsonb, checksum = $5, completed_at = now(),
		    error_code = NULL, error_detail = NULL
		WHERE id = $1 AND status = 'creating'`, id, manifest.DatabaseVersion, size, body, checksum)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("恢复的备份审计记录不存在或状态不正确")
	}
	return nil
}

// DrainState reports work that makes a filesystem/database snapshot unsafe.
func (Repository) DrainState(ctx context.Context, q database.Querier) (activeJobs, temporaryFiles int64, err error) {
	err = q.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM jobs WHERE status IN ('running','validating','cancel_requested')),
			(SELECT count(*) FROM file_objects WHERE state = 'temporary')`).Scan(&activeJobs, &temporaryFiles)
	return activeJobs, temporaryFiles, err
}

func (Repository) DatabaseVersion(ctx context.Context, q database.Querier) (string, error) {
	var version string
	if err := q.QueryRow(ctx, `SHOW server_version`).Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}
