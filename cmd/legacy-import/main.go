// legacy-import migrates the archived Python data tree through a one-way,
// audited process. scan is source/database read-free; import is the only mode
// which connects to PostgreSQL.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/TSY244/ai-study-tutor/internal/legacyimport"
	"github.com/TSY244/ai-study-tutor/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "scan" && args[0] != "import") {
		fmt.Fprintln(stderr, "用法：legacy-import scan|import --source <dir> --report <file> [--resolution <file>] [--dry-run] [--fail-on-warning]")
		return 2
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	source := flags.String("source", "/legacy", "legacy source root")
	reportPath := flags.String("report", "", "JSON report path")
	resolutionPath := flags.String("resolution", "", "JSON resolution path")
	dryRun := flags.Bool("dry-run", false, "scan and validate only")
	failOnWarning := flags.Bool("fail-on-warning", false, "non-completed report exits non-zero")
	if err := flags.Parse(args[1:]); err != nil || *reportPath == "" {
		if err == nil {
			fmt.Fprintln(stderr, "--report 不能为空")
		}
		return 2
	}

	manifest, scanErr := legacyimport.Scan(ctx, *source)
	if command == "scan" || *dryRun {
		if manifest != nil {
			if err := writeJSON(*reportPath, manifest); err != nil {
				fmt.Fprintf(stderr, "写入报告失败：%v\n", err)
				return 1
			}
		}
		if scanErr != nil {
			fmt.Fprintf(stderr, "扫描失败：%v\n", scanErr)
			return 1
		}
		if *failOnWarning && len(manifest.Findings) > 0 {
			fmt.Fprintln(stderr, "扫描发现待处理项；--fail-on-warning 阻止继续")
			return 1
		}
		if *dryRun {
			fmt.Fprintf(stdout, "dry-run scan completed: %s\n", *reportPath)
		} else {
			fmt.Fprintf(stdout, "scan completed: %s\n", *reportPath)
		}
		return 0
	}
	if scanErr != nil {
		if manifest != nil {
			_ = writeJSON(*reportPath, manifest)
		}
		fmt.Fprintf(stderr, "扫描失败：%v\n", scanErr)
		return 1
	}

	resolution, err := readResolution(*resolutionPath)
	if err != nil {
		fmt.Fprintf(stderr, "读取 resolution 失败：%v\n", err)
		return 2
	}
	cfg, err := config.Load(config.ServiceLegacyImport)
	if err != nil {
		fmt.Fprintln(stderr, "legacy-import 数据库配置无效")
		return 1
	}
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pool, err := database.Connect(connectCtx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(stderr, "legacy-import 无法连接数据库")
		return 1
	}
	defer pool.Close()

	result, err := legacyimport.NewService(pool).Import(ctx, *source, legacyimport.ImportOptions{
		Resolution: resolution, FailOnWarning: *failOnWarning,
	})
	if result != nil {
		if writeErr := writeJSON(*reportPath, result); writeErr != nil {
			fmt.Fprintf(stderr, "写入报告失败：%v\n", writeErr)
			return 1
		}
	}
	if err != nil {
		if errors.Is(err, legacyimport.ErrNeedsReview) {
			fmt.Fprintf(stderr, "导入需要人工确认：%s\n", *reportPath)
			return 1
		}
		fmt.Fprintf(stderr, "导入失败：%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "import %s: %s\n", result.Status, *reportPath)
	return 0
}

func readResolution(path string) (*legacyimport.Resolution, error) {
	if path == "" {
		return nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var resolution legacyimport.Resolution
	if err := json.Unmarshal(body, &resolution); err != nil {
		return nil, err
	}
	return &resolution, nil
}

// writeJSON uses the report directory as the staging location, then atomically
// renames only a complete report into place. It never writes anywhere in source.
func writeJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".legacy-import-report-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
