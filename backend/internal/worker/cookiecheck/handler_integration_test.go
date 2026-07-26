package cookiecheck_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/sitecookie"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/cookiecheck"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

const testSessdata = "integration-test-sessdata-0123456789abcdef"

type env struct {
	pool     *pgxpool.Pool
	runner   *worker.Runner
	store    *sitecookie.Store
	navCalls *atomic.Int32
	navSrv   *httptest.Server
}

func newEnv(t *testing.T, navStatus int, navBody string, cookieContent string) *env {
	t.Helper()
	pool := databasetest.New(t)
	files, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	navCalls := &atomic.Int32{}
	navSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		navCalls.Add(1)
		if navStatus != http.StatusOK {
			w.WriteHeader(navStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(navBody))
	}))
	t.Cleanup(navSrv.Close)

	cookiePath := filepath.Join(t.TempDir(), "cookies", "bilibili.txt")
	store := sitecookie.NewStore(cookiePath)
	if cookieContent != "" {
		if _, err := store.Save(cookieContent); err != nil {
			t.Fatalf("预置 Cookie 失败: %v", err)
		}
	}

	checker := sitecookie.NewChecker(navSrv.Client())
	checker.NavURL = navSrv.URL

	reg := worker.NewRegistry()
	reg.Register(job.KindBilibiliCookieTest, &cookiecheck.Handler{Store: store, Checker: checker})
	return &env{
		pool:  pool,
		store: store,
		runner: &worker.Runner{
			Pool: pool, Registry: reg, Files: files, WorkerID: "cookie-test-worker",
			Lease: 5 * time.Second, HeartbeatInterval: 200 * time.Millisecond,
			CancelPollInterval: 50 * time.Millisecond, PollInterval: 10 * time.Millisecond,
			Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		navCalls: navCalls,
		navSrv:   navSrv,
	}
}

func (e *env) runJob(t *testing.T) *job.Job {
	t.Helper()
	j := &job.Job{
		Kind: job.KindBilibiliCookieTest, AggregateType: "system", AggregateID: uuid.New(),
		Payload: []byte(`{}`), MaxAttempts: 1,
	}
	if err := (&job.Repository{}).Create(context.Background(), e.pool, j); err != nil {
		t.Fatal(err)
	}
	claimed, err := e.runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("应领取到一个任务")
	}
	got, err := (&job.Repository{}).GetByID(context.Background(), e.pool, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func netscapeWithExpires(expires string) string {
	return ".bilibili.com\tTRUE\t/\tTRUE\t" + expires + "\tSESSDATA\t" + testSessdata + "\n"
}

func TestCookieCheckSucceedsWithLoginInfo(t *testing.T) {
	e := newEnv(t, http.StatusOK,
		`{"code":0,"message":"0","data":{"isLogin":true,"mid":654321,"uname":"测试用户"}}`,
		netscapeWithExpires("1800601560"))

	j := e.runJob(t)
	if j.Status != job.StatusSucceeded {
		t.Fatalf("检测应成功: %s %v", j.Status, j.ErrorCode)
	}
	var result map[string]any
	if err := json.Unmarshal(j.Result, &result); err != nil {
		t.Fatalf("结果应为 JSON: %v", err)
	}
	if result["status"] != "ok" || result["login"] != true || result["uname"] != "测试用户" {
		t.Fatalf("结果不含登录信息: %v", result)
	}
	if strings.Contains(string(j.Result), testSessdata) {
		t.Fatal("Job 结果绝不包含 SESSDATA")
	}
	if e.navCalls.Load() != 1 {
		t.Fatalf("应恰好调用一次 nav 端点, got %d", e.navCalls.Load())
	}
}

func TestCookieCheckNotConfigured(t *testing.T) {
	e := newEnv(t, http.StatusOK, `{"code":0,"data":{"isLogin":true}}`, "")

	j := e.runJob(t)
	if j.Status != job.StatusFailed {
		t.Fatalf("未配置应失败: %s", j.Status)
	}
	if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeBilibiliCookieNotConfigured {
		t.Fatalf("错误码应为 bilibili_cookie_not_configured, got %v", j.ErrorCode)
	}
	if j.ErrorRetryable == nil || *j.ErrorRetryable {
		t.Fatal("未配置应为不可重试")
	}
	if e.navCalls.Load() != 0 {
		t.Fatal("未配置时不应访问 B 站")
	}
}

func TestCookieCheckInvalidCookie(t *testing.T) {
	e := newEnv(t, http.StatusOK,
		`{"code":-101,"message":"账号未登录","data":{"isLogin":false}}`,
		netscapeWithExpires("1800601560"))

	j := e.runJob(t)
	if j.Status != job.StatusFailed {
		t.Fatalf("未登录应失败: %s", j.Status)
	}
	if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeBilibiliCookieInvalid {
		t.Fatalf("错误码应为 bilibili_cookie_invalid, got %v", j.ErrorCode)
	}
	if j.ErrorRetryable == nil || *j.ErrorRetryable {
		t.Fatal("Cookie 失效应为不可重试")
	}
	if j.ErrorDetail != nil && strings.Contains(*j.ErrorDetail, testSessdata) {
		t.Fatal("错误详情绝不包含 SESSDATA")
	}
}

func TestCookieCheckExpiredSkipsRemoteCall(t *testing.T) {
	e := newEnv(t, http.StatusOK, `{"code":0,"data":{"isLogin":true}}`,
		netscapeWithExpires("1700000000")) // 2023，已过期

	j := e.runJob(t)
	if j.Status != job.StatusFailed {
		t.Fatalf("过期应失败: %s", j.Status)
	}
	if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeBilibiliCookieExpired {
		t.Fatalf("错误码应为 bilibili_cookie_expired, got %v", j.ErrorCode)
	}
	if e.navCalls.Load() != 0 {
		t.Fatal("本地已过期时不应访问 B 站")
	}
}

func TestCookieCheckTransientFailureIsRetryable(t *testing.T) {
	e := newEnv(t, http.StatusPreconditionFailed, "", netscapeWithExpires("1800601560"))

	j := e.runJob(t)
	if j.Status != job.StatusFailed {
		t.Fatalf("反爬 412 应失败: %s", j.Status)
	}
	if j.ErrorCode == nil || *j.ErrorCode != worker.ErrCodeBilibiliCheckFailed {
		t.Fatalf("错误码应为 bilibili_check_failed, got %v", j.ErrorCode)
	}
	if j.ErrorRetryable == nil || !*j.ErrorRetryable {
		t.Fatal("远端暂时性失败应为可重试")
	}
}
