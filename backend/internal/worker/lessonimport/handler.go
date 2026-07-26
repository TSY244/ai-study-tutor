package lessonimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/material"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

// lesson_import 稳定 stage code（03 §6.4；generating_* 阶段在 B6 接入，
// 本阶段按需跳过）。进度百分比是 API 展示契约的一部分。
const (
	StageAcquiring                = "acquiring"
	StageDetecting                = "detecting"
	StageExtracting               = "extracting"
	StageNormalizing              = "normalizing"
	StageChunking                 = "chunking"
	StageGeneratingTeacherOutline = "generating_teacher_outline"
	StageGeneratingStudentNotes   = "generating_student_notes"
	StageValidating               = "validating"
	StagePublishing               = "publishing"
)

const (
	progressAcquiring                = 10
	progressDetecting                = 20
	progressExtracting               = 40
	progressNormalizing              = 55
	progressChunking                 = 70
	progressGeneratingTeacherOutline = 75
	progressGeneratingStudentNotes   = 80
	progressValidating               = 85
	progressPublishing               = 95
)

// 默认上限。
const (
	defaultMaxDownloadBytes = int64(2) << 30
	defaultMaxTextRunes     = 2_000_000
)

// payload 是 lesson_import Job 的输入快照（API 创建时写入）。
type payload struct {
	MaterialID        uuid.UUID `json:"material_id"`
	LessonID          uuid.UUID `json:"lesson_id"`
	Kind              string    `json:"material_kind"`
	SourceURL         string    `json:"source_url"`
	FileObjectID      string    `json:"file_object_id"`
	OriginalName      string    `json:"original_name"`
	PreferredLanguage string    `json:"preferred_language"`
	TranscriptionMode string    `json:"transcription_mode"`
}

func parsePayload(raw []byte) (payload, error) {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	if p.MaterialID == uuid.Nil || p.LessonID == uuid.Nil {
		return p, errors.New("payload 缺少 material_id/lesson_id")
	}
	return p, nil
}

// Handler 实现 lesson_import 素材管线（worker.Handler）。
type Handler struct {
	Pool  *pgxpool.Pool
	Files *filestore.Store
	Proc  proc.Runner

	// SSRF 校验注入点（测试用桩，避免外网依赖）。
	Resolver  func(ctx context.Context, host string) ([]net.IP, error)
	Transport http.RoundTripper

	URLAllowlist     []string
	MaxDownloadBytes int64
	MaxTextRunes     int

	// 转写配置（payload.transcription_mode 优先于进程配置）。
	TranscriptionMode string
	WhisperModelPath  string

	// AI 仅在 Worker 显式注入 provider 时启用；nil 表示降级为只发布 raw_text。
	AI aipkg.Provider
	// AIAvailable 可选，用于多 Key 网关热重载：false 时保持 raw_text
	// 降级，运行期新增启用 Key 后无需重启即可开始生成文档。
	AIAvailable func() bool

	// URLImportAI 是 URL 素材专用的网关限定 Provider（GatewayOnly，无
	// legacy 回退），只负责教纲/笔记生成（普通 Messages 调用经中转网关）。
	// nil 或 URLImportAvailable()=false 时 URL 导入以非重试
	// claude_not_configured 失败：不做旧的 raw-text-only 降级，也不回退
	// legacy 直连 Key 或本地下载管线。
	URLImportAI aipkg.Provider
	// URLImportAvailable 可选，热重载判定网关当前是否有启用 Key（也是
	// CLI 提取的凭证来源）；nil 视为可用（测试便捷）。
	URLImportAvailable func() bool

	// URLExtractor 是 URL 素材的链接内容提取器（worker 容器内 Claude
	// Code CLI + WebFetch 客户端抓取，凭证取自 claudegateway keys.json）。
	// nil 时 URL 导入同样以非重试 claude_not_configured 失败。
	URLExtractor URLExtractor

	// 外部工具名（参数数组调用，无 shell）。
	YtDlpBin     string
	FFmpegBin    string
	WhisperBin   string
	PdftotextBin string

	materials material.Repository
	lessons   lesson.Repository
	documents document.Repository
	aiRuns    aipkg.Repository
}

// NewHandler 创建 Handler；未显式设置的字段取默认值。
func NewHandler(pool *pgxpool.Pool, files *filestore.Store, procRunner proc.Runner) *Handler {
	return &Handler{Pool: pool, Files: files, Proc: procRunner}
}

