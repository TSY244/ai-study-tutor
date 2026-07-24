// seed_integration_test.go 用真实嵌入式 PostgreSQL 验证 seed：
// 首次写入演示数据，重复执行为幂等跳过。
package main

import (
	"context"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

func TestSeedIntegration(t *testing.T) {
	pool := databasetest.New(t)
	t.Setenv("DATABASE_URL", pool.Config().ConnString())

	if err := run(); err != nil {
		t.Fatalf("首次 seed 失败: %v", err)
	}

	ctx := context.Background()
	var (
		lessons    int
		readyDocs  int
		materials  int
		queuedJobs int
	)
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM lessons l JOIN courses c ON c.id = l.course_id WHERE c.slug = 'demo-course'`).Scan(&lessons); err != nil {
		t.Fatal(err)
	}
	if lessons != 2 {
		t.Fatalf("应有 2 个课时: %d", lessons)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM documents d
		JOIN lessons l ON l.id = d.lesson_id
		JOIN courses c ON c.id = l.course_id
		WHERE c.slug = 'demo-course' AND d.kind = 'raw_text' AND d.is_active`).Scan(&readyDocs); err != nil {
		t.Fatal(err)
	}
	if readyDocs != 1 {
		t.Fatalf("应有 1 个 active raw_text 文档: %d", readyDocs)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM materials m
		JOIN lessons l ON l.id = m.lesson_id
		JOIN courses c ON c.id = l.course_id
		WHERE c.slug = 'demo-course'`).Scan(&materials); err != nil {
		t.Fatal(err)
	}
	if materials != 2 {
		t.Fatalf("应有 2 条素材: %d", materials)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM jobs
		WHERE idempotency_scope = 'seed' AND idempotency_key = 'demo-lesson2-import-v1'
		  AND kind = 'lesson_import' AND status = 'queued'`).Scan(&queuedJobs); err != nil {
		t.Fatal(err)
	}
	if queuedJobs != 1 {
		t.Fatalf("应有 1 个 queued 导入任务: %d", queuedJobs)
	}

	// 幂等：重复执行不报错、不复制数据。
	if err := run(); err != nil {
		t.Fatalf("重复 seed 失败: %v", err)
	}
	var courses int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM courses WHERE slug = 'demo-course'`).Scan(&courses); err != nil {
		t.Fatal(err)
	}
	if courses != 1 {
		t.Fatalf("重复执行不应复制课程: %d", courses)
	}
}
