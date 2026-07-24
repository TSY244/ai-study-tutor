package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/maintenance"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
)

// Runner is the narrow adapter for tools that only exist in the one-shot
// Tools image. Callers must not put secrets in args or persist command output.
type Runner interface {
	Run(ctx context.Context, name string, args []string, env []string) error
}

// ExecRunner invokes a fixed executable with an argument array. Output is
// discarded deliberately: pg_dump/pg_restore diagnostics can include details
// unsuitable for structured application logs.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args []string, env []string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return errors.New("外部备份工具执行失败")
	}
	return nil
}

// Service creates a portable backup while holding the existing maintenance
// Owner Lock and Business Gate protocol.
type Service struct {
	Pool        *pgxpool.Pool
	DataDir     string
	BackupDir   string
	DatabaseURL string
	AppVersion  string

	Lease        time.Duration
	Heartbeat    time.Duration
	DrainTimeout time.Duration
	PollInterval time.Duration
	Runner       Runner
	Now          func() time.Time
	Repo         Repository
}

type Result struct {
	ID       uuid.UUID
	Name     string
	Manifest Manifest
}

// Create publishes a complete backup directory only after pg_dump, file
// archive, manifest and checksums all succeeded within a quiesced window.
func (s Service) Create(ctx context.Context) (result Result, err error) {
	if err := s.validate(); err != nil {
		return Result{}, err
	}
	backupID := uuid.New()
	owner := "backup-" + backupID.String()
	name := backupName(s.now())

	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("获取备份维护连接失败")
	}
	defer conn.Release()
	locked, err := gate.TryMaintenanceOwner(ctx, conn)
	if err != nil {
		return Result{}, fmt.Errorf("获取备份维护锁失败")
	}
	if !locked {
		return Result{}, errors.New("已有备份或维护操作正在进行")
	}
	defer func() { _ = gate.UnlockMaintenanceOwner(context.Background(), conn) }()

	if err := s.enterMaintenance(ctx, conn, backupID, owner, name); err != nil {
		return Result{}, err
	}
	managed := true
	succeeded := false
	defer func() {
		if managed && !succeeded {
			s.finishFailure(backupID)
		}
	}()

	stopHeartbeat := s.startHeartbeat(owner)
	defer stopHeartbeat()
	if err := s.waitForDrain(ctx); err != nil {
		return Result{}, err
	}

	manifest, err := s.createQuiescedBundle(ctx, conn, backupID, name)
	if err != nil {
		return Result{}, err
	}
	succeeded = true
	return Result{ID: backupID, Name: name, Manifest: manifest}, nil
}

func (s *Service) validate() error {
	if s.Pool == nil || s.DataDir == "" || s.BackupDir == "" || s.DatabaseURL == "" {
		return errors.New("备份配置不完整")
	}
	if s.Runner == nil {
		s.Runner = ExecRunner{}
	}
	if s.Lease <= 0 {
		s.Lease = 45 * time.Second
	}
	if s.Heartbeat <= 0 {
		s.Heartbeat = 10 * time.Second
	}
	if s.Heartbeat >= s.Lease {
		return errors.New("备份心跳必须短于维护租约")
	}
	if s.DrainTimeout <= 0 {
		s.DrainTimeout = 2 * time.Minute
	}
	if s.PollInterval <= 0 {
		s.PollInterval = time.Second
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.AppVersion == "" {
		s.AppVersion = "unknown"
	}
	return nil
}

func (s Service) now() time.Time {
	if s.Now == nil {
		return time.Now().UTC()
	}
	return s.Now().UTC()
}

func backupName(now time.Time) string {
	return bundleNamePrefix + now.UTC().Format("20060102T150405Z")
}

func (s Service) enterMaintenance(ctx context.Context, conn *pgxpool.Conn, id uuid.UUID, owner, name string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.New("开始备份维护事务失败")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.LockBusinessExclusive(ctx, tx); err != nil {
		return errors.New("获取备份业务锁失败")
	}
	state := maintenance.Repository{}
	if err := state.EnterMaintenance(ctx, tx, owner, "backup", s.Lease); err != nil {
		return errors.New("系统已在维护模式")
	}
	if err := s.Repo.Create(ctx, tx, id, owner, s.AppVersion, name); err != nil {
		return errors.New("创建备份审计记录失败")
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("提交备份维护状态失败")
	}
	return nil
}

func (s Service) startHeartbeat(owner string) func() {
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(s.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				heartbeatCtx, heartbeatCancel := context.WithTimeout(ctx, s.Heartbeat)
				_ = (maintenance.Repository{}).Heartbeat(heartbeatCtx, s.Pool, owner, s.Lease)
				heartbeatCancel()
			}
		}
	}()
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

func (s Service) waitForDrain(ctx context.Context) error {
	deadline := time.NewTimer(s.DrainTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(s.PollInterval)
	defer ticker.Stop()
	for {
		activeJobs, temporaryFiles, err := s.Repo.DrainState(ctx, s.Pool)
		if err != nil {
			return errors.New("检查备份排空状态失败")
		}
		if activeJobs == 0 && temporaryFiles == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("备份等待活动任务和临时文件超时")
		case <-ticker.C:
		}
	}
}