func (h *Handler) withDefaults() {
	if h.Proc == nil {
		h.Proc = proc.ExecRunner{}
	}
	if h.MaxDownloadBytes <= 0 {
		h.MaxDownloadBytes = defaultMaxDownloadBytes
	}
	if h.MaxTextRunes <= 0 {
		h.MaxTextRunes = defaultMaxTextRunes
	}
	if h.TranscriptionMode == "" {
		h.TranscriptionMode = "subtitle_only"
	}
	if h.YtDlpBin == "" {
		h.YtDlpBin = "yt-dlp"
	}
	if h.FFmpegBin == "" {
		h.FFmpegBin = "ffmpeg"
	}
	if h.WhisperBin == "" {
		h.WhisperBin = "whisper-cli"
	}
	if h.PdftotextBin == "" {
		h.PdftotextBin = "pdftotext"
	}
}

func (h *Handler) maxDownloadBytes() int64 { return h.MaxDownloadBytes }

// acquisition 是 acquiring 阶段的产物。
type acquisition struct {
	path          string      // 任务目录内的素材文件
	dtype         string      // 已确定的类型；空串表示需 detecting 阶段判定
	parserVersion string      // URL 路径已确定的解析器版本（subtitle/whisper）
	source        *sourceInfo // URL 素材的来源摘要
}

// Execute 运行素材处理管线。成功返回 Outcome（发布在 Complete 事务中执行）。
func (h *Handler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	h.withDefaults()
	p, err := parsePayload(ex.Job.Payload)
	if err != nil {
		return nil, worker.WrapError(worker.ErrCodeInternal,
			"任务输入快照无效，请重新创建任务。", false, err)
	}
	m, err := h.materials.GetByID(ctx, h.Pool, p.MaterialID)
	if errors.Is(err, material.ErrNotFound) {
		return nil, worker.NewError(worker.ErrCodeParseFailed, "素材不存在或已删除。", false)
	}
	if err != nil {
		return nil, worker.WrapError(worker.ErrCodeInternal, "读取素材失败，将自动重试。", true, err)
	}

	// URL 素材在通用文件获取序列之前分支：走 Claude Web Fetch 提取管线，
	// 不经过本地 DNS、重定向预检、yt-dlp、FFmpeg 或 Whisper。
	if p.Kind == material.KindURL {
		return h.executeURL(ctx, ex, p, m)
	}

	if err := ex.Stage(ctx, StageAcquiring, progressAcquiring, "获取素材内容"); err != nil {
		return nil, err
	}
	acq, err := h.acquire(ctx, ex, p, m)
	if err != nil {
		return nil, err
	}

	if err := ex.Stage(ctx, StageDetecting, progressDetecting, "识别素材类型"); err != nil {
		return nil, err
	}
	dtype := acq.dtype
	if dtype == "" {
		if dtype, err = detectFile(acq.path, originalName(p, m)); err != nil {
			return nil, worker.WrapError(worker.ErrCodeInternal, "读取素材文件失败，将自动重试。", true, err)
		}
	}

	if err := ex.Stage(ctx, StageExtracting, progressExtracting, "提取文本内容"); err != nil {
		return nil, err
	}
	rawText, parserVersion, err := h.extract(ctx, acq.path, dtype, effectiveMode(h, p), ex.WorkDir)
	if err != nil {
		return nil, err
	}
	if acq.parserVersion != "" {
		parserVersion = acq.parserVersion
	}

	if err := ex.Stage(ctx, StageNormalizing, progressNormalizing, "规范化文本"); err != nil {
		return nil, err
	}
	norm := Normalize(rawText)
	if norm == "" {
		return nil, worker.NewError(worker.ErrCodeEmptyContent,
			"未提取到有效文本内容。请检查文件内容或直接粘贴文本。", false)
	}
	if len([]rune(norm)) > h.MaxTextRunes {
		return nil, worker.NewError(worker.ErrCodeTooLarge,
			fmt.Sprintf("文本超过上限（%d 字符）。请拆分素材后重试。", h.MaxTextRunes), false)
	}

	if err := ex.Stage(ctx, StageChunking, progressChunking, "切分文本块"); err != nil {
		return nil, err
	}
	chunks := ChunkText(norm)

	if err := ex.Stage(ctx, StageValidating, progressValidating, "校验解析结果"); err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, worker.NewError(worker.ErrCodeEmptyContent,
			"未提取到有效文本内容。请检查文件内容或直接粘贴文本。", false)
	}
	generated := map[string]string{}
	if h.AI != nil && (h.AIAvailable == nil || h.AIAvailable()) {
		generated, err = h.generateLessonDocuments(ctx, ex, p, norm, h.AI)
		if err != nil {
			return nil, err
		}
	}

	if err := ex.Stage(ctx, StagePublishing, progressPublishing, "发布文档版本"); err != nil {
		return nil, err
	}
	return &worker.Outcome{
		Publish: h.publishFunc(ex.Job, p, m, norm, dtype, parserVersion, acq.source, chunks, generated),
	}, nil
}

