package cookiecheck

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/sitecookie"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

const ErrLoginExpired = "bilibili_login_expired"

type LoginHandler struct {
	Sessions *sitecookie.LoginSessions
	Proc     proc.Runner
	BiliBin  string
}

type loginEnvelope struct {
	OK   bool `json:"ok"`
	Data struct {
		LoggedIn  bool `json:"logged_in"`
		ExpiresIn int  `json:"expires_in"`
	} `json:"data"`
	Error struct {
		Code   string `json:"code"`
		Detail struct {
			Scanned bool `json:"scanned"`
		} `json:"detail"`
	} `json:"error"`
}

func (h *LoginHandler) Execute(ctx context.Context, ex *worker.Execution) (*worker.Outcome, error) {
	if err := ex.Stage(ctx, "generating_qrcode", 20, "生成 B 站登录二维码"); err != nil {
		return nil, err
	}
	if err := h.login(ctx, ex.Job.ID); err != nil {
		return nil, err
	}
	return &worker.Outcome{Publish: func(context.Context, pgx.Tx) ([]byte, error) {
		return []byte(`{"logged_in":true}`), nil
	}}, nil
}

func (h *LoginHandler) login(ctx context.Context, id uuid.UUID) (resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, 200*time.Second)
	defer cancel()
	defer func() {
		if resultErr != nil {
			_ = h.Sessions.Cleanup(id)
		}
	}()
	profile := h.Sessions.Profile(id)
	if err := os.MkdirAll(profile, 0o700); err != nil {
		return worker.NewError(worker.ErrCodeInternal, "无法创建扫码登录目录。", false)
	}
	bin := h.BiliBin
	if bin == "" {
		bin = "bili"
	}
	runner := h.Proc
	if runner == nil {
		runner = proc.ExecRunner{}
	}
	run := func(args ...string) (loginEnvelope, error) {
		res, err := runner.Run(ctx, proc.Cmd{Name: bin, Args: args, Dir: profile,
			Env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + profile, "BILI_CLI_HOME=" + profile, "NO_COLOR=1"}})
		var env loginEnvelope
		if ctx.Err() != nil {
			return env, ctx.Err()
		}
		var missing *proc.MissingToolError
		if errors.As(err, &missing) {
			return env, worker.NewError(worker.ErrCodeToolMissing, "扫码登录工具未安装，请重建 Worker 镜像。", false)
		}
		if res == nil || res.StdoutTruncated || json.Unmarshal(res.Stdout, &env) != nil {
			return env, worker.NewError(worker.ErrCodeBilibiliCheckFailed, "无法连接 B 站登录服务，请重新生成二维码。", false)
		}
		if env.OK && err != nil {
			return env, worker.NewError(worker.ErrCodeBilibiliCheckFailed, "登录工具执行失败，请重新生成二维码。", false)
		}
		return env, nil
	}
	env, err := run("auth", "login", "--qr-out", filepath.Join(profile, "login_qrcode.png"))
	if err != nil {
		return err
	}
	if !env.OK {
		return loginCommandError(env.Error.Code)
	}
	ttl := env.Data.ExpiresIn
	if ttl <= 0 || ttl > 180 {
		ttl = 180
	}
	expires := time.Now().Add(time.Duration(ttl) * time.Second)
	expiresText := expires.UTC().Format(time.RFC3339)
	for {
		st := sitecookie.LoginStatus{State: "waiting_scan", Message: "请用哔哩哔哩 App 扫码登录。", ExpiresAt: &expiresText}
		if env.Error.Detail.Scanned {
			st.State, st.Message = "scanned", "已扫码，请在手机上确认登录。"
		}
		if err := h.Sessions.Write(id, st); err != nil {
			return worker.NewError(worker.ErrCodeInternal, "保存扫码登录状态失败。", false)
		}
		if time.Now().After(expires) {
			return loginCommandError("login_expired")
		}
		env, err = run("auth", "status", "--wait", "2")
		if err != nil {
			return err
		}
		if env.OK && env.Data.LoggedIn {
			return h.Sessions.Write(id, sitecookie.LoginStatus{State: "confirmed", Message: "登录已确认，正在保存登录状态。"})
		}
		if env.Error.Code != "login_pending" {
			return loginCommandError(env.Error.Code)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func loginCommandError(code string) error {
	switch code {
	case "login_expired":
		return worker.NewError(ErrLoginExpired, "二维码已过期，请重新生成。", false)
	case "risk_control", "captcha_required":
		return worker.NewError(worker.ErrCodeBilibiliCheckFailed, "B 站要求人工验证，请在 B 站 App 中完成后重试。", false)
	default:
		return worker.NewError(worker.ErrCodeBilibiliCheckFailed, "B 站扫码登录失败，请重新生成二维码。", false)
	}
}

func (h *LoginHandler) FinalizeFail(_ context.Context, _ pgx.Tx, j *job.Job, _ string) error {
	return h.Sessions.Cleanup(j.ID)
}

func (h *LoginHandler) FinalizeCancel(_ context.Context, _ pgx.Tx, j *job.Job) error {
	return h.Sessions.Cleanup(j.ID)
}
