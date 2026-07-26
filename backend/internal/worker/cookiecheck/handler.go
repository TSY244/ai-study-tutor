// Package cookiecheck 实现 bilibili_cookie_test 任务：用已保存的 B 站
// Cookie 请求登录状态端点，验证 Cookie 可用性。检测在 Worker 进程执行
// （API 容器无外网 egress）；完整 Cookie 只在 Worker 内存与
// bilibili.txt 中存在，Job 结果/错误只含账号昵称等非敏感信息。
package cookiecheck

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/sitecookie"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
)

// Handler 是 bilibili_cookie_test 的 Job Handler。
type Handler struct {
	Store   *sitecookie.Store
	Checker *sitecookie.Checker
	Now     func() time.Time
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	if err := ex.Stage(ctx, "loading_cookie", 20, "读取 B 站 Cookie"); err != nil {
		return nil, err
	}
	f, err := h.Store.Load()
	if err != nil {
		switch {
		case errors.Is(err, sitecookie.ErrNotConfigured), errors.Is(err, sitecookie.ErrNotFound):
			return nil, worker.NewError(worker.ErrCodeBilibiliCookieNotConfigured,
				"尚未保存 B 站 Cookie。请先在设置中保存后重新检测。", false)
		default:
			return nil, worker.WrapError(worker.ErrCodeInternal, "读取 Cookie 文件失败，将自动重试。", true, err)
		}
	}
	sess, ok := f.Sessdata()
	if !ok {
		return nil, worker.NewError(worker.ErrCodeBilibiliCookieInvalid,
			"Cookie 文件缺少 SESSDATA。请在设置中重新保存。", false)
	}
	if sess.Expires > 0 && sess.Expires <= h.now().Unix() {
		return nil, worker.NewError(worker.ErrCodeBilibiliCookieExpired,
			"SESSDATA 已超过本地有效期。请重新获取并在设置中保存。", false)
	}

	if err := ex.Stage(ctx, "checking", 60, "检测 B 站登录状态"); err != nil {
		return nil, err
	}
	result, err := h.Checker.CheckBilibili(ctx, f)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		switch {
		case errors.Is(err, sitecookie.ErrCookieInvalid):
			return nil, worker.NewError(worker.ErrCodeBilibiliCookieInvalid,
				"B 站返回未登录：Cookie 已失效。请重新获取并在设置中保存。", false)
		default:
			var checkErr *sitecookie.CheckError
			if errors.As(err, &checkErr) {
				return nil, worker.WrapError(worker.ErrCodeBilibiliCheckFailed,
					"暂时无法连接 B 站完成检测，将自动重试。", true, err)
			}
			return nil, worker.WrapError(worker.ErrCodeInternal, "Cookie 检测失败，将自动重试。", true, err)
		}
	}

	if err := ex.Stage(ctx, "publishing", 90, "记录检测结果"); err != nil {
		return nil, err
	}
	return &worker.Outcome{Publish: func(context.Context, pgx.Tx) ([]byte, error) {
		payload := map[string]any{
			"status": "ok",
			"login":  true,
			"mid":    result.Mid,
			"uname":  result.Uname,
		}
		if sess.Expires > 0 {
			payload["sessdata_expires_at"] = time.Unix(sess.Expires, 0).UTC().Format(time.RFC3339)
		}
		return json.Marshal(payload)
	}}, nil
}

func (h *Handler) FinalizeFail(context.Context, pgx.Tx, *job.Job, string) error { return nil }
func (h *Handler) FinalizeCancel(context.Context, pgx.Tx, *job.Job) error       { return nil }
