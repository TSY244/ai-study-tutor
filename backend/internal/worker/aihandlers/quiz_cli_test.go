package aihandlers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	proc "github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func TestBuildQuizCLIRequest(t *testing.T) {
	req := buildQuizCLIRequest("/tmp/job-1", quizCLIPromptInput{LessonTitle: "做空", QuestionCount: 8, Review: false})

	if req.Purpose != aipkg.PurposeQuizGenerate {
		t.Fatalf("purpose 错误: %s", req.Purpose)
	}
	if req.PromptTemplateVersion != aipkg.PromptQuizCLIV1 {
		t.Fatalf("prompt 版本错误: %s", req.PromptTemplateVersion)
	}
	if req.SchemaVersion != aipkg.SchemaQuizV1 {
		t.Fatalf("schema 版本错误: %s", req.SchemaVersion)
	}
	for _, want := range []string{
		"/tmp/job-1/raw_text.md", "/tmp/job-1/student_notes.md", "/tmp/job-1/quiz.json",
		"《做空》", "8 道常规", "不可信数据", "single_choice", "multiple_choice", "cognitive_level",
		"challenge", "reflect", "python3 -m json.tool", "全角引号",
	} {
		if !strings.Contains(req.User, want) {
			t.Fatalf("prompt 应包含 %q:\n%s", want, req.User)
		}
	}
}

func TestBuildQuizCLIRequestDefaultsQuestionCount(t *testing.T) {
	req := buildQuizCLIRequest("/tmp/job-1", quizCLIPromptInput{LessonTitle: "做空", QuestionCount: 0})
	if !strings.Contains(req.User, "8 道常规") {
		t.Fatalf("QuestionCount<=0 应默认 8 道:\n%s", req.User)
	}
}

func TestBuildQuizCLIRequestReview(t *testing.T) {
	req := buildQuizCLIRequest("/tmp/job-2", quizCLIPromptInput{LessonTitle: "做空", QuestionCount: 5, Review: true})
	if req.Purpose != aipkg.PurposeReviewQuizGenerate {
		t.Fatalf("purpose 错误: %s", req.Purpose)
	}
	if req.PromptTemplateVersion != aipkg.PromptReviewQuizCLIV1 {
		t.Fatalf("prompt 版本错误: %s", req.PromptTemplateVersion)
	}
	if !strings.Contains(req.User, "5 道复习") {
		t.Fatalf("复习模式题数文案错误:\n%s", req.User)
	}
}

const quizCLIKeySentinel = "sk-ant-QUIZ-CLI-KEY-SENTINEL-7b31"

func writeQuizCLIGatewayKeys(t *testing.T, keys ...map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"keys": keys})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func quizCLITestKey(name, apiKey string, priority, weight int, enabled bool) map[string]any {
	return map[string]any{
		"id": "ck_" + name, "name": name, "api_key": apiKey, "model": "claude-quiz-cli-test",
		"enabled": enabled, "priority": priority, "weight": weight,
	}
}

// quizCLIEnvelopeJSON 生成 `claude -p --output-format json` 的信封 stdout。
func quizCLIEnvelopeJSON(t *testing.T, env map[string]any) *proc.Result {
	t.Helper()
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return &proc.Result{Stdout: raw}
}

func newQuizCLIGenerator(keysFile string, runner proc.Runner) *ClaudeQuizCLIGenerator {
	return &ClaudeQuizCLIGenerator{
		Proc: runner, KeysFile: keysFile,
		BaseURL: "https://gateway.example", Home: "/var/lib/ai-study-tutor",
		ClaudeBin: "claude", Timeout: time.Minute,
	}
}

const quizCLIGoodJSON = `{"title":"测验","questions":[{"type":"single_choice","stem":"题","options":{"A":"x","B":"y"},"correct_answer":"A","explanation":null,"tested_concepts":["c"],"difficulty":"easy","cognitive_level":"recall","score":5}]}`

