// HTTP 层：课时导出与文件下载的路由绑定与响应映射。
// 业务编排全部在 Service；本文件只做参数解析、DTO 映射与 Problem 输出。
package export

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// Handler 绑定导出/下载端点。
type Handler struct {
	svc *Service
}

// NewHandler 创建导出 Handler。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册路由。POST exports 挂 jsonLimit（小 JSON 请求体）；
// GET download 是流式下载，不挂 BodyLimit。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.POST("/lessons/:lesson_id/exports", jsonLimit, h.createLessonExport)
	rg.GET("/files/:file_id/download", h.downloadFile)
}

// createExportRequest 是 ExportCreate 契约（additionalProperties:false）。
type createExportRequest struct {
	Format string `json:"format"`
}

// FileDTO 是契约 FileObject（markdown 同步导出的 200 响应）。
type FileDTO struct {
	ID          uuid.UUID `json:"id"`
	DisplayName *string   `json:"display_name"`
	MediaType   string    `json:"media_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	DownloadURL string    `json:"download_url"`
	CreatedAt   string    `json:"created_at"`
}

// fileDTO 映射文件对象到契约 DTO：display_name 输出 original_name，
// download_url 指向本模块的下载端点。
func fileDTO(o *filestore.Object) FileDTO {
	name := o.OriginalName
	return FileDTO{
		ID:          o.ID,
		DisplayName: &name,
		MediaType:   o.MediaType,
		SizeBytes:   o.SizeBytes,
		SHA256:      o.SHA256,
		DownloadURL: "/api/v1/files/" + o.ID.String() + "/download",
		CreatedAt:   httpx.Time(o.CreatedAt),
	}
}

// createLessonExport 处理 POST /api/v1/lessons/{lesson_id}/exports：
// format=markdown 同步导出 200 FileObject；format=zip 创建后台任务 202。
func (h *Handler) createLessonExport(c *gin.Context) {
	lessonID, err := uuid.Parse(c.Param("lesson_id"))
	if err != nil {
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
		return
	}
	var req createExportRequest
	if !httpx.BindJSON(c, &req) {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "请求体不是合法 JSON 或包含未声明字段。")
		return
	}
	switch req.Format {
	case "markdown":
		obj, err := h.svc.ExportMarkdown(c.Request.Context(), lessonID)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, fileDTO(obj))
	case "zip":
		j, err := h.svc.CreateZipExportJob(c.Request.Context(), lessonID)
		if err != nil {
			writeError(c, err)
			return
		}
		c.Header("Location", "/api/v1/jobs/"+j.ID.String())
		c.JSON(http.StatusAccepted, gin.H{"job": job.ToDTO(j, nil)})
	default:
		httpx.WriteValidation(c, []httpserver.FieldError{
			httpx.FieldErr("format", "invalid_enum", "format 必须是 markdown 或 zip。"),
		})
	}
}

// writeError 把 service 错误映射为 Problem；lesson.ErrNotFound 由本包
// 自行映射（httpx 不能反向依赖 lesson 包），形状与 httpx.WriteError
// 的 404 一致。
func writeError(c *gin.Context, err error) {
	if errors.Is(err, lesson.ErrNotFound) {
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
		return
	}
	httpx.WriteError(c, err)
}

// downloadFile 处理 GET /api/v1/files/{file_id}/download：流式返回
// ready 文件对象的内容；非 ready/不存在一律 404，storage_key 不外泄。
func (h *Handler) downloadFile(c *gin.Context) {
	fileID, err := uuid.Parse(c.Param("file_id"))
	if err != nil {
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
		return
	}
	obj, f, err := h.svc.OpenDownload(c.Request.Context(), fileID)
	if err != nil {
		writeError(c, err)
		return
	}
	defer func() { _ = f.Close() }()

	mediaType := obj.MediaType
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	name := obj.OriginalName
	if name == "" {
		name = "download"
	} else {
		name = filepath.Base(name)
	}
	c.Header("Content-Type", mediaType)
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.QueryEscape(name))
	c.Header("Content-Length", strconv.FormatInt(obj.SizeBytes, 10))
	c.Status(http.StatusOK)
	// 写出一半失败无法改状态码，连接断开即终止，无需额外处理。
	_, _ = io.Copy(c.Writer, f)
}
