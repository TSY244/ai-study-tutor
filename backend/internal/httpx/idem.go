package httpx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// IdemExecutor 编排 POST 创建类端点的幂等语义（04 §2.3）：
//
//  1. Idempotency-Key 必须存在且为 UUID（缺失/非法 → 400 bad_request）；
//  2. Begin 判定 fresh/replay/conflict/in_progress；
//  3. fresh 时执行 fn；fn 成功返回 (status, body) 后写出响应——fn 必须
//     在业务事务内调用 CompleteJSON（同事务提交）；fn 失败则 Abandon
//     （事务前失败不留 completed 记录，客户端可用原 Key 安全重试）。
type IdemExecutor struct {
	Svc *idempotency.Service
}

// IdemFunc 是一次业务执行：rec.ID 用于幂等完成记录与文件 Owner。
type IdemFunc func(ctx context.Context, rec *idempotency.Record) (status int, body any, err error)

// HashRequest 计算请求指纹：method + 路由模板 + 规范化 JSON 文本
// （键排序由 encoding/json 保证）+ 可选内容 hash（上传文件 SHA-256）。
func HashRequest(method, routeTemplate string, canonicalBody []byte, contentSHA256 string) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write([]byte(routeTemplate))
	h.Write([]byte{0})
	h.Write(canonicalBody)
	h.Write([]byte{0})
	h.Write([]byte(contentSHA256))
	return hex.EncodeToString(h.Sum(nil))
}

// CanonicalJSON 把绑定后的请求体重编码为规范化 JSON（map 键排序）。
func CanonicalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// Run 执行幂等流程并直接写出响应；调用方在 fn 返回后不再写响应。
func (x *IdemExecutor) Run(c *gin.Context, scope, requestHash string, fn IdemFunc) {
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
	rec, outcome, err := x.Svc.Begin(c.Request.Context(), idempotency.BeginInput{
		Scope: scope, Key: key, Method: c.Request.Method,
		RouteTemplate: c.FullPath(), RequestHash: requestHash,
	})
	if err != nil {
		WriteError(c, err)
		return
	}
	switch outcome {
	case idempotency.OutcomeReplay:
		status := http.StatusOK
		if rec.ResponseStatus != nil {
			status = *rec.ResponseStatus
		}
		// 回放保存的响应头（如 Location）；只含 CompleteJSON 显式登记的头。
		var headers map[string]string
		if err := json.Unmarshal(rec.ResponseHeaders, &headers); err == nil {
			for k, v := range headers {
				c.Header(k, v)
			}
		}
		c.Data(status, "application/json", rec.ResponseBody)
		return
	case idempotency.OutcomeConflict:
		httpserver.AbortWithProblem(c, http.StatusConflict, "idempotency_conflict",
			"幂等键冲突", "相同 Idempotency-Key 携带了不同的请求内容。")
		return
	case idempotency.OutcomeInProgress:
		c.Header("Retry-After", "2")
		httpserver.AbortWithProblem(c, http.StatusConflict, "idempotency_in_progress",
			"请求处理中", "相同请求仍在处理中，请使用原 Idempotency-Key 稍后重试。")
		return
	}

	status, body, err := fn(c.Request.Context(), rec)
	if err != nil {
		// 事务前失败：删除 in_progress 记录，允许安全重试。
		_ = x.Svc.Abandon(context.Background(), rec.ID)
		WriteError(c, err)
		return
	}
	c.JSON(status, body)
}

// CompleteJSON 在业务事务内完成幂等记录（03 §6.5 规则 4）。
// response_body 只含创建/任务响应的非敏感 JSON；headers 为需随回放返回的
// 响应头（如 Location），可为 nil。
func CompleteJSON(ctx context.Context, tx database.Querier, svc *idempotency.Service,
	recID uuid.UUID, status int, body any, headers map[string]string, resourceType *string, resourceID, jobID *uuid.UUID) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var headerRaw []byte
	if len(headers) > 0 {
		if headerRaw, err = json.Marshal(headers); err != nil {
			return err
		}
	}
	return svc.Complete(ctx, tx, recID, status, raw, headerRaw, resourceType, resourceID, jobID)
}
