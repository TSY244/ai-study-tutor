package lessonimport

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
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

// cliKeySentinel 只准出现在子进程 env，绝不进入任何 error detail。
const cliKeySentinel = "sk-ant-CLI-KEY-SENTINEL-9f8e"

// writeCLIGatewayKeys 写一份 keys.json（与 claudegateway 磁盘 schema 同构）。
func writeCLIGatewayKeys(t *testing.T, keys ...map[string]any) string {
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

func cliTestKey(name, apiKey string, priority, weight int, enabled bool) map[string]any {
	return map[string]any{
		"id": "ck_" + name, "name": name, "api_key": apiKey, "model": "claude-cli-test",
		"enabled": enabled, "priority": priority, "weight": weight,
	}
}

func cliExtractRequest(t *testing.T, canonical string) aipkg.Request {
	t.Helper()
	req, err := aipkg.BuildLessonURLExtractRequest(aipkg.LessonURLExtractPromptInput{
		URL:            canonical,
		AllowedDomains: []string{"www.youtube.com", "youtube.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func cliEnvelopeJSON(t *testing.T, env map[string]any) *proc.Result {
	t.Helper()
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return &proc.Result{Stdout: raw}
}

func newCLIExtractor(keysFile string, runner proc.Runner) *ClaudeURLCLIExtractor {
	return &ClaudeURLCLIExtractor{
		Proc:      runner,
		KeysFile:  keysFile,
		BaseURL:   "https://gateway.example",
		Home:      "/var/lib/ai-study-tutor",
		ClaudeBin: "claude",
		Timeout:   time.Minute,
	}
}

// TestClaudeURLCLIExtractSuccess：成功信封 -> result/usage/latency/session
// 映射；argv 与工具白名单正确；prompt 含 canonical URL；env 恰好是白名单
// 变量且 api_key 取自最高优先级启用 Key。
func TestClaudeURLCLIExtractSuccess(t *testing.T) {
	keysFile := writeCLIGatewayKeys(t,
		cliTestKey("disabled", "sk-ant-disabled", 1, 100, false),
		cliTestKey("low-prio", "sk-ant-low", 20, 100, true),
		cliTestKey("winner", cliKeySentinel, 5, 200, true),
		cliTestKey("same-prio-lighter", "sk-ant-lighter", 5, 50, true),
	)
	fake := proc.NewFake()
	fake.Enqueue(proc.Static(cliEnvelopeJSON(t, map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"result":      `{"title":"极限入门","raw_text_markdown":"大家好","extraction_note":"含字幕"}`,
		"duration_ms": 42000, "session_id": "sess-123",
		"usage": map[string]any{
			"input_tokens": 100, "output_tokens": 200,
			"cache_read_input_tokens": 30, "cache_creation_input_tokens": 40,
		},
	}), nil))

	x := newCLIExtractor(keysFile, fake)
	resp, err := x.Extract(context.Background(), "/work/dir", cliExtractRequest(t, "https://www.youtube.com/watch?v=abc"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseClaudeURLResult(resp.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "极限入门" || res.RawTextMarkdown != "大家好" || res.ExtractionNote != "含字幕" {
		t.Fatalf("result 映射错误: %+v", res)
	}
	if resp.Usage.InputTokens != 100 || resp.Usage.OutputTokens != 200 ||
		resp.Usage.CacheReadTokens != 30 || resp.Usage.CacheWriteTokens != 40 {
		t.Fatalf("usage 映射错误: %+v", resp.Usage)
	}
	if resp.Latency != 42*time.Second {
		t.Fatalf("latency 映射错误: %v", resp.Latency)
	}
	if resp.ProviderRequestID != "sess-123" {
		t.Fatalf("session_id 应映射为 request id: %q", resp.ProviderRequestID)
	}
	if resp.Model != "claude-cli-test" {
		t.Fatalf("model 应取自选中 Key: %q", resp.Model)
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("应恰好一次子进程调用: %d", len(calls))
	}
	cmd := calls[0]
	if cmd.Name != "claude" || cmd.Dir != "/work/dir" {
		t.Fatalf("argv/工作目录错误: %+v", cmd)
	}
	joined := strings.Join(cmd.Args, "\x00")
	for _, want := range []string{"--allowedTools", "Bash Read Write WebFetch", "-p", "--output-format", "json"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv 缺少 %q: %v", want, cmd.Args)
		}
	}
	// prompt 含 canonical URL，且携带不可信数据与分隔标记输出约束。
	var prompt string
	for i, a := range cmd.Args {
		if a == "-p" && i+1 < len(cmd.Args) {
			prompt = cmd.Args[i+1]
		}
	}
	if !strings.Contains(prompt, "https://www.youtube.com/watch?v=abc") {
		t.Fatalf("prompt 应含 canonical URL: %q", prompt)
	}
	for _, want := range []string{"yt-dlp", "--skip-download", "untrusted", "<<<RAW_TEXT_MARKDOWN>>>"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt 缺少 %q: %q", want, prompt)
		}
	}

	// env 恰好是白名单变量；api_key 是优先级最高的启用 Key。
	wantKeys := map[string]bool{
		"HOME": false, "PATH": false, "CI": false, "NO_COLOR": false,
		"ANTHROPIC_BASE_URL": false, "ANTHROPIC_AUTH_TOKEN": false,
	}
	for _, kv := range cmd.Env {
		name, _, _ := strings.Cut(kv, "=")
		if _, ok := wantKeys[name]; !ok {
			t.Fatalf("env 含多余变量 %q", name)
		}
		wantKeys[name] = true
	}
	for name, seen := range wantKeys {
		if !seen {
			t.Fatalf("env 缺少 %s", name)
		}
	}
	if !containsEnv(cmd.Env, "HOME=/var/lib/ai-study-tutor") ||
		!containsEnv(cmd.Env, "CI=1") || !containsEnv(cmd.Env, "NO_COLOR=1") ||
		!containsEnv(cmd.Env, "ANTHROPIC_BASE_URL=https://gateway.example") ||
		!containsEnv(cmd.Env, "ANTHROPIC_AUTH_TOKEN="+cliKeySentinel) {
		t.Fatalf("env 值错误: %v", cmd.Env)
	}
}

func containsEnv(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}

// TestClaudeURLCLIExtractErrorMapping：信封/进程错误 -> 稳定错误码，且任何
// error detail 都不含 api_key 哨兵。
func TestClaudeURLCLIExtractErrorMapping(t *testing.T) {
	cases := []struct {
		name      string
		respond   proc.RespondFunc
		wantCode  string
		retryable bool
	}{
		{"is_error envelope", proc.Static(cliEnvelopeJSON(t, map[string]any{
			"type": "result", "subtype": "error", "is_error": true,
			"result": "upstream echoed " + cliKeySentinel,
		}), nil), aipkg.ErrMaterialFetchFailed, false},
		{"api 401", proc.Static(cliEnvelopeJSON(t, map[string]any{
			"type": "result", "subtype": "error", "is_error": true,
			"api_error_status": 401,
		}), nil), aipkg.ErrAuthFailed, false},
		{"api 403", proc.Static(cliEnvelopeJSON(t, map[string]any{
			"type": "result", "subtype": "error", "is_error": true,
			"api_error_status": 403,
		}), nil), aipkg.ErrAuthFailed, false},
		{"api 429", proc.Static(cliEnvelopeJSON(t, map[string]any{
			"type": "result", "subtype": "error", "is_error": true,
			"api_error_status": 429,
		}), nil), aipkg.ErrMaterialFetchFailed, true},
		{"api 500", proc.Static(cliEnvelopeJSON(t, map[string]any{
			"type": "result", "subtype": "error", "is_error": true,
			"api_error_status": 500,
		}), nil), aipkg.ErrMaterialFetchFailed, true},
		{"non-success subtype", proc.Static(cliEnvelopeJSON(t, map[string]any{
			"type": "result", "subtype": "error_max_turns", "is_error": false,
		}), nil), aipkg.ErrMaterialFetchFailed, false},
		{"malformed envelope", proc.Static(&proc.Result{
			Stdout: []byte("not json " + cliKeySentinel),
		}, nil), aipkg.ErrMaterialFetchFailed, false},
		{"exit non-zero", proc.Static(nil, &proc.ExitError{
			Code: 1, StderrTail: "boom " + cliKeySentinel,
		}), aipkg.ErrMaterialFetchFailed, false},
		{"missing cli", proc.Static(nil, &proc.MissingToolError{
			Tool: "claude",
		}), aipkg.ErrNotConfigured, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keysFile := writeCLIGatewayKeys(t, cliTestKey("winner", cliKeySentinel, 10, 100, true))
			fake := proc.NewFake()
			fake.Enqueue(tc.respond)
			x := newCLIExtractor(keysFile, fake)
			_, err := x.Extract(context.Background(), t.TempDir(), cliExtractRequest(t, "https://www.youtube.com/watch?v=abc"))
			if err == nil {
				t.Fatal("应返回错误")
			}
			perr := aipkg.AsProviderError(err)
			if perr.Code != tc.wantCode || perr.Retryable != tc.retryable {
				t.Fatalf("错误映射错误: code=%s retryable=%v, want %s/%v",
					perr.Code, perr.Retryable, tc.wantCode, tc.retryable)
			}
			if strings.Contains(perr.Detail, cliKeySentinel) || strings.Contains(err.Error(), cliKeySentinel) {
				t.Fatalf("error detail 泄漏 api_key: %v", err)
			}
		})
	}
}

// TestClaudeURLCLIExtractTimeout：CLI 自身超时（父 ctx 存活）-> 可重试
// material_fetch_failed。
func TestClaudeURLCLIExtractTimeout(t *testing.T) {
	keysFile := writeCLIGatewayKeys(t, cliTestKey("winner", cliKeySentinel, 10, 100, true))
	fake := proc.NewFake()
	fake.Enqueue(proc.BlockUntilDone())
	x := newCLIExtractor(keysFile, fake)
	x.Timeout = 50 * time.Millisecond

	_, err := x.Extract(context.Background(), t.TempDir(), cliExtractRequest(t, "https://www.youtube.com/watch?v=abc"))
	perr := aipkg.AsProviderError(err)
	if perr.Code != aipkg.ErrMaterialFetchFailed || !perr.Retryable {
		t.Fatalf("超时应为可重试 material_fetch_failed: %v", err)
	}
}

// TestClaudeURLCLIExtractJobCancel：父 ctx 取消优先于 CLI 错误映射（由
// worker 运行器归类为 job_timeout/cancelled）。
func TestClaudeURLCLIExtractJobCancel(t *testing.T) {
	keysFile := writeCLIGatewayKeys(t, cliTestKey("winner", cliKeySentinel, 10, 100, true))
	fake := proc.NewFake()
	fake.Enqueue(proc.BlockUntilDone())
	x := newCLIExtractor(keysFile, fake)
	x.Timeout = time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err := x.Extract(ctx, t.TempDir(), cliExtractRequest(t, "https://www.youtube.com/watch?v=abc"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("父 ctx 取消应原样返回: %v", err)
	}
}

// TestClaudeURLCLIExtractNoEnabledKey：keys.json 无启用 Key ->
// claude_not_configured，不调用子进程。
func TestClaudeURLCLIExtractNoEnabledKey(t *testing.T) {
	keysFile := writeCLIGatewayKeys(t, cliTestKey("disabled", cliKeySentinel, 10, 100, false))
	fake := proc.NewFake()
	x := newCLIExtractor(keysFile, fake)
	_, err := x.Extract(context.Background(), t.TempDir(), cliExtractRequest(t, "https://www.youtube.com/watch?v=abc"))
	if aipkg.AsProviderError(err).Code != aipkg.ErrNotConfigured {
		t.Fatalf("应 claude_not_configured: %v", err)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("无启用 Key 不应调用子进程: %+v", fake.Calls())
	}
}

// TestClaudeURLCLIExtractResultNotJSON：信封成功但 result 非 JSON -> 现有
// claude_invalid_json 路径（由 parseClaudeURLResult 归类，可重试）。
func TestClaudeURLCLIExtractResultNotJSON(t *testing.T) {
	keysFile := writeCLIGatewayKeys(t, cliTestKey("winner", cliKeySentinel, 10, 100, true))
	fake := proc.NewFake()
	fake.Enqueue(proc.Static(cliEnvelopeJSON(t, map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"result": "抓取失败，无法输出 JSON " + cliKeySentinel,
	}), nil))
	x := newCLIExtractor(keysFile, fake)
	resp, err := x.Extract(context.Background(), t.TempDir(), cliExtractRequest(t, "https://www.youtube.com/watch?v=abc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseClaudeURLResult(resp.JSON); aipkg.AsProviderError(err).Code != aipkg.ErrInvalidJSON {
		t.Fatalf("result 非 JSON 应 claude_invalid_json: %v", err)
	}
}

// TestStripCodeFence：容忍 ```json 代码围栏包裹。
func TestStripCodeFence(t *testing.T) {
	raw := "```json\n{\"title\":\"t\"}\n```"
	if got := stripCodeFence(raw); got != `{"title":"t"}` {
		t.Fatalf("围栏剥离错误: %q", got)
	}
	if got := stripCodeFence("  {\"title\":\"t\"}  "); got != `{"title":"t"}` {
		t.Fatalf("空白剥离错误: %q", got)
	}
}

// TestNormalizeCLIResultJSON：严格 JSON（含围栏）原样通过；分隔标记格式
// （含未转义引号/反斜杠的 Markdown，即模型产出非法 JSON 的真实失败模式）
// 重新编码为合法 JSON 并能通过 parseClaudeURLResult；两种格式都缺失时
// 原样返回，由 parseClaudeURLResult 归类 claude_invalid_json。
func TestNormalizeCLIResultJSON(t *testing.T) {
	strict := `{"title":"t","raw_text_markdown":"r","extraction_note":"n"}`
	if got := normalizeCLIResultJSON("```json\n" + strict + "\n```"); string(got) != strict {
		t.Fatalf("严格 JSON 应原样通过: %q", got)
	}

	marked := "<<<TITLE>>>\n定投完全攻略\n<<<RAW_TEXT_MARKDOWN>>>\n# 标题\n\n> \"让经济学·像玩游戏一样上头\"\n\n路径 C:\\videos\\a.mp4\n<<<EXTRACTION_NOTE>>>\n字幕不可获取，仅页面文本"
	got := normalizeCLIResultJSON(marked)
	res, err := parseClaudeURLResult(got)
	if err != nil {
		t.Fatalf("标记格式应解析成功: %v", err)
	}
	if res.Title != "定投完全攻略" ||
		res.RawTextMarkdown != "# 标题\n\n> \"让经济学·像玩游戏一样上头\"\n\n路径 C:\\videos\\a.mp4" ||
		res.ExtractionNote != "字幕不可获取，仅页面文本" {
		t.Fatalf("标记解析内容错误: %+v", res)
	}

	garbage := normalizeCLIResultJSON("抓取失败，无法输出")
	if _, err := parseClaudeURLResult(garbage); aipkg.AsProviderError(err).Code != aipkg.ErrInvalidJSON {
		t.Fatalf("无格式内容应 claude_invalid_json: %v", err)
	}
}

// TestClaudeURLCLIExtractMarkedResult：信封 result 为分隔标记格式时成功
// 提取并通过下游 schema 校验。
func TestClaudeURLCLIExtractMarkedResult(t *testing.T) {
	keysFile := writeCLIGatewayKeys(t, cliTestKey("winner", cliKeySentinel, 10, 100, true))
	fake := proc.NewFake()
	fake.Enqueue(proc.Static(cliEnvelopeJSON(t, map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"result":      "<<<TITLE>>>\n极限入门\n<<<RAW_TEXT_MARKDOWN>>>\n大家好，\"引用原文\" 不需要转义\n<<<EXTRACTION_NOTE>>>\n含字幕",
		"duration_ms": 42000, "session_id": "sess-marked",
		"usage": map[string]any{"input_tokens": 100, "output_tokens": 200},
	}), nil))
	x := newCLIExtractor(keysFile, fake)
	resp, err := x.Extract(context.Background(), t.TempDir(), cliExtractRequest(t, "https://www.youtube.com/watch?v=abc"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseClaudeURLResult(resp.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "极限入门" || res.RawTextMarkdown != "大家好，\"引用原文\" 不需要转义" || res.ExtractionNote != "含字幕" {
		t.Fatalf("标记格式 result 映射错误: %+v", res)
	}
}