// acquire 按素材类型获取内容到任务目录。
func (h *Handler) acquire(ctx context.Context, ex *worker.Execution, p payload, m *material.Material) (*acquisition, error) {
	switch p.Kind {
	case material.KindText:
		text := ""
		if m.InlineText != nil {
			text = *m.InlineText
		}
		path := filepath.Join(ex.WorkDir, "acquired.txt")
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return nil, worker.WrapError(worker.ErrCodeInternal, "写入临时文件失败，将自动重试。", true, err)
		}
		return &acquisition{path: path, dtype: TypeTXT}, nil

	case material.KindUpload:
		objID := p.FileObjectID
		if objID == "" && m.FileObjectID != nil {
			objID = m.FileObjectID.String()
		}
		id, err := uuid.Parse(objID)
		if err != nil {
			return nil, worker.NewError(worker.ErrCodeParseFailed, "上传素材的文件引用无效。", false)
		}
		obj, err := h.Files.GetByID(ctx, h.Pool, id)
		if err != nil {
			return nil, worker.NewError(worker.ErrCodeParseFailed, "上传文件不可用，请重新上传。", false)
		}
		if err := h.Files.AssertReady(ctx, h.Pool, obj.ID); err != nil {
			return nil, worker.NewError(worker.ErrCodeParseFailed, "上传文件不可用，请重新上传。", false)
		}
		src, err := h.Files.AbsPath(obj.StorageKey)
		if err != nil {
			return nil, worker.NewError(worker.ErrCodeParseFailed, "上传文件不可用，请重新上传。", false)
		}
		dst := filepath.Join(ex.WorkDir, "acquired"+strings.ToLower(filepath.Ext(originalName(p, m))))
		if err := copyWithLimit(src, dst, h.maxDownloadBytes()); err != nil {
			return nil, err
		}
		return &acquisition{path: dst}, nil
	}
	return nil, worker.NewError(worker.ErrCodeUnsupportedType, "未知素材类型。", false)
}

