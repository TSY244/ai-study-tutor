package material

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// 路由级请求体上限默认值（装配层可经构造参数覆盖）。
const (
	defaultUploadLimitBytes = int64(2) << 30 // 2 GiB
	defaultTextLimitBytes   = int64(4) << 20 // 约 4.2 MiB
	defaultJSONLimitBytes   = int64(1) << 20
)

// 上传 MIME 白名单（net/http.DetectContentType 输出去参数后精确匹配）。
// application/zip 是 DOCX 候选，由 B5 解析阶段再确认；text/plain 覆盖
// txt/md/srt/vtt。audio/wave 是 Go 1.25 对 WAV 的嗅探输出（契约白名单
// 中的 audio/x-wav 在更早工具链上使用），两者都接受。
var allowedMIME = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/bmp":       true,
	"image/tiff":      true,
	"image/webp":      true,
	"application/zip": true,
	"text/plain":      true,
	"text/html":       true,
	"audio/mpeg":      true,
	"audio/aac":       true,
	"audio/flac":      true,
	"audio/ogg":       true,
	"application/ogg": true,
	"audio/mp4":       true,
	"audio/x-wav":     true,
	"audio/wave":      true,
	"video/mp4":       true,
	"video/webm":      true,
}

// 上传表单可选字段的合法枚举（契约 transcription_mode）。
const (
	TranscriptionSubtitleOnly = "subtitle_only"
	TranscriptionWhisperLocal = "whisper_local"
)

// Handler 只做 HTTP 绑定与响应映射；业务编排在 Service。
type Handler struct {
	svc         *Service
	idem        *httpx.IdemExecutor
	uploadLimit int64
	textLimit   int64
	jsonLimit   int64
}

// NewHandler 创建素材 Handler。三个请求体上限为 0 时使用默认值
// （upload 2GiB / text 约 4.2MiB / json 1MiB）。
func NewHandler(svc *Service, idem *httpx.IdemExecutor, uploadLimitBytes, textLimitBytes, jsonLimitBytes int64) *Handler {
	if uploadLimitBytes <= 0 {
		uploadLimitBytes = defaultUploadLimitBytes
	}
	if textLimitBytes <= 0 {
		textLimitBytes = defaultTextLimitBytes
	}
	if jsonLimitBytes <= 0 {
		jsonLimitBytes = defaultJSONLimitBytes
	}
	return &Handler{
		svc: svc, idem: idem,
		uploadLimit: uploadLimitBytes, textLimit: textLimitBytes, jsonLimit: jsonLimitBytes,
	}
}

// RegisterRoutes 注册素材路由；BodyLimit 按路由挂载：
// upload → 413 upload_too_large；text → textLimitBytes（400 bad_request）；
// url/list/delete → jsonLimitBytes。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/lessons/:lesson_id/materials")
	g.POST("/url", httpserver.BodyLimit(h.jsonLimit, http.StatusBadRequest, "bad_request"), h.createURL)
	g.POST("/text", httpserver.BodyLimit(h.textLimit, http.StatusBadRequest, "bad_request"), h.createText)
	g.POST("/upload", httpserver.BodyLimit(h.uploadLimit, http.StatusRequestEntityTooLarge, "upload_too_large"), h.upload)
	g.GET("", httpserver.BodyLimit(h.jsonLimit, http.StatusBadRequest, "bad_request"), h.list)
	rg.GET("/materials/:material_id", h.getMaterial)
	rg.PATCH("/materials/:material_id", httpserver.BodyLimit(h.textLimit, http.StatusBadRequest, "bad_request"), h.editMaterial)
	rg.DELETE("/materials/:material_id",
		httpserver.BodyLimit(h.jsonLimit, http.StatusBadRequest, "bad_request"), h.delete)
}

type urlCreateRequest struct {
	URL               string  `json:"url"`
	PreferredLanguage *string `json:"preferred_language"`
}

type textCreateRequest struct {
	Title *string `json:"title"`
	Text  string  `json:"text"`
}

