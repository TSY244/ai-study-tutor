// 数据库迁移命令入口：应用/查看版本化迁移（platform/migrate，
// SQL 文件内嵌，迁移版本表 schema_migrations）。
//
// 用法：
//
//	migrate up       应用全部未执行的迁移，输出本次应用的版本
//	migrate status   显示每个迁移的应用状态
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/migrate"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "up" && os.Args[1] != "status") {
		fmt.Fprintln(os.Stderr, "用法：migrate up|status")
		os.Exit(2)
	}

	cfg, err := config.Load(config.ServiceMigrate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate 启动失败：%v\n", err)
		os.Exit(1)
	}

	pool, err := database.Connect(context.Background(), cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate 数据库配置无效：%v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch os.Args[1] {
	case "up":
		applied, err := migrate.Up(ctx, pool)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate up 失败：%v\n", err)
			os.Exit(1)
		}
		if len(applied) == 0 {
			fmt.Println("数据库已是最新，无需迁移")
		} else {
			fmt.Printf("已应用迁移：%v\n", applied)
		}
	case "status":
		statuses, err := migrate.Current(ctx, pool)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate status 失败：%v\n", err)
			os.Exit(1)
		}
		for _, s := range statuses {
			mark := "pending"
			if s.Applied {
				mark = "applied " + s.AppliedAt
			}
			fmt.Printf("%s_%s\t%s\n", s.Version, s.Name, mark)
		}
	}
}