// executeURL 运行 URL 素材的 Claude 提取管线：canonical 再校验 ->
// worker 容器内 Claude Code CLI（WebFetch 客户端抓取）提取 -> 规范化/切分
// -> 同一网关限定 Provider 生成两份文档 -> 一次性发布（publishFunc 事务
// 保持 all-or-nothing）。全程不经过本地 DNS、重定向预检、yt-dlp、FFmpeg
// 或 Whisper。
func (h *Handler) executeURL(ctx context.Context, ex *worker.Execution, p payload, m *material.Material) (*worker.Outcome, error) {
	if h.URLExtractor == nil || h.URLImportAI == nil || (h.URLImportAvailable != nil && !h.URLImportAvailable()) {
		return nil, worker.NewError(worker.ErrCodeClaudeNotConfigured,
			"Claude 网关未配置启用的 Key，无法导入链接素材；请配置后重试。", false)
	}
	raw := p.SourceURL
	if raw == "" && m.SourceURL != nil {
		raw = *m.SourceURL
	}
	if err := ex.Stage(ctx, StageAcquiring, progressAcquiring, "校验素材链接"); err != nil {
		return nil, err
	}
	canonical, domains, err := canonicalClaudeURL(raw, h.URLAllowlist)
	if err != nil {
		return nil, err
	}
	req, err := aipkg.BuildLessonURLExtractRequest(aipkg.LessonURLExtractPromptInput{
		URL:            canonical,
		AllowedDomains: domains,
	})
	if err != nil {
		return nil, worker.WrapError(worker.ErrCodeInternal, "构建 Claude 请求失败。", false, err)
	}

	if err := ex.Stage(ctx, StageExtracting, progressExtracting, "Claude 提取链接内容"); err != nil {
		return nil, err
	}
	res, err := h.generateClaudeURL(ctx, ex.Job.ID, ex.WorkDir, req)
	if err != nil {
		return nil, err
	}

	if err := ex.Stage(ctx, StageNormalizing, progressNormalizing, "规范化文本"); err != nil {
		return nil, err
	}
	norm := Normalize(res.RawTextMarkdown)
	if err := validateClaudeURLText(norm, h.MaxTextRunes); err != nil {
		return nil, err
	}

	if err := ex.Stage(ctx, StageChunking, progressChunking, "切分文本块"); err != nil {
		return nil, err
	}
	chunks := ChunkText(norm)

	if err := ex.Stage(ctx, StageValidating, progressValidating, "校验解析结果"); err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, worker.NewError(worker.ErrCodeEmptyContent,
			"未提取到有效文本内容。请检查链接内容或直接粘贴文本。", false)
	}

	// URL 素材的教纲/笔记必须由同一网关限定 Provider 生成（防止提取与
	// 生成之间 Key 被移除导致 legacy 回退），且两份都非空才允许进入
	// 发布事务，保证三份文档 all-or-nothing。
	generated, err := h.generateLessonDocuments(ctx, ex, p, norm, h.URLImportAI)
	if err != nil {
		return nil, err
	}
	for _, kind := range []string{document.KindTeacherOutline, document.KindStudentNotes} {
		if strings.TrimSpace(generated[kind]) == "" {
			return nil, worker.NewError(worker.ErrCodeClaudeDomainInvalid,
				"Claude 生成的学习文档不完整，将自动重试。", true)
		}
	}

	if err := ex.Stage(ctx, StagePublishing, progressPublishing, "发布文档版本"); err != nil {
		return nil, err
	}
	return &worker.Outcome{
		Publish: h.publishFunc(ex.Job, p, m, norm, TypeTXT, claudeURLParserVersion,
			claudeURLSourceInfo(res, canonical), chunks, generated),
	}, nil
}

// generateClaudeURL 经 worker 容器内的 Claude Code CLI 提取链接正文，并按
// generateMarkdown 同款模式恰好记录一条 purpose=lesson_url_extract 的
// 脱敏 ai_run；提取错误经 claudeWorkerError 保留错误码与重试标志
// （ErrMaterialFetchFailed -> worker.ErrCodeMaterialFetchFailed）。
func (h *Handler) generateClaudeURL(ctx context.Context, jobID uuid.UUID, workDir string, req aipkg.Request) (claudeURLResult, error) {
	resp, callErr := h.URLExtractor.Extract(ctx, workDir, req)
	var res claudeURLResult
	runErr := callErr
	if callErr == nil {
		res, runErr = parseClaudeURLResult(resp.JSON)
	}
	if err := h.aiRuns.Create(ctx, h.Pool, aipkg.NewRun(jobID, req, resp, runErr)); err != nil {
		return claudeURLResult{}, worker.WrapError(worker.ErrCodeInternal, "记录 Claude 调用失败，将自动重试。", true, err)
	}
	if runErr != nil {
		return claudeURLResult{}, claudeWorkerError(runErr)
	}
	return res, nil
}

// validateClaudeURLText 校验 Claude 提取的正文：空文本与超长映射为稳定的
// 不可重试错误码（与本地管线同一语义）。
func validateClaudeURLText(norm string, maxRunes int) error {
	if norm == "" {
		return worker.NewError(worker.ErrCodeEmptyContent,
			"未提取到有效文本内容。请检查链接内容或直接粘贴文本。", false)
	}
	if len([]rune(norm)) > maxRunes {
		return worker.NewError(worker.ErrCodeTooLarge,
			fmt.Sprintf("文本超过上限（%d 字符）。请拆分素材后重试。", maxRunes), false)
	}
	return nil
}