func (s Service) createQuiescedBundle(ctx context.Context, conn *pgxpool.Conn, id uuid.UUID, name string) (manifest Manifest, err error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return Manifest{}, errors.New("开始备份静默事务失败")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.LockBusinessExclusive(ctx, tx); err != nil {
		return Manifest{}, errors.New("获取备份静默锁失败")
	}
	activeJobs, temporaryFiles, err := s.Repo.DrainState(ctx, tx)
	if err != nil || activeJobs != 0 || temporaryFiles != 0 {
		return Manifest{}, errors.New("备份静默窗口前仍有活动数据")
	}
	if err := (maintenance.Repository{}).SetPhase(ctx, tx, maintenance.PhaseQuiesced); err != nil {
		return Manifest{}, errors.New("进入备份静默窗口失败")
	}
	dbVersion, err := s.Repo.DatabaseVersion(ctx, tx)
	if err != nil {
		return Manifest{}, errors.New("读取数据库版本失败")
	}

	if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
		return Manifest{}, errors.New("创建备份目录失败")
	}
	stage := filepath.Join(s.BackupDir, ".tmp-"+id.String())
	final := filepath.Join(s.BackupDir, name)
	if err := os.Mkdir(stage, 0o700); err != nil {
		return Manifest{}, errors.New("创建备份暂存目录失败")
	}
	published := false
	defer func() {
		if err != nil {
			_ = os.RemoveAll(stage)
			if published {
				_ = os.RemoveAll(final)
			}
		}
	}()

	pgEnv, cleanupPGPass, err := postgresEnvironment(s.DatabaseURL)
	if err != nil {
		return Manifest{}, err
	}
	defer cleanupPGPass()
	dumpPath := filepath.Join(stage, DatabaseDumpName)
	if err := s.Runner.Run(ctx, "pg_dump", []string{
		"--format=custom", "--no-owner", "--no-privileges", "--file=" + dumpPath,
	}, pgEnv); err != nil {
		return Manifest{}, errors.New("数据库导出失败")
	}
	archivePath := filepath.Join(stage, FilesArchiveName)
	if err := s.Runner.Run(ctx, "tar", []string{
		"--create", "--zstd", "--file=" + archivePath,
		"--directory=" + s.DataDir, "objects",
	}, nil); err != nil {
		return Manifest{}, errors.New("文件卷归档失败")
	}
	fileCount, fileBytes, err := publishedObjectStats(filepath.Join(s.DataDir, "objects"))
	if err != nil {
		return Manifest{}, errors.New("统计已发布文件失败")
	}
	manifest = Manifest{
		FormatVersion:   1,
		BackupID:        id.String(),
		AppVersion:      s.AppVersion,
		DatabaseVersion: dbVersion,
		CreatedAt:       s.now(),
		FileCount:       fileCount,
		FileBytes:       fileBytes,
		ActiveJobs:      activeJobs,
	}
	checksum, err := WriteBundleManifest(stage, manifest)
	if err != nil {
		return Manifest{}, err
	}
	size, err := directorySize(stage)
	if err != nil {
		return Manifest{}, errors.New("统计备份大小失败")
	}
	if err := os.Rename(stage, final); err != nil {
		return Manifest{}, errors.New("发布备份目录失败")
	}
	published = true
	if err := s.Repo.MarkReady(ctx, tx, id, dbVersion, size, manifest, checksum); err != nil {
		return Manifest{}, err
	}
	if err := (maintenance.Repository{}).ClearMaintenance(ctx, tx); err != nil {
		return Manifest{}, errors.New("清理备份维护状态失败")
	}
	if err := tx.Commit(ctx); err != nil {
		return Manifest{}, errors.New("提交备份发布事务失败")
	}
	return manifest, nil
}

func (s Service) finishFailure(id uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if gate.LockBusinessExclusive(ctx, tx) != nil {
		return
	}
	if s.Repo.MarkFailed(ctx, tx, id) != nil {
		return
	}
	if (maintenance.Repository{}).ClearMaintenance(ctx, tx) != nil {
		return
	}
	_ = tx.Commit(ctx)
}

func postgresEnvironment(dsn string) ([]string, func(), error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, nil, errors.New("数据库连接配置无效")
	}
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	pass, err := os.CreateTemp("", "ai-study-tutor-pgpass-")
	if err != nil {
		return nil, nil, errors.New("创建 PostgreSQL 临时凭证失败")
	}
	path := pass.Name()
	cleanup := func() { _ = os.Remove(path) }
	if err := pass.Chmod(0o600); err != nil {
		_ = pass.Close()
		cleanup()
		return nil, nil, errors.New("保护 PostgreSQL 临时凭证失败")
	}
	line := strings.Join([]string{
		pgpassEscape(host), fmt.Sprintf("%d", port), pgpassEscape(cfg.Database), pgpassEscape(cfg.User), pgpassEscape(cfg.Password),
	}, ":") + "\n"
	if _, err := pass.WriteString(line); err != nil {
		_ = pass.Close()
		cleanup()
		return nil, nil, errors.New("写入 PostgreSQL 临时凭证失败")
	}
	if err := pass.Close(); err != nil {
		cleanup()
		return nil, nil, errors.New("关闭 PostgreSQL 临时凭证失败")
	}
	return []string{
		"PGHOST=" + host,
		fmt.Sprintf("PGPORT=%d", port),
		"PGDATABASE=" + cfg.Database,
		"PGUSER=" + cfg.User,
		"PGPASSFILE=" + path,
	}, cleanup, nil
}

func postgresDatabaseName(dsn string) (string, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil || cfg.Database == "" {
		return "", errors.New("数据库连接配置无效")
	}
	return cfg.Database, nil
}

func pgpassEscape(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", ":", "\\:")
	return replacer.Replace(value)
}

func publishedObjectStats(root string) (count, bytes int64, err error) {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	} else if err != nil {
		return 0, 0, err
	}
	err = filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			count++
			bytes += info.Size()
		}
		return nil
	})
	return count, bytes, err
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}