// createURL 处理 POST /api/v1/lessons/{lesson_id}/materials/url。
func (h *Handler) createURL(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	var req urlCreateRequest
	if !httpx.BindJSON(c, &req) {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "请求体必须是符合契约的 JSON。")
		return
	}
	var errs []httpserver.FieldError
	if req.URL == "" {
		errs = append(errs, httpx.FieldErr("url", "required", "url 不能为空。"))
	} else if !strings.HasPrefix(strings.ToLower(req.URL), "https://") {
		errs = append(errs, httpx.FieldErr("url", "pattern", "url 必须以 https:// 开头。"))
	}
	if req.PreferredLanguage != nil && utf8.RuneCountInString(*req.PreferredLanguage) > 30 {
		errs = append(errs, httpx.FieldErr("preferred_language", "too_long",
			"preferred_language 不能超过 30 字符。"))
	}
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	canonical := map[string]any{"url": req.URL}
	if req.PreferredLanguage != nil {
		canonical["preferred_language"] = *req.PreferredLanguage
	}
	requestHash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(canonical), "")
	h.idem.Run(c, "createUrlMaterial", requestHash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			body, err := h.svc.CreateURLMaterial(ctx, h.idem.Svc, rec, URLInput{
				LessonID:          lessonID,
				URL:               req.URL,
				PreferredLanguage: req.PreferredLanguage,
			})
			if err != nil {
				return 0, nil, err
			}
			c.Header("Location", "/api/v1/jobs/"+body.Job.ID.String())
			return http.StatusAccepted, body, nil
		})
}

// createText 处理 POST /api/v1/lessons/{lesson_id}/materials/text。
func (h *Handler) createText(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	var req textCreateRequest
	if !httpx.BindJSON(c, &req) {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "请求体必须是符合契约的 JSON。")
		return
	}
	var errs []httpserver.FieldError
	if n := utf8.RuneCountInString(req.Text); n == 0 {
		errs = append(errs, httpx.FieldErr("text", "required", "text 不能为空。"))
	} else if n > 1000000 {
		errs = append(errs, httpx.FieldErr("text", "too_long", "text 不能超过 1000000 字符。"))
	}
	if req.Title != nil && utf8.RuneCountInString(*req.Title) > 200 {
		errs = append(errs, httpx.FieldErr("title", "too_long", "title 不能超过 200 字符。"))
	}
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	canonical := map[string]any{"text": req.Text}
	if req.Title != nil {
		canonical["title"] = *req.Title
	}
	requestHash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(canonical), "")
	h.idem.Run(c, "createTextMaterial", requestHash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			body, err := h.svc.CreateTextMaterial(ctx, h.idem.Svc, rec, TextInput{
				LessonID: lessonID,
				Title:    req.Title,
				Text:     req.Text,
			})
			if err != nil {
				return 0, nil, err
			}
			c.Header("Location", "/api/v1/jobs/"+body.Job.ID.String())
			return http.StatusAccepted, body, nil
		})
}

// upload 处理 POST /api/v1/lessons/{lesson_id}/materials/upload。
//
// 顺序：先校验 Idempotency-Key（不读 body）→ MultipartReader 流式读取
// file part（边写暂存边算 SHA-256，前 512 字节嗅探 MIME）→ MIME 白名单
// （415）→ 按元数据+文件 hash 计算 requestHash → IdemExecutor.Run →
// Service.UploadMaterial（两阶段发布）。任何失败或非 fresh 判定都会
// 删除暂存文件。
func (h *Handler) upload(c *gin.Context) {
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "缺少 Idempotency-Key 请求头。")
		return
	}
	if _, err := uuid.Parse(key); err != nil {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "Idempotency-Key 必须是 UUID。")
		return
	}
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	mr, err := c.Request.MultipartReader()
	if err != nil {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "请求必须是 multipart/form-data。")
		return
	}
	stagingDir, stagedPath, err := h.svc.PrepareStaging()
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	// 成功发布后暂存文件已被 rename 接管，删除空目录无副作用；
	// 其他所有路径都必须删除暂存文件。
	defer func() { _ = os.RemoveAll(stagingDir) }()

	form, err := h.streamUpload(mr, stagedPath)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			// 交给 BodyLimit 中间件映射 413 upload_too_large。
			_ = c.Error(err)
			c.Abort()
			return
		}
		httpx.WriteError(c, err)
		return
	}
	requestHash := httpx.HashRequest(c.Request.Method, c.FullPath(),
		httpx.CanonicalJSON(map[string]any{
			"filename":           form.filename,
			"preferred_language": form.preferredLanguage,
			"transcription_mode": form.transcriptionMode,
		}), form.sha256)
	h.idem.Run(c, "uploadMaterial", requestHash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			body, err := h.svc.UploadMaterial(ctx, h.idem.Svc, rec, UploadInput{
				LessonID:          lessonID,
				StagedPath:        stagedPath,
				OriginalName:      form.filename,
				MediaType:         form.mediaType,
				PreferredLanguage: form.preferredLanguage,
				TranscriptionMode: form.transcriptionMode,
			})
			if err != nil {
				return 0, nil, err
			}
			c.Header("Location", "/api/v1/jobs/"+body.Job.ID.String())
			return http.StatusAccepted, body, nil
		})
}