func (h *Handler) generateLessonDocuments(ctx context.Context, ex *worker.Execution, p payload, rawText string, provider aipkg.Provider) (map[string]string, error) {
	les, err := h.lessons.GetByID(ctx, h.Pool, p.LessonID)
	if errors.Is(err, lesson.ErrNotFound) {
		return nil, worker.NewError(worker.ErrCodeParseFailed, "课时不存在或已删除。", false)
	}
	if err != nil {
		return nil, worker.WrapError(worker.ErrCodeInternal, "读取课时失败，将自动重试。", true, err)
	}
	out := map[string]string{}
	for _, spec := range []struct {
		kind     string
		stage    string
		progress int16
		message  string
	}{
		{kind: document.KindTeacherOutline, stage: StageGeneratingTeacherOutline, progress: progressGeneratingTeacherOutline, message: "生成教师教纲"},
		{kind: document.KindStudentNotes, stage: StageGeneratingStudentNotes, progress: progressGeneratingStudentNotes, message: "生成学生笔记"},
	} {
		if err := ex.Stage(ctx, spec.stage, spec.progress, spec.message); err != nil {
			return nil, err
		}
		req, err := aipkg.BuildLessonDocumentRequest(aipkg.LessonDocumentPromptInput{
			LessonTitle: les.Title,
			RawText:     rawText,
			Kind:        spec.kind,
		})
		if err != nil {
			return nil, worker.WrapError(worker.ErrCodeInternal, "构建 Claude 请求失败。", false, err)
		}
		md, err := h.generateMarkdown(ctx, ex.Job.ID, req, provider)
		if err != nil {
			return nil, err
		}
		out[spec.kind] = md
	}
	return out, nil
}

func (h *Handler) generateMarkdown(ctx context.Context, jobID uuid.UUID, req aipkg.Request, provider aipkg.Provider) (string, error) {
	resp, callErr := provider.Generate(ctx, req)
	var parsed struct {
		Markdown string `json:"markdown"`
	}
	runErr := callErr
	if callErr == nil {
		if err := json.Unmarshal(resp.JSON, &parsed); err != nil {
			runErr = aipkg.WrapError(aipkg.ErrInvalidJSON, "Claude 返回的 JSON 无法解析，请重试。", err)
		} else if strings.TrimSpace(parsed.Markdown) == "" {
			runErr = aipkg.ErrorForCode(aipkg.ErrDomainInvalid, "Claude 返回的文档内容为空，请重试。")
		}
	}
	if err := h.aiRuns.Create(ctx, h.Pool, aipkg.NewRun(jobID, req, resp, runErr)); err != nil {
		return "", worker.WrapError(worker.ErrCodeInternal, "记录 Claude 调用失败，将自动重试。", true, err)
	}
	if runErr != nil {
		return "", claudeWorkerError(runErr)
	}
	return strings.TrimSpace(parsed.Markdown), nil
}

func claudeWorkerError(err error) error {
	perr := aipkg.AsProviderError(err)
	return worker.WrapError(perr.Code, perr.Detail, perr.Retryable, err)
}

// publishFunc 在 Complete 事务中：发布 raw_text 新版本、推进素材/课时状态，
// 并返回写入 jobs.result 的小体积摘要（chunk 清单只存聚合 hash，可由
// raw_text 以 chunk/v1 复现）。
func (h *Handler) publishFunc(j *job.Job, p payload, m *material.Material, norm, dtype, parserVersion string,
	source *sourceInfo, chunks []Chunk, generated map[string]string) worker.PublishFunc {
	return func(ctx context.Context, tx pgx.Tx) ([]byte, error) {
		d := &document.Document{
			LessonID:    p.LessonID,
			Kind:        document.KindRawText,
			ContentMD:   norm,
			SourceJobID: &j.ID,
		}
		if err := h.documents.CreateVersion(ctx, tx, d); err != nil {
			return nil, err
		}
		if err := h.documents.Publish(ctx, tx, d.ID); err != nil {
			return nil, err
		}
		published := map[string]map[string]any{
			document.KindRawText: {
				"document_id": d.ID.String(),
				"revision":    d.Revision,
			},
		}
		for _, kind := range []string{document.KindTeacherOutline, document.KindStudentNotes} {
			md := strings.TrimSpace(generated[kind])
			if md == "" {
				continue
			}
			doc := &document.Document{
				LessonID:    p.LessonID,
				Kind:        kind,
				ContentMD:   md,
				SourceJobID: &j.ID,
			}
			if err := h.documents.CreateVersion(ctx, tx, doc); err != nil {
				return nil, err
			}
			if err := h.documents.Publish(ctx, tx, doc.ID); err != nil {
				return nil, err
			}
			published[kind] = map[string]any{
				"document_id": doc.ID.String(),
				"revision":    doc.Revision,
			}
		}
		patch := map[string]any{
			"detected_type":     dtype,
			"parser_version":    parserVersion,
			"normalize_version": NormalizeVersion,
			"chunk_version":     ChunkVersion,
			"char_count":        len([]rune(norm)),
			"chunk_count":       len(chunks),
		}
		if source != nil {
			patch["source"] = source
		}
		if err := h.materials.SetResult(ctx, tx, m.ID, material.StatusReady, mergeMeta(m.Metadata, patch)); err != nil {
			return nil, err
		}
		les, err := h.lessons.GetByID(ctx, tx, p.LessonID)
		if err != nil {
			return nil, err
		}
		if err := h.lessons.UpdateStatus(ctx, tx, les.ID, "ready", les.Version); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"material_id":       m.ID.String(),
			"document_id":       d.ID.String(),
			"document_revision": d.Revision,
			"detected_type":     dtype,
			"parser_version":    parserVersion,
			"normalize_version": NormalizeVersion,
			"chunk_version":     ChunkVersion,
			"char_count":        len([]rune(norm)),
			"chunk_count":       len(chunks),
			"chunks_sha256":     manifestHash(chunks),
			"documents":         published,
		})
	}
}