// writeQuizCLIOutput 模拟 agent 按文件契约落盘 quiz.json。
func writeQuizCLIOutput(t *testing.T, res *proc.Result) proc.RespondFunc {
	t.Helper()
	return func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if err := os.WriteFile(filepath.Join(cmd.Dir, quizCLIOutputFile), []byte(quizCLIGoodJSON), 0o600); err != nil {
			return nil, err
		}
		return res, nil
	}
}

func TestQuizCLIGenerateSuccess(t *testing.T) {
	keysFile := writeQuizCLIGatewayKeys(t,
		quizCLITestKey("disabled", "sk-ant-disabled", 1, 100, false),
		quizCLITestKey("low-prio", "sk-ant-low", 20, 100, true),
		quizCLITestKey("winner", quizCLIKeySentinel, 5, 200, true),
	)
	fake := proc.NewFake()
	fake.Enqueue(writeQuizCLIOutput(t, quizCLIEnvelopeJSON(t, map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"result": "done", "duration_ms": 42000, "session_id": "sess_quiz_1",
		"usage": map[string]any{"input_tokens": 100, "output_tokens": 2000,
			"cache_read_input_tokens": 30, "cache_creation_input_tokens": 40},
	})))
	g := newQuizCLIGenerator(keysFile, fake)
	req := buildQuizCLIRequest("/wd", quizCLIPromptInput{LessonTitle: "做空", QuestionCount: 8})

	resp, err := g.Generate(context.Background(), t.TempDir(), req)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.JSON) != quizCLIGoodJSON {
		t.Fatalf("应返回 quiz.json 内容: %s", resp.JSON)
	}
	if resp.Model != "claude-quiz-cli-test" || resp.ProviderRequestID != "sess_quiz_1" {
		t.Fatalf("model/session 映射错误: %+v", resp)
	}
	if resp.Usage.InputTokens != 100 || resp.Usage.OutputTokens != 2000 ||
		resp.Usage.CacheReadTokens != 30 || resp.Usage.CacheWriteTokens != 40 {
		t.Fatalf("usage 映射错误: %+v", resp.Usage)
	}
	if resp.Latency != 42*time.Second {
		t.Fatalf("latency 映射错误: %v", resp.Latency)
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("应恰好一次 CLI 调用: %d", len(calls))
	}
	cmd := calls[0]
	wantArgs := []string{"--allowedTools", quizCLIAllowedTools, "-p", req.User, "--output-format", "json"}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("argv 错误: %v", cmd.Args)
	}
	for i := range wantArgs {
		if cmd.Args[i] != wantArgs[i] {
			t.Fatalf("argv[%d] 错误: got %q want %q", i, cmd.Args[i], wantArgs[i])
		}
	}
	// env 恰好是白名单变量；api_key 来自最高优先级启用 Key。
	wantEnv := map[string]bool{
		"HOME=/var/lib/ai-study-tutor": true,
		"SHELL=/bin/bash":              true,
		"CI=1":                         true,
		"NO_COLOR=1":                   true,
		"ANTHROPIC_AUTH_TOKEN=" + quizCLIKeySentinel: true,
		"ANTHROPIC_BASE_URL=https://gateway.example": true,
	}
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "PATH=") {
			continue
		}
		if !wantEnv[e] {
			t.Fatalf("env 出现非白名单变量: %s", e)
		}
		delete(wantEnv, e)
	}
	if len(wantEnv) != 0 {
		t.Fatalf("env 缺少变量: %v", wantEnv)
	}
}

