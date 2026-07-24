// backup creates a verified offline backup bundle. It is only run by the
// Docker Compose tools profile and is never exposed through the HTTP API.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/backup"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "create" {
		fmt.Fprintln(os.Stderr, "用法：backup create")
		os.Exit(2)
	}
	cfg, err := config.Load(config.ServiceBackup)
	if err != nil {
		fmt.Fprintln(os.Stderr, "backup 配置无效")
		os.Exit(2)
	}
	pool, err := database.Connect(context.Background(), cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "backup 数据库配置无效")
		os.Exit(2)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	result, err := (backup.Service{
		Pool:         pool,
		DataDir:      cfg.DataDir,
		BackupDir:    cfg.BackupDir,
		DatabaseURL:  cfg.DB.URL,
		AppVersion:   cfg.Version,
		Lease:        cfg.Maintenance.LeaseDuration,
		Heartbeat:    cfg.Maintenance.HeartbeatInterval,
		DrainTimeout: 2 * time.Minute,
		Runner:       backup.ExecRunner{},
	}).Create(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "backup 失败；请查看 Tools 容器日志并运行 doctor")
		os.Exit(1)
	}
	fmt.Printf("backup ready: %s (%s)\n", result.Name, result.ID)
}
