package job

import (
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
)

// ResourceBusyError 构造带 active_job_id/generation_family 扩展的
// 409 resource_busy（04 §8：不同幂等键也不能创建第二个活动任务）。
// 本构造函数属于 job 模块：httpx 不反向依赖业务包，生成任务冲突的
// 响应由各生成类 service 统一经本函数产出。
func ResourceBusyError(activeJobID uuid.UUID) *httpx.ProblemError {
	p := httpserver.NewProblem(409, "resource_busy", "资源繁忙",
		"该课时已有进行中的生成任务，请关联到现有任务。")
	p.Extensions = map[string]any{
		"active_job_id":     activeJobID.String(),
		"generation_family": GenerationFamilyLessonGeneration,
	}
	return &httpx.ProblemError{Problem: p}
}