func TestQuizCLIGenerateErrorMapping(t *testing.T) {
	keysFile := writeQuizCLIGatewayKeys(t, quizCLITestKey("k", quizCLIKeySentinel, 1, 100, true))
	goodEnv := map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": "done"}

	cases := []struct {
		name          string
		respond       proc.RespondFunc
		wantCode      string
		wantRetryable bool
	}{
		{"missing_quiz_json", proc.Static(quizCLIEnvelopeJSON(t, goodEnv), nil), aipkg.ErrInvalidJSON, true},
		{"invalid_quiz_json", func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
			if err := os.WriteFile(filepath.Join(cmd.Dir, quizCLIOutputFile), []byte("{broken"), 0o600); err != nil {
				return nil, err
			}
			return quizCLIEnvelopeJSON(t, goodEnv), nil
		}, aipkg.ErrInvalidJSON, true},
		{"auth_401", proc.Static(quizCLIEnvelopeJSON(t, map[string]any{"type": "result", "subtype": "error", "is_error": true, "api_error_status": 401}), nil), aipkg.ErrAuthFailed, false},
		{"auth_403", proc.Static(quizCLIEnvelopeJSON(t, map[string]any{"type": "result", "subtype": "error", "is_error": true, "api_error_status": 403}), nil), aipkg.ErrAuthFailed, false},
		{"rate_429", proc.Static(quizCLIEnvelopeJSON(t, map[string]any{"type": "result", "subtype": "error", "is_error": true, "api_error_status": 429}), nil), aipkg.ErrRateLimited, true},
		{"server_500", proc.Static(quizCLIEnvelopeJSON(t, map[string]any{"type": "result", "subtype": "error", "is_error": true, "api_error_status": 500}), nil), aipkg.ErrUnavailable, true},
		{"is_error_envelope", proc.Static(quizCLIEnvelopeJSON(t, map[string]any{"type": "result", "subtype": "error", "is_error": true, "result": "上游错误 " + quizCLIKeySentinel}), nil), aipkg.ErrUnavailable, true},
		{"exit_nonzero_no_envelope", proc.Static(&proc.Result{Stdout: []byte("not json")}, &proc.ExitError{Code: 1, StderrTail: "boom " + quizCLIKeySentinel}), aipkg.ErrUnavailable, true},
		{"exit_nonzero_envelope_401", proc.Static(quizCLIEnvelopeJSON(t, map[string]any{"type": "result", "subtype": "error", "is_error": true, "api_error_status": 401}), &proc.ExitError{Code: 1, StderrTail: "boom"}), aipkg.ErrAuthFailed, false},
		{"garbage_stdout", proc.Static(&proc.Result{Stdout: []byte("not json " + quizCLIKeySentinel)}, nil), aipkg.ErrUnavailable, true},
		{"missing_cli", proc.Static(nil, &proc.MissingToolError{Tool: "claude"}), aipkg.ErrNotConfigured, false},
		{"timeout", proc.BlockUntilDone(), aipkg.ErrTimeout, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := proc.NewFake()
			fake.Enqueue(tc.respond)
			g := newQuizCLIGenerator(keysFile, fake)
			if tc.name == "timeout" {
				g.Timeout = 50 * time.Millisecond
			}
			req := buildQuizCLIRequest("/wd", quizCLIPromptInput{LessonTitle: "t", QuestionCount: 1})
			_, err := g.Generate(context.Background(), t.TempDir(), req)
			if err == nil {
				t.Fatal("应返回错误")
			}
			perr := aipkg.AsProviderError(err)
			if perr.Code != tc.wantCode || perr.Retryable != tc.wantRetryable {
				t.Fatalf("code=%s retryable=%v, want %s/%v", perr.Code, perr.Retryable, tc.wantCode, tc.wantRetryable)
			}
			if strings.Contains(perr.Detail, quizCLIKeySentinel) || strings.Contains(err.Error(), quizCLIKeySentinel) {
				t.Fatalf("error 泄漏 api_key: %v", err)
			}
		})
	}
}

func TestQuizCLIGenerateNoEnabledKey(t *testing.T) {
	keysFile := writeQuizCLIGatewayKeys(t, quizCLITestKey("off", "sk-ant-x", 1, 100, false))
	fake := proc.NewFake()
	g := newQuizCLIGenerator(keysFile, fake)
	req := buildQuizCLIRequest("/wd", quizCLIPromptInput{LessonTitle: "t", QuestionCount: 1})
	_, err := g.Generate(context.Background(), t.TempDir(), req)
	perr := aipkg.AsProviderError(err)
	if perr.Code != aipkg.ErrNotConfigured || perr.Retryable {
		t.Fatalf("无启用 Key 应不可重试 claude_not_configured: %v", perr)
	}
	if len(fake.Calls()) != 0 {
		t.Fatal("无启用 Key 时不得执行 CLI")
	}
}

