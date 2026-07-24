// restore restores a verified bundle only into an empty local data volume.
// It intentionally requires an explicit environment confirmation.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/TSY244/ai-study-tutor/internal/backup"
	"github.com/TSY244/ai-study-tutor/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "用法：restore <backup-name>（需 RESTORE_CONFIRM=RESTORE）")
		os.Exit(2)
	}
	cfg, err := config.Load(config.ServiceRestore)
	if err != nil {
		fmt.Fprintln(os.Stderr, "restore 配置无效")
		os.Exit(2)
	}
	pool, err := database.Connect(context.Background(), cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "restore 数据库配置无效")
		os.Exit(2)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	err = (backup.RestoreService{
		Pool:        pool,
		DataDir:     cfg.DataDir,
		BackupDir:   cfg.BackupDir,
		DatabaseURL: cfg.DB.URL,
		Confirmed:   os.Getenv("RESTORE_CONFIRM") == "RESTORE",
		Runner:      backup.ExecRunner{},
	}).Restore(ctx, os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "restore 失败；数据卷或数据库未发布为成功状态，请查看 Tools 容器日志")
		os.Exit(1)
	}
	fmt.Println("restore completed")
}
