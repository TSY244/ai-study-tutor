// Package api 装配 /api/v1 业务路由：统一构造各模块 Service/Handler、
// 共享幂等执行器与路由级 BodyLimit 中间件。
//
// 本包只做装配，不含业务逻辑；模块边界（handler/service/repository）
// 由各内部包自持。
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/attempt"
	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/export"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/material"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
	"github.com/TSY244/ai-study-tutor/backend/internal/sitecookie"
	"github.com/TSY244/ai-study-tutor/backend/internal/system"
)

// DefaultURLAllowlist 是 URL 素材的默认主机白名单（精确或后缀匹配；
// 空列表表示全部拒绝）。本机单用户部署默认放行两个视频站点。
var DefaultURLAllowlist = []string{
	"youtube.com", "www.youtube.com",
	"bilibili.com", "www.bilibili.com",
}

// Deps 是路由装配的依赖集合。
type Deps struct {
	Pool  *pgxpool.Pool
	Files *filestore.Store
	// Version is the deployed application version displayed by system status.
	Version string
	// URLAllowlist 为 nil 时使用 DefaultURLAllowlist。
	URLAllowlist []string
	// SkipURLDNSValidation is used by the no-egress API deployment. Worker DNS
	// and redirect validation remains mandatory before any remote fetch.
	SkipURLDNSValidation bool
	// BodyLimitBytes 是 JSON 端点请求体上限；<=0 时默认 1 MiB。
	BodyLimitBytes int64
	// UploadMaxBytes 是 multipart 上传上限；<=0 时由 material 模块默认。
	UploadMaxBytes int64
	// ClaudeGateway 是 Claude 多 Key 网关管理端点；nil 表示未配置网关
	//（/system/claude-keys 不注册）。由 cmd/api 按 CLAUDE_GATEWAY_CONFIG_FILE 注入。
	ClaudeGateway *claudegateway.Handler
	// BilibiliCookie 是 B 站 Cookie 管理端点；nil 表示未配置
	//（/system/bilibili-cookie 不注册）。由 cmd/api 按 SITE_COOKIES_FILE 注入。
	BilibiliCookie *sitecookie.Handler
}

// RegisterRoutes 在引擎上注册全部 /api/v1 业务端点。
func RegisterRoutes(engine *gin.Engine, d Deps) {
	if d.BodyLimitBytes <= 0 {
		d.BodyLimitBytes = 1 << 20
	}
	if d.URLAllowlist == nil {
		d.URLAllowlist = DefaultURLAllowlist
	}
	rg := engine.Group("/api/v1")
	jsonLimit := httpserver.BodyLimit(d.BodyLimitBytes, http.StatusBadRequest, "bad_request")
	idem := &httpx.IdemExecutor{Svc: idempotency.NewService(d.Pool)}

	courseRepo := course.Repository{}
	lessonRepo := lesson.Repository{}
	attemptRepo := attempt.Repository{}

	course.NewHandler(course.NewService(d.Pool), idem).RegisterRoutes(rg, jsonLimit)
	lesson.NewHandler(lesson.NewService(d.Pool), idem).RegisterRoutes(rg, jsonLimit)
	materialSvc := material.NewService(d.Pool, d.Files, d.URLAllowlist)
	materialSvc.SkipURLDNSValidation = d.SkipURLDNSValidation
	material.NewHandler(materialSvc, idem, d.UploadMaxBytes, 0, d.BodyLimitBytes).RegisterRoutes(rg)
	document.NewHandler(document.NewService(d.Pool), idem).RegisterRoutes(rg, jsonLimit)

	// 出题生成：常规练习接真实策略门禁（掌握度提前停止 → 422 practice_not_needed）；
	// 复习出题不受限制。
	genSvc := quiz.NewGenerationService(d.Pool)
	genSvc.QuizPolicy = (&mastery.PracticeEvaluator{
		Scores:  attemptRepo,
		Weak:    mastery.Repository{},
		Lessons: lessonRepo,
	}).Needed
	quiz.NewGenerationHandler(genSvc, idem).RegisterRoutes(rg, jsonLimit)
	quiz.NewQueryHandler(quiz.NewQueryService(d.Pool)).RegisterRoutes(rg)

	attempt.NewHandler(attempt.NewService(d.Pool), idem).RegisterRoutes(rg, jsonLimit)
	mastery.NewHandler(
		mastery.NewWeaknessService(d.Pool, attemptRepo),
		mastery.NewProgressService(d.Pool, attemptRepo, courseRepo, lessonRepo),
		idem).RegisterRoutes(rg, jsonLimit)
	job.NewHandler(job.NewService(d.Pool), idem).RegisterRoutes(rg, jsonLimit)
	system.NewHandler(job.NewService(d.Pool), idem,
		system.NewStatusService(d.Pool, d.Files.DataDir(), d.Version)).RegisterRoutes(rg, jsonLimit)
	if d.ClaudeGateway != nil {
		d.ClaudeGateway.RegisterRoutes(rg, jsonLimit, idem)
	}
	if d.BilibiliCookie != nil {
		d.BilibiliCookie.RegisterRoutes(rg, jsonLimit, idem)
	}
	export.NewHandler(export.NewService(d.Pool, d.Files)).RegisterRoutes(rg, jsonLimit)
}
