// Package filestore 实现文件对象的两阶段发布与恢复（03 §5.3）：
//
//	temporary row -> hash -> atomic rename -> ready/link
//
// 文件保存在 APP_DATA_DIR 下，数据库只保存相对 storage_key：
//
//	tmp/{owner_id}/{file_id}                        暂存（可原子重命名）
//	objects/{sha256 前两位}/{file_id}               已发布最终对象
//	quarantine/maintenance-recovery/{file_id}       恢复隔离
//	quarantine/orphan/{file_id}                     无数据库记录的孤立对象
//
// 业务实体只许引用 ready 对象（AssertReady / Link 在 tx2 中执行）。
package filestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
)

// 文件对象状态机：temporary -> ready | quarantined | deleted。
const (
	StateTemporary   = "temporary"
	StateReady       = "ready"
	StateQuarantined = "quarantined"
	StateDeleted     = "deleted"
)

// temporary Owner 类型。
const (
	OwnerTypeJob         = "job"
	OwnerTypeIdempotency = "idempotency"
)

var (
	// ErrNotReady：业务实体试图引用非 ready 状态的文件对象。
	ErrNotReady = errors.New("文件对象尚未发布，业务实体不能引用")
	// ErrNotFound：文件对象不存在。
	ErrNotFound = errors.New("文件对象不存在")
	// ErrPathEscape：存储键试图逃逸数据目录。
	ErrPathEscape = errors.New("存储键非法：路径逃逸数据目录")
	// ErrHashMismatch：最终文件内容与记录 hash 不一致。
	ErrHashMismatch = errors.New("文件内容校验失败")
)

// Object 是 file_objects 表的领域表示。
type Object struct {
	ID                 uuid.UUID
	StorageKey         string
	OriginalName       string
	MediaType          string
	SizeBytes          int64
	SHA256             string
	State              string
	TemporaryOwnerType *string
	TemporaryOwnerID   *uuid.UUID
	TemporaryExpiresAt *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// PublishInput 描述一次两阶段发布。
type PublishInput struct {
	OwnerType    string    // OwnerTypeJob / OwnerTypeIdempotency
	OwnerID      uuid.UUID // Job ID 或幂等记录 ID
	StagedPath   string    // 数据目录 tmp/ 内已写好的暂存文件（绝对路径）
	OriginalName string
	MediaType    string
}

// LinkFunc 在第二个短事务（tx2）中把 ready 对象关联到业务实体。
type LinkFunc func(ctx context.Context, tx pgx.Tx, objectID uuid.UUID) error

// Store 编排两阶段发布。pool 用于独立短事务；dataDir 是数据卷根。
type Store struct {
	pool         *pgxpool.Pool
	dataDir      string
	temporaryTTL time.Duration
}

// New 创建 Store。temporaryTTL 是 temporary 行的回收期限（默认 24h）。
func New(pool *pgxpool.Pool, dataDir string, temporaryTTL time.Duration) (*Store, error) {
	if pool == nil {
		return nil, errors.New("filestore: pool 不能为空")
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("filestore: 数据目录无效: %w", err)
	}
	if temporaryTTL <= 0 {
		temporaryTTL = 24 * time.Hour
	}
	for _, sub := range []string{"tmp", "objects", filepath.Join("quarantine", "maintenance-recovery"), filepath.Join("quarantine", "orphan")} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0o755); err != nil {
			return nil, fmt.Errorf("filestore: 创建数据子目录失败: %w", err)
		}
	}
	return &Store{pool: pool, dataDir: abs, temporaryTTL: temporaryTTL}, nil
}

// DataDir 返回数据卷根（绝对路径）。
func (s *Store) DataDir() string { return s.dataDir }

// TmpDirFor 返回指定 Owner 的暂存目录（调用方先把上传/产物写入这里）。
func (s *Store) TmpDirFor(ownerID uuid.UUID) string {
	return filepath.Join(s.dataDir, "tmp", ownerID.String())
}

// AbsPath 把相对 storage_key 解析为绝对路径，拒绝路径逃逸。
func (s *Store) AbsPath(storageKey string) (string, error) {
	clean := filepath.Clean(storageKey)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	return filepath.Join(s.dataDir, clean), nil
}

func finalKey(shaHex string, id uuid.UUID) string {
	prefix := shaHex
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return filepath.Join("objects", prefix, id.String())
}

