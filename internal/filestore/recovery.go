package filestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

// RecoverResult 是单次文件 Recovery 的处置计数。
type RecoverResult struct {
	Completed   int // 补全为 ready（hash + 最终路径 + 已提交 Owner 证明）
	Quarantined int // 无法证明发布完成，隔离到 quarantine/maintenance-recovery/
	Deleted     int // 文件已不存在，行标为 deleted
	SkippedLive int // Owner 仍存活，跳过
	Orphaned    int // 无数据库记录的孤立最终对象，隔离到 quarantine/orphan/
}

// Recover 处理过期 temporary 行与孤立最终对象（03 §5.3 第 6 步、
// §6.7 修复步骤 4）。必须在调用方事务中执行；maintenance-repair
// 在 Business Gate exclusive lock 下调用，周期 Recovery 使用普通事务。
//
// 判定规则（使用数据库时间，不依赖主机时钟）：
//
//   - Owner 仍存活（job 租约有效或 job 尚未开始）→ 跳过；
//   - 最终文件存在且 hash 匹配且 Owner 已提交（job succeeded /
//     幂等记录 completed）→ 补全为 ready；
//   - 其他能找到文件的情况 → 原子移动到 quarantine/maintenance-recovery/{id}；
//   - 临时与最终路径都找不到文件 → 行标为 deleted；
//   - 最后扫描 objects/ 下无数据库记录的孤立对象 → quarantine/orphan/。
func (s *Store) Recover(ctx context.Context, q database.Querier, batchSize int) (RecoverResult, error) {
	tx, ok := q.(pgx.Tx)
	if !ok {
		return RecoverResult{}, errors.New("filestore: Recover 必须在事务中执行")
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	rows, err := tx.Query(ctx, `
		SELECT id, storage_key, COALESCE(sha256,''), COALESCE(size_bytes,0),
		       temporary_owner_type, temporary_owner_id
		FROM file_objects
		WHERE state = 'temporary' AND temporary_expires_at <= now()
		ORDER BY temporary_expires_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1`, batchSize)
	if err != nil {
		return RecoverResult{}, err
	}
	type expiredRow struct {
		id        uuid.UUID
		key       string
		sha       string
		size      int64
		ownerType string
		ownerID   uuid.UUID
	}
	var expired []expiredRow
	for rows.Next() {
		var r expiredRow
		if err := rows.Scan(&r.id, &r.key, &r.sha, &r.size, &r.ownerType, &r.ownerID); err != nil {
			rows.Close()
			return RecoverResult{}, err
		}
		expired = append(expired, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return RecoverResult{}, err
	}

	var res RecoverResult
	for _, r := range expired {
		alive, err := s.ownerAlive(ctx, tx, r.ownerType, r.ownerID)
		if err != nil {
			return res, err
		}
		if alive {
			res.SkippedLive++
			continue
		}
		if err := s.recoverOne(ctx, tx, r.id, r.key, r.sha, r.ownerType, r.ownerID, &res); err != nil {
			return res, err
		}
	}

	orphans, err := s.recoverOrphans(ctx, tx)
	if err != nil {
		return res, err
	}
	res.Orphaned = orphans
	return res, nil
}

// ownerAlive 判断 temporary Owner 是否仍存活（仍有权完成发布）。
func (s *Store) ownerAlive(ctx context.Context, tx pgx.Tx, ownerType string, ownerID uuid.UUID) (bool, error) {
	switch ownerType {
	case OwnerTypeJob:
		var status string
		err := tx.QueryRow(ctx,
			`SELECT status FROM jobs WHERE id = $1`, ownerID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		switch status {
		case "queued":
			return true, nil // 任务尚未开始，仍可能完成发布
		case "running", "validating", "cancel_requested":
			var valid bool
			if err := tx.QueryRow(ctx,
				`SELECT lease_expires_at > now() FROM jobs WHERE id = $1`, ownerID).Scan(&valid); err != nil {
				return false, err
			}
			return valid, nil
		default:
			return false, nil
		}
	case OwnerTypeIdempotency:
		// 幂等 Owner 没有独立心跳：temporary_expires_at 即其租约，
		// 走到这里说明已过期，视为不存活。
		return false, nil
	default:
		return false, nil
	}
}

// recoverOne 处置单个过期 temporary 行。
func (s *Store) recoverOne(ctx context.Context, tx pgx.Tx, id uuid.UUID, tmpKey, sha, ownerType string, ownerID uuid.UUID, res *RecoverResult) error {
	fKey := finalKey(sha, id)
	fAbs := filepath.Join(s.dataDir, fKey)
	tmpAbs := filepath.Join(s.dataDir, tmpKey)

	// 最终文件存在且 hash 匹配 → 检查 Owner 提交证明。
	if _, err := os.Stat(fAbs); err == nil && sha != "" {
		actual, _, herr := hashFile(fAbs)
		if herr == nil && actual == sha {
			committed, err := s.ownerCommitted(ctx, tx, ownerType, ownerID)
			if err != nil {
				return err
			}
			if committed {
				tag, err := tx.Exec(ctx, `
					UPDATE file_objects
					SET storage_key = $2, state = 'ready',
					    temporary_owner_type = NULL, temporary_owner_id = NULL,
					    temporary_expires_at = NULL, updated_at = now()
					WHERE id = $1 AND state = 'temporary'`, id, fKey)
				if err != nil {
					return err
				}
				if tag.RowsAffected() == 1 {
					res.Completed++
					return nil
				}
			}
		}
	}

	// 无法证明发布完成：找得到的文件隔离，找不到标记 deleted。
	src := ""
	if _, err := os.Stat(fAbs); err == nil {
		src = fAbs
	} else if _, err := os.Stat(tmpAbs); err == nil {
		src = tmpAbs
	}
	if src == "" {
		if _, err := tx.Exec(ctx, `
			UPDATE file_objects
			SET state = 'deleted',
			    temporary_owner_type = NULL, temporary_owner_id = NULL,
			    temporary_expires_at = NULL, updated_at = now()
			WHERE id = $1`, id); err != nil {
			return err
		}
		res.Deleted++
		return nil
	}
	qKey := filepath.Join("quarantine", "maintenance-recovery", id.String())
	qAbs := filepath.Join(s.dataDir, qKey)
	if err := os.MkdirAll(filepath.Dir(qAbs), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, qAbs); err != nil {
		return fmt.Errorf("filestore: 隔离移动失败: %w", err)
	}
	syncDir(filepath.Dir(qAbs))
	if _, err := tx.Exec(ctx, `
		UPDATE file_objects
		SET storage_key = $2, state = 'quarantined',
		    temporary_owner_type = NULL, temporary_owner_id = NULL,
		    temporary_expires_at = NULL, updated_at = now()
		WHERE id = $1`, id, qKey); err != nil {
		return err
	}
	res.Quarantined++
	return nil
}

// ownerCommitted 返回 Owner 是否已提交（发布证明）：job succeeded 或
// 幂等记录 completed。
func (s *Store) ownerCommitted(ctx context.Context, tx pgx.Tx, ownerType string, ownerID uuid.UUID) (bool, error) {
	switch ownerType {
	case OwnerTypeJob:
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, ownerID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return status == "succeeded", err
	case OwnerTypeIdempotency:
		var state string
		err := tx.QueryRow(ctx, `SELECT state FROM idempotency_records WHERE id = $1`, ownerID).Scan(&state)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return state == "completed", err
	default:
		return false, nil
	}
}

// recoverOrphans 扫描 objects/ 下无数据库记录的孤立最终对象。
func (s *Store) recoverOrphans(ctx context.Context, tx pgx.Tx) (int, error) {
	objectsDir := filepath.Join(s.dataDir, "objects")
	count := 0
	err := filepath.Walk(objectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(s.dataDir, path)
		if err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM file_objects WHERE storage_key = $1)`, rel).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		qKey := filepath.Join("quarantine", "orphan", strings.ReplaceAll(rel, string(filepath.Separator), "_"))
		qAbs := filepath.Join(s.dataDir, qKey)
		if err := os.MkdirAll(filepath.Dir(qAbs), 0o755); err != nil {
			return err
		}
		if err := os.Rename(path, qAbs); err != nil {
			return fmt.Errorf("filestore: 孤立对象隔离失败: %w", err)
		}
		count++
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return count, err
	}
	syncDir(objectsDir)
	return count, nil
}