// uploadForm 是流式解析 multipart 的结果。
type uploadForm struct {
	filename          string
	mediaType         string
	sha256            string
	size              int64
	preferredLanguage string
	transcriptionMode string
}

// streamUpload 流式读取 multipart：file part 写入 stagedPath（边写边算
// SHA-256，前 512 字节嗅探 MIME），字段 part 只取
// preferred_language/transcription_mode。
func (h *Handler) streamUpload(mr *multipart.Reader, stagedPath string) (*uploadForm, error) {
	form := &uploadForm{}
	haveFile := false
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return nil, err
			}
			return nil, badRequestProblemErr("multipart 数据无效。")
		}
		switch name := part.FormName(); {
		case name == "file" && part.FileName() != "":
			if haveFile {
				_ = part.Close()
				return nil, badRequestProblemErr("file 字段只能上传一个文件。")
			}
			haveFile = true
			form.filename = sanitizeFilename(part.FileName())
			mt, sum, size, serr := stageFile(part, stagedPath)
			_ = part.Close()
			if serr != nil {
				return nil, serr
			}
			form.mediaType, form.sha256, form.size = mt, sum, size
		case name == "preferred_language" || name == "transcription_mode":
			v, rerr := io.ReadAll(io.LimitReader(part, 1<<10))
			_ = part.Close()
			if rerr != nil {
				var maxErr *http.MaxBytesError
				if errors.As(rerr, &maxErr) {
					return nil, rerr
				}
				return nil, badRequestProblemErr("multipart 数据无效。")
			}
			if name == "preferred_language" {
				form.preferredLanguage = string(v)
			} else {
				form.transcriptionMode = string(v)
			}
		default:
			// 契约未声明 additionalProperties:false，未知字段直接丢弃。
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
		}
	}
	if !haveFile {
		return nil, badRequestProblemErr("缺少 file 字段。")
	}
	base, _, err := mime.ParseMediaType(form.mediaType)
	if err != nil || !allowedMIME[base] {
		return nil, httpx.NewProblemError(http.StatusUnsupportedMediaType,
			"unsupported_media_type", "不支持的文件类型",
			"仅支持 图片/PDF/PPTX/DOCX/文本/字幕/HTML/常见音视频素材。")
	}
	if form.transcriptionMode != "" &&
		form.transcriptionMode != TranscriptionSubtitleOnly &&
		form.transcriptionMode != TranscriptionWhisperLocal {
		p := httpserver.NewProblem(http.StatusUnprocessableEntity, "validation_error",
			"请求参数无效", "请修正标记的字段后重试。")
		p.Errors = []httpserver.FieldError{httpx.FieldErr("transcription_mode", "invalid_enum",
			"transcription_mode 必须是 subtitle_only 或 whisper_local。")}
		return nil, &httpx.ProblemError{Problem: p}
	}
	return form, nil
}

