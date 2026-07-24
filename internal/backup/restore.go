package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/maintenance"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
)

// RestoreService restores one verified bundle into a fresh data volume and
// database. It is intentionally a one-shot, explicitly confirmed operation.
type RestoreService struct {
	Pool        *pgxpool.Pool
	DataDir     string
	BackupDir   string
	DatabaseURL string
	Confirmed   bool
	Runner      Runner
	Repo        Repository
}

// Restore verifies the selected backup before invoking tools, stages objects
// under APP_DATA_DIR, restores PostgreSQL, then atomically publishes objects
// and clears the maintenance state embedded in the database dump.
func (s RestoreService) Restore(ctx context.Context, name string) error {
	if s.Pool == nil || s.DataDir == "" || s.BackupDir == "" || s.DatabaseURL == "" {
		return errors.New("恢复配置不完整")
	}
	if !s.Confirmed {
		return errors.New("恢复需要显式确认")
	}
	if s.Runner == nil {
		s.Runner = ExecRunner{}
	}
	manifest, bundleDir, err := VerifyBundle(s.BackupDir, name)
	if err != nil {
		return err
	}
	backupID, err := uuid.Parse(manifest.BackupID)
	if err != nil {
		return errors.New("备份清单中的备份 ID 无效")
	}
	if err := ensureEmptyDirectory(s.DataDir); err != nil {
		return err
	}
	databaseName, err := postgresDatabaseName(s.DatabaseURL)
	if err != nil {
		return err
	}

	stage := filepath.Join(s.DataDir, ".restore-"+backupID.String())
	if err := os.Mkdir(stage, 0o700); err != nil {
		return errors.New("创建恢复暂存目录失败")
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stage)
		}
	}()

	pgEnv, cleanupPGPass, err := postgresEnvironment(s.DatabaseURL)
	if err != nil {
		return err
	}
	defer cleanupPGPass()
	if err := s.Runner.Run(ctx, "tar", []string{
		"--extract", "--zstd", "--no-same-owner", "--no-same-permissions",
		"--file=" + filepath.Join(bundleDir, FilesArchiveName),
		"--directory=" + stage,
	}, nil); err != nil {
		return errors.New("解压备份文件卷失败")
	}
	if _, err := os.Stat(filepath.Join(stage, "objects")); err != nil {
		return errors.New("备份文件卷缺少已发布对象目录")
	}
	if err := s.Runner.Run(ctx, "pg_restore", []string{
		"--single-transaction", "--clean", "--if-exists", "--no-owner", "--no-privileges", "--exit-on-error",
		"--dbname=" + databaseName,
		filepath.Join(bundleDir, DatabaseDumpName),
	}, pgEnv); err != nil {
		return errors.New("数据库恢复失败")
	}
	if err := os.Rename(filepath.Join(stage, "objects"), filepath.Join(s.DataDir, "objects")); err != nil {
		return errors.New("发布恢复文件失败")
	}
	published = true

	checksum, err := bundleChecksum(filepath.Join(bundleDir, ChecksumsName))
	if err != nil {
		return err
	}
	size, err := directorySize(bundleDir)
	if err != nil {
		return errors.New("统计恢复备份大小失败")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return errors.New("开始恢复收尾事务失败")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.LockBusinessExclusive(ctx, tx); err != nil {
		return errors.New("获取恢复业务锁失败")
	}
	if err := s.Repo.MarkRestored(ctx, tx, backupID, size, manifest, checksum); err != nil {
		return err
	}
	if err := (maintenance.Repository{}).ClearMaintenance(ctx, tx); err != nil {
		return errors.New("清理恢复维护状态失败")
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("提交恢复收尾事务失败")
	}
	return nil
}

func ensureEmptyDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.New("创建数据卷目录失败")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.New("读取数据卷目录失败")
	}
	if len(entries) != 0 {
		return errors.New("恢复只允许写入空数据卷")
	}
	return nil
}

func bundleChecksum(path string) (string, error) {
	sum, err := hashRegularFile(path)
	if err != nil {
		return "", errors.New("计算恢复校验清单失败")
	}
	return sum, nil
}
