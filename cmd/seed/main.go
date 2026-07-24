// Seed 写入前端联调演示数据（幂等，可重复执行）。
//
// 数据内容（合成，无真实用户数据）：
//   - 课程 demo-course「演示课程：计算机科学导论」；
//   - 课时 1「算法复杂度入门」：ready，含一条 ready 文本素材与已发布
//     raw_text 文档（source_job_id 为空，表示非任务生成内容）；
//   - 课时 2「二分查找」：processing，含一条 pending 文本素材和一个
//     queued 的 lesson_import Job——启动 Worker 后即可观察完整的
//     阶段事件流与文档发布（前端 SSE/Job 演示）。
//
// 用法：
//
//	make seed    （DATABASE_URL 等配置与 api/worker 相同）
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TSY244/ai-study-tutor/internal/course"
	"github.com/TSY244/ai-study-tutor/internal/document"
	"github.com/TSY244/ai-study-tutor/internal/job"
	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/material"
	"github.com/TSY244/ai-study-tutor/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

const (
	demoSlug      = "demo-course"
	demoJobScope  = "seed"
	demoJobKey    = "demo-lesson2-import-v1"
	demoLessonDoc = `# 算法复杂度入门

## 为什么要分析复杂度

同一段代码在不同规模的数据上表现差异巨大。复杂度分析让我们在不运行
程序的情况下预估运行时间的增长趋势。

## 大 O 记号

大 O 记号描述输入规模 n 趋向无穷时运行时间的上界：

- O(1)：常数时间，如数组下标访问。
- O(log n)：对数时间，如二分查找。
- O(n)：线性时间，如顺序遍历。
- O(n log n)：高效排序的下界，如归并排序。
- O(n^2)：双重循环，如冒泡排序。

## 分析方法

1. 找基本操作（比较、赋值）。
2. 统计基本操作执行次数关于 n 的函数。
3. 保留最高阶项并忽略常数系数。

练习：分析顺序查找最好、最坏和平均情况下的复杂度。`
	demoLesson2Text = `二分查找适用于有序数组。每次取中间元素与目标比较，
若相等则命中；若目标更小则在左半区间继续，否则在右半区间继续。
每轮比较把搜索区间缩小一半，因此时间复杂度为 O(log n)。
注意：数组必须先排序；插入删除频繁的场合应考虑其他结构。`
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed 失败：%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(config.ServiceSeed)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DB.URL)
	if err != nil {
		return err
	}
	defer pool.Close()

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM courses WHERE slug = $1)`, demoSlug).Scan(&exists); err != nil {
		return err
	}
	if exists {
		fmt.Printf("演示数据已存在（slug=%s），跳过。\n", demoSlug)
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	c := &course.Course{
		Slug:        demoSlug,
		Title:       "演示课程：计算机科学导论",
		Description: strptr("前端联调演示数据（seed 生成，可安全删除重建）。"),
	}
	if err := (course.Repository{}).Create(ctx, tx, c); err != nil {
		return err
	}

	// 课时 1：ready + 素材 ready + 已发布 raw_text 文档。
	l1 := &lesson.Lesson{CourseID: c.ID, Ordinal: 1, Title: "算法复杂度入门"}
	if err := (lesson.Repository{}).Create(ctx, tx, l1); err != nil {
		return err
	}
	docText := demoLessonDoc
	m1 := &material.Material{
		LessonID:   l1.ID,
		Kind:       material.KindText,
		InlineText: &docText,
		Metadata: mustJSON(map[string]any{
			"title":             "算法复杂度讲义",
			"parser_version":    "txt/v1",
			"normalize_version": "norm/v1",
			"chunk_version":     "chunk/v1",
		}),
	}
	if err := (material.Repository{}).Create(ctx, tx, m1); err != nil {
		return err
	}
	if err := (material.Repository{}).SetResult(ctx, tx, m1.ID, material.StatusReady, m1.Metadata); err != nil {
		return err
	}
	d := &document.Document{LessonID: l1.ID, Kind: document.KindRawText, ContentMD: demoLessonDoc}
	if err := (document.Repository{}).CreateVersion(ctx, tx, d); err != nil {
		return err
	}
	if err := (document.Repository{}).Publish(ctx, tx, d.ID); err != nil {
		return err
	}
	if err := (lesson.Repository{}).UpdateStatus(ctx, tx, l1.ID, "ready", l1.Version); err != nil {
		return err
	}

	// 课时 2：processing + pending 素材 + queued lesson_import Job。
	l2 := &lesson.Lesson{CourseID: c.ID, Ordinal: 2, Title: "二分查找"}
	if err := (lesson.Repository{}).Create(ctx, tx, l2); err != nil {
		return err
	}
	if err := (lesson.Repository{}).UpdateStatus(ctx, tx, l2.ID, "processing", l2.Version); err != nil {
		return err
	}
	text2 := demoLesson2Text
	m2 := &material.Material{
		LessonID:   l2.ID,
		Kind:       material.KindText,
		InlineText: &text2,
		Metadata:   mustJSON(map[string]any{"title": "二分查找笔记"}),
	}
	if err := (material.Repository{}).Create(ctx, tx, m2); err != nil {
		return err
	}
	family := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindLessonImport,
		AggregateType:    "lesson",
		AggregateID:      l2.ID,
		GenerationFamily: &family,
		Payload: mustJSON(map[string]any{
			"material_id":   m2.ID.String(),
			"lesson_id":     l2.ID.String(),
			"material_kind": material.KindText,
			"title":         "二分查找笔记",
		}),
		IdempotencyScope: strptr(demoJobScope),
		IdempotencyKey:   strptr(demoJobKey),
	}
	if err := (&job.Repository{}).Create(ctx, tx, j); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("演示数据已写入：\n")
	fmt.Printf("  课程   %s（%s）\n", c.Title, c.Slug)
	fmt.Printf("  课时 1 %s：ready（含已发布 raw_text 文档）\n", l1.Title)
	fmt.Printf("  课时 2 %s：processing（queued lesson_import Job %s）\n", l2.Title, j.ID)
	fmt.Printf("启动 Worker 后课时 2 将完成导入并转为 ready。\n")
	return nil
}

func strptr(s string) *string { return &s }

func mustJSON(v map[string]any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}