// stageFile 把 file part 流式写入 stagedPath：先读前 512 字节嗅探 MIME，
// 再续写剩余部分，全程计算 SHA-256 与大小。
func stageFile(part *multipart.Part, stagedPath string) (mediaType, shaHex string, size int64, err error) {
	f, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", "", 0, fmt.Errorf("material: 创建暂存文件失败: %w", err)
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 512)
	n, rerr := io.ReadFull(part, head)
	if rerr != nil && !errors.Is(rerr, io.EOF) && !errors.Is(rerr, io.ErrUnexpectedEOF) {
		var maxErr *http.MaxBytesError
		if errors.As(rerr, &maxErr) {
			return "", "", 0, rerr
		}
		return "", "", 0, badRequestProblemErr("multipart 数据无效。")
	}
	head = head[:n]
	mediaType = http.DetectContentType(head)
	// Go's standard sniffer misses FLAC, some M4A containers and headerless MP3/AAC.
	// Match actual signatures; never trust the browser MIME or filename.
	if bytes.HasPrefix(head, []byte("II\x2a\x00")) || bytes.HasPrefix(head, []byte("MM\x00\x2a")) {
		mediaType = "image/tiff"
	} else if bytes.HasPrefix(head, []byte("fLaC")) {
		mediaType = "audio/flac"
	} else if len(head) >= 12 && string(head[4:8]) == "ftyp" && (string(head[8:11]) == "M4A" || string(head[8:11]) == "M4B") {
		mediaType = "audio/mp4"
	} else if len(head) >= 2 && head[0] == 0xff && head[1]&0xe0 == 0xe0 {
		if head[1]&0xf6 == 0xf0 {
			mediaType = "audio/aac"
		} else {
			mediaType = "audio/mpeg"
		}
	}
	h := sha256.New()
	w := io.MultiWriter(f, h)
	if _, err := w.Write(head); err != nil {
		return "", "", 0, fmt.Errorf("material: 写暂存文件失败: %w", err)
	}
	rest, err := io.Copy(w, part)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return "", "", 0, err
		}
		return "", "", 0, badRequestProblemErr("multipart 数据无效。")
	}
	return mediaType, hex.EncodeToString(h.Sum(nil)), int64(n) + rest, nil
}

// sanitizeFilename 只保留路径基名用于展示（防路径穿越）。
func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	if base == "." || base == "/" || base == `\` {
		return "upload"
	}
	return base
}

// list 处理 GET /api/v1/lessons/{lesson_id}/materials（游标分页）。
func (h *Handler) list(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "limit 必须是 1-100 的整数。")
		return
	}
	var afterCreatedAt *time.Time
	var afterID *uuid.UUID
	if raw := c.Query("cursor"); raw != "" {
		var cur map[string]any
		if !httpx.DecodeCursor(raw, &cur) {
			httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
				"请求无效", "cursor 无效。")
			return
		}
		tsRaw, ok1 := cur["created_at"].(string)
		idRaw, ok2 := cur["id"].(string)
		ts, err1 := time.Parse(time.RFC3339Nano, tsRaw)
		id, err2 := uuid.Parse(idRaw)
		if !ok1 || !ok2 || err1 != nil || err2 != nil {
			httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
				"请求无效", "cursor 无效。")
			return
		}
		afterCreatedAt = &ts
		afterID = &id
	}
	coll, err := h.svc.ListMaterials(c.Request.Context(), ListInput{
		LessonID:       lessonID,
		AfterCreatedAt: afterCreatedAt,
		AfterID:        afterID,
		Limit:          limit,
	})
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, coll)
}

// delete 处理 DELETE /api/v1/materials/{material_id}。
func (h *Handler) delete(c *gin.Context) {
	materialID, ok := pathUUID(c, "material_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteMaterial(c.Request.Context(), materialID); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// pathUUID 解析路径参数 UUID；非法时写 400 并返回 false。
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request",
			"请求无效", "路径参数 "+name+" 必须是 UUID。")
		return uuid.Nil, false
	}
	return id, true
}

// badRequestProblemErr 构造 400 bad_request 错误（multipart 解析失败）。
func badRequestProblemErr(detail string) *httpx.ProblemError {
	return httpx.NewProblemError(http.StatusBadRequest, "bad_request", "请求无效", detail)
}