// Publish 执行两阶段发布：
//
//  1. 校验 StagedPath 位于数据目录 tmp/ 内，流式计算 SHA-256/大小；
//  2. tx1（Business Gate shared lock）：插入 file_objects(state=temporary)；
//  3. 原子重命名到 objects/{prefix}/{file_id} 并 fsync；
//  4. tx2：核对 id/hash/size，state 置 ready，执行 link 关联业务实体。
//
// 第 4 步提交前进程崩溃不会留下半成品：temporary 行由 Recover 按
// hash/最终路径/Owner 证明完成、隔离或删除（03 §5.3 第 6 步）。
func (s *Store) Publish(ctx context.Context, in PublishInput, link LinkFunc) (*Object, error) {
	if in.OwnerType != OwnerTypeJob && in.OwnerType != OwnerTypeIdempotency {
		return nil, fmt.Errorf("filestore: 非法 Owner 类型 %q", in.OwnerType)
	}
	tmpDir := filepath.Join(s.dataDir, "tmp")
	stagedClean := filepath.Clean(in.StagedPath)
	if !strings.HasPrefix(stagedClean, tmpDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("%w: 暂存文件必须位于 %s 内", ErrPathEscape, tmpDir)
	}
	shaHex, size, err := hashFile(stagedClean)
	if err != nil {
		return nil, err
	}

	id := uuid.New()
	tmpKey := filepath.Join("tmp", in.OwnerID.String(), id.String())
	tmpAbs := filepath.Join(s.dataDir, tmpKey)
	if err := os.MkdirAll(filepath.Dir(tmpAbs), 0o755); err != nil {
		return nil, err
	}
	if stagedClean != tmpAbs {
		if err := os.Rename(stagedClean, tmpAbs); err != nil {
			return nil, fmt.Errorf("filestore: 归一暂存路径失败: %w", err)
		}
	}

	// tx1：插入 temporary 行。
	tx1, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx1.Rollback(ctx) }()
	if err := gate.LockBusinessShared(ctx, tx1); err != nil {
		return nil, err
	}
	if _, err := tx1.Exec(ctx, `
		INSERT INTO file_objects (id, storage_key, original_name, media_type,
			size_bytes, sha256, state,
			temporary_owner_type, temporary_owner_id, temporary_expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,'temporary',$7,$8, now() + $9::interval)`,
		id, tmpKey, in.OriginalName, in.MediaType, size, shaHex,
		in.OwnerType, in.OwnerID, s.temporaryTTL.String()); err != nil {
		return nil, fmt.Errorf("filestore: 创建 temporary 行失败: %w", err)
	}
	if err := tx1.Commit(ctx); err != nil {
		return nil, err
	}

	// 原子重命名到最终路径。
	fKey := finalKey(shaHex, id)
	fAbs := filepath.Join(s.dataDir, fKey)
	if err := os.MkdirAll(filepath.Dir(fAbs), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpAbs, fAbs); err != nil {
		return nil, fmt.Errorf("filestore: 原子重命名失败: %w", err)
	}
	syncDir(filepath.Dir(fAbs))
	syncDir(filepath.Join(s.dataDir, "tmp"))

	// tx2：核对并置 ready，关联业务实体。
	tx2, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx2.Rollback(ctx) }()
	if err := gate.LockBusinessShared(ctx, tx2); err != nil {
		return nil, err
	}
	tag, err := tx2.Exec(ctx, `
		UPDATE file_objects
		SET storage_key = $2, state = 'ready', size_bytes = $3, sha256 = $4,
		    temporary_owner_type = NULL, temporary_owner_id = NULL,
		    temporary_expires_at = NULL, updated_at = now()
		WHERE id = $1 AND state = 'temporary' AND sha256 = $4 AND size_bytes = $3`,
		id, fKey, size, shaHex)
	if err != nil {
		return nil, fmt.Errorf("filestore: 发布核对失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("filestore: 发布核对未命中（可能被并发回收）: %w", ErrNotFound)
	}
	if link != nil {
		if err := link(ctx, tx2, id); err != nil {
			return nil, fmt.Errorf("filestore: 关联业务实体失败: %w", err)
		}
	}
	if err := tx2.Commit(ctx); err != nil {
		return nil, err
	}
	return &Object{
		ID: id, StorageKey: fKey, OriginalName: in.OriginalName, MediaType: in.MediaType,
		SizeBytes: size, SHA256: shaHex, State: StateReady,
	}, nil
}

// AssertReady 校验文件对象处于 ready 状态；业务实体 Repository 在
// 建立引用时必须调用，防止引用 temporary/quarantined 对象。
func (s *Store) AssertReady(ctx context.Context, q database.Querier, id uuid.UUID) error {
	var state string
	err := q.QueryRow(ctx, `SELECT state FROM file_objects WHERE id = $1`, id).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if state != StateReady {
		return fmt.Errorf("%w（当前状态 %s）", ErrNotReady, state)
	}
	return nil
}

// GetByID 读取文件对象。
func (s *Store) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Object, error) {
	var o Object
	err := q.QueryRow(ctx, `
		SELECT id, storage_key, COALESCE(original_name,''), COALESCE(media_type,''),
		       COALESCE(size_bytes,0), COALESCE(sha256,''), state,
		       temporary_owner_type, temporary_owner_id, temporary_expires_at,
		       created_at, updated_at
		FROM file_objects WHERE id = $1`, id).Scan(
		&o.ID, &o.StorageKey, &o.OriginalName, &o.MediaType,
		&o.SizeBytes, &o.SHA256, &o.State,
		&o.TemporaryOwnerType, &o.TemporaryOwnerID, &o.TemporaryExpiresAt,
		&o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("filestore: 打开文件失败: %w", err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("filestore: 读取文件失败: %w", err)
	}
	if err := f.Sync(); err != nil {
		return "", 0, fmt.Errorf("filestore: fsync 失败: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer func() { _ = d.Close() }()
	_ = d.Sync()
}
