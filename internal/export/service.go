// Package export 实现课时导出与文件下载（B3）：
//
//   - markdown 同步导出：把课时的 active 文档渲染为单个 Markdown 文件，
//     经 filestore 两阶段发布后返回 FileObject；
//   - zip 导出：事务内创建 lesson_export Job（202），实际打包由 B5 Worker
//     异步完成；
//   - 文件下载：仅 ready 状态的文件对象可下载，流式响应内容。
package export

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/document"
	"github.com/TSY244/ai-study-tutor/internal/filestore"
	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/job"
	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
)

// 导出的文档节：kind -> 节标题（契约固定三类，按此顺序渲染）。
var docSections = []struct {
	kind    string
	heading string
}{
	{"student_notes", "学生笔记"},
	{"teacher_outline", "教师教纲"},
	{"raw_text", "原文"},
}

// Service 编排课时导出与文件下载。事务边界（pool.Begin）在本层；
// SQL 在各模块 Repository / filestore.Store。
type Service struct {
	pool  *pgxpool.Pool
	files *filestore.Store
}

// NewService 创建导出 Service。
func NewService(pool *pgxpool.Pool, files *filestore.Store) *Service {
	return &Service{pool: pool, files: files}
}

// ExportMarkdown 同步导出课时为单个 Markdown 文件并发布为文件对象。
// 课时不存在返回 lesson.ErrNotFound（404）；课时没有任何 active 文档时
// 返回 404 not_found（detail「该课时还没有可导出的文档。」）。
func (s *Service) ExportMarkdown(ctx context.Context, lessonID uuid.UUID) (*filestore.Object, error) {
	l, err := (lesson.Repository{}).GetByID(ctx, s.pool, lessonID)
	if err != nil {
		return nil, err
	}
	content, found, err := s.renderMarkdown(ctx, lessonID, l.Title)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, httpx.NewProblemError(404, "not_found", "资源不存在",
			"该课时还没有可导出的文档。")
	}

	// Owner 决策：markdown 导出是一次性同步产物，请求内没有幂等记录或
	// Job 可作为 temporary Owner，因此合成一个一次性 Owner UUID。
	// Publish 在本请求内完整走完 tx1（temporary）+ tx2（ready 并清空
	// Owner 字段），不会留下悬挂的 temporary 行。
	ownerID := uuid.New()
	dir := s.files.TmpDirFor(ownerID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	staged := filepath.Join(dir, "export.md")
	if err := os.WriteFile(staged, []byte(content), 0o644); err != nil {
		return nil, err
	}
	obj, err := s.files.Publish(ctx, filestore.PublishInput{
		OwnerType:    filestore.OwnerTypeIdempotency,
		OwnerID:      ownerID,
		StagedPath:   staged,
		OriginalName: filepath.Base(l.Title + ".md"),
		MediaType:    "text/markdown",
	}, nil)
	if err != nil {
		return nil, err
	}
	// Publish 返回的 Object 不含 created_at，回读完整行供 DTO 使用。
	return s.files.GetByID(ctx, s.pool, obj.ID)
}

// renderMarkdown 渲染课时 Markdown：首行 "# {title}"，其后每类存在的
// active 文档一节，节间空行。found=false 表示三类文档都不存在。
func (s *Service) renderMarkdown(ctx context.Context, lessonID uuid.UUID, title string) (content string, found bool, err error) {
	var sb strings.Builder
	sb.WriteString("# " + title + "\n")
	docs := document.Repository{}
	for _, sec := range docSections {
		d, err := docs.GetActive(ctx, s.pool, lessonID, sec.kind)
		if errors.Is(err, document.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", false, err
		}
		found = true
		sb.WriteString("\n## " + sec.heading + "\n\n")
		sb.WriteString(strings.TrimRight(d.ContentMD, "\n"))
		sb.WriteString("\n")
	}
	return sb.String(), found, nil
}

// CreateZipExportJob 在单个事务内创建 lesson_export Job（aggregate 指向
// 课时，generation_family 为空）。维护模式下返回 gate.ErrMaintenanceMode
// （503）。打包与文件发布由 B5 Worker 实现。
func (s *Service) CreateZipExportJob(ctx context.Context, lessonID uuid.UUID) (*job.Job, error) {
	if _, err := (lesson.Repository{}).GetByID(ctx, s.pool, lessonID); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"lesson_id": lessonID.String(),
		"format":    "zip",
	})
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return nil, err
	}
	j := &job.Job{
		Kind:          job.KindLessonExport,
		AggregateType: "lesson",
		AggregateID:   lessonID,
		Payload:       payload,
	}
	if err := (&job.Repository{}).Create(ctx, tx, j); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return j, nil
}

// OpenDownload 校验文件对象可下载并打开内容流，调用方负责关闭返回的文件。
// 不存在或非 ready（temporary/quarantined/deleted）一律返回
// filestore.ErrNotFound（404，不泄露文件存在性细节）；DB 有 ready 行但
// 磁盘内容缺失时返回 404（detail「文件内容缺失」）。
func (s *Service) OpenDownload(ctx context.Context, fileID uuid.UUID) (*filestore.Object, *os.File, error) {
	o, err := s.files.GetByID(ctx, s.pool, fileID)
	if err != nil {
		return nil, nil, err
	}
	if o.State != filestore.StateReady {
		return nil, nil, filestore.ErrNotFound
	}
	abs, err := s.files.AbsPath(o.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, httpx.NewProblemError(404, "not_found", "资源不存在",
				"文件内容缺失。")
		}
		return nil, nil, err
	}
	return o, f, nil
}