// TestQuizCLIGenerateJobCancel：父 ctx 取消优先于 CLI 错误映射（由
// worker 运行器归类为 job_timeout/cancelled）。
func TestQuizCLIGenerateJobCancel(t *testing.T) {
	keysFile := writeQuizCLIGatewayKeys(t, quizCLITestKey("winner", quizCLIKeySentinel, 10, 100, true))
	fake := proc.NewFake()
	fake.Enqueue(proc.BlockUntilDone())
	g := newQuizCLIGenerator(keysFile, fake)
	g.Timeout = time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	req := buildQuizCLIRequest("/wd", quizCLIPromptInput{LessonTitle: "t", QuestionCount: 1})
	_, err := g.Generate(ctx, t.TempDir(), req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("父 ctx 取消应原样返回: %v", err)
	}
}

// TestQuizCLIGenerateCorruptKeysFile：keys.json 损坏 -> 可重试
// claude_unavailable，不调用子进程。
func TestQuizCLIGenerateCorruptKeysFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := proc.NewFake()
	g := newQuizCLIGenerator(path, fake)
	req := buildQuizCLIRequest("/wd", quizCLIPromptInput{LessonTitle: "t", QuestionCount: 1})
	_, err := g.Generate(context.Background(), t.TempDir(), req)
	perr := aipkg.AsProviderError(err)
	if perr.Code != aipkg.ErrUnavailable || !perr.Retryable {
		t.Fatalf("损坏 keys.json 应可重试 claude_unavailable: %v", perr)
	}
	if len(fake.Calls()) != 0 {
		t.Fatal("keys.json 损坏时不得执行 CLI")
	}
}

// TestQuizCLIEnvBaseURLPrecedence：ANTHROPIC_BASE_URL 注入优先级 ——
// Key.base_url > 构造 BaseURL > 不注入。
func TestQuizCLIEnvBaseURLPrecedence(t *testing.T) {
	keyWithURL := quizCLITestKey("k", quizCLIKeySentinel, 1, 100, true)
	keyWithURL["base_url"] = "https://key-gw.example"
	cases := []struct {
		name        string
		key         map[string]any
		ctorBaseURL string
		wantEnv     string // 为空表示 env 不得含 ANTHROPIC_BASE_URL
	}{
		{"key_base_url_wins", keyWithURL, "https://gateway.example", "ANTHROPIC_BASE_URL=https://key-gw.example"},
		{"constructor_fallback", quizCLITestKey("k", quizCLIKeySentinel, 1, 100, true), "https://gateway.example", "ANTHROPIC_BASE_URL=https://gateway.example"},
		{"empty_no_injection", quizCLITestKey("k", quizCLIKeySentinel, 1, 100, true), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keysFile := writeQuizCLIGatewayKeys(t, tc.key)
			fake := proc.NewFake()
			fake.Enqueue(writeQuizCLIOutput(t, quizCLIEnvelopeJSON(t, map[string]any{
				"type": "result", "subtype": "success", "is_error": false, "result": "done",
			})))
			g := newQuizCLIGenerator(keysFile, fake)
			g.BaseURL = tc.ctorBaseURL
			req := buildQuizCLIRequest("/wd", quizCLIPromptInput{LessonTitle: "t", QuestionCount: 1})
			if _, err := g.Generate(context.Background(), t.TempDir(), req); err != nil {
				t.Fatal(err)
			}
			calls := fake.Calls()
			if len(calls) != 1 {
				t.Fatalf("应恰好一次 CLI 调用: %d", len(calls))
			}
			got := ""
			for _, e := range calls[0].Env {
				if strings.HasPrefix(e, "ANTHROPIC_BASE_URL=") {
					got = e
				}
			}
			if got != tc.wantEnv {
				t.Fatalf("ANTHROPIC_BASE_URL = %q, want %q (env=%v)", got, tc.wantEnv, calls[0].Env)
			}
		})
	}
}