// FinalizeFail 在 Job 进入 failed 终态的同一事务：素材置 failed 并记录
// 错误码，课时置 failed。素材/课时已删除时跳过（best effort）。
func (h *Handler) FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error {
	p, err := parsePayload(j.Payload)
	if err != nil {
		return nil
	}
	m, err := h.materials.GetByID(ctx, tx, p.MaterialID)
	if err == nil {
		if err := h.materials.SetResult(ctx, tx, m.ID, material.StatusFailed,
			mergeMeta(m.Metadata, map[string]any{"error_code": code})); err != nil {
			return err
		}
	} else if !errors.Is(err, material.ErrNotFound) {
		return err
	}
	if les, err := h.lessons.GetByID(ctx, tx, p.LessonID); err == nil {
		return h.lessons.UpdateStatus(ctx, tx, les.ID, "failed", les.Version)
	} else if errors.Is(err, lesson.ErrNotFound) {
		return nil
	} else {
		return err
	}
}

// FinalizeCancel 在确认取消的同一事务：素材保持 pending（未处理，不污名化），
// 课时状态按是否已有发布文档归位（有文档回 ready，否则回 draft）。
func (h *Handler) FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error {
	p, err := parsePayload(j.Payload)
	if err != nil {
		return nil
	}
	les, err := h.lessons.GetByID(ctx, tx, p.LessonID)
	if errors.Is(err, lesson.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	docs, err := h.documents.ListActiveByLesson(ctx, tx, p.LessonID)
	if err != nil {
		return err
	}
	status := "draft"
	if len(docs) > 0 {
		status = "ready"
	}
	return h.lessons.UpdateStatus(ctx, tx, les.ID, status, les.Version)
}

// effectiveMode 计算转写模式：payload 覆盖进程配置。
func effectiveMode(h *Handler, p payload) string {
	if p.TranscriptionMode != "" {
		return p.TranscriptionMode
	}
	return h.TranscriptionMode
}

// originalName 取展示文件名：payload 优先，素材记录兜底。
func originalName(p payload, m *material.Material) string {
	if p.OriginalName != "" {
		return p.OriginalName
	}
	if m.OriginalName != nil && *m.OriginalName != "" {
		return *m.OriginalName
	}
	return "upload"
}

// copyWithLimit 流式复制上传文件到任务目录并强制体积上限。
func copyWithLimit(src, dst string, limit int64) error {
	in, err := os.Open(src)
	if err != nil {
		return worker.WrapError(worker.ErrCodeInternal, "读取素材文件失败，将自动重试。", true, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return worker.WrapError(worker.ErrCodeInternal, "写入临时文件失败，将自动重试。", true, err)
	}
	defer func() { _ = out.Close() }()
	written, err := io.Copy(out, io.LimitReader(in, limit+1))
	if err != nil {
		return worker.WrapError(worker.ErrCodeInternal, "读取素材文件失败，将自动重试。", true, err)
	}
	if written > limit {
		return worker.NewError(worker.ErrCodeTooLarge, "文件超过下载/体积上限。", false)
	}
	return nil
}

// manifestHash 聚合 chunk 清单 hash（复现校验用，不含正文）。
func manifestHash(chunks []Chunk) string {
	h := sha256.New()
	for _, c := range chunks {
		h.Write([]byte(c.SHA256))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// mergeMeta 合并素材元数据补丁（不覆盖未提及的既有键）。
func mergeMeta(base []byte, patch map[string]any) []byte {
	m := map[string]any{}
	_ = json.Unmarshal(base, &m)
	for k, v := range patch {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}
