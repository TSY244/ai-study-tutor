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
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

const biliTestURL = "https://www.bilibili.com/video/BV1v34y1j7Nu?p=2"

func biliTestProfile(t *testing.T, cmd proc.Cmd) string {
	t.Helper()
	for _, kv := range cmd.Env {
		if strings.HasPrefix(kv, "BILI_CLI_HOME=") {
			return strings.TrimPrefix(kv, "BILI_CLI_HOME=")
		}
	}
	t.Fatal("missing isolated BILI_CLI_HOME")
	return ""
}

func biliTestEnvelope(t *testing.T, data any) *proc.Result {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"ok": true, "data": data})
	if err != nil {
		t.Fatal(err)
	}
	return &proc.Result{Stdout: raw}
}

func biliTestSheet(t *testing.T, profile string) string {
	t.Helper()
	path := filepath.Join(profile, "sheet.jpg")
	if err := os.WriteFile(path, []byte("test-image"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBilibiliExtractMergesFullTranscriptAndVisualSummary(t *testing.T) {
	keys := writeCLIGatewayKeys(t, cliTestKey("k", cliKeySentinel, 1, 1, true))
	cookie := filepath.Join(t.TempDir(), "bilibili.txt")
	const cookieSentinel = "COOKIE-SECRET-SENTINEL"
	if err := os.WriteFile(cookie, []byte("SESSDATA="+cookieSentinel+"; bili_jct=test-csrf; DedeUserID=42"), 0o600); err != nil {
		t.Fatal(err)
	}
	transcript := strings.Repeat("完整字幕不能被默认的六千字截断。\n", 800)
	fake := proc.NewFake()
	var profile string
	fake.Enqueue(func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if cmd.Name != "bili" || strings.Join(cmd.Args, " ") != "video watch BV1v34y1j7Nu --page 2 --frames 9 --comments 0 --no-danmaku --max-chars 0 --transcript auto" {
			t.Fatalf("wrong bili command: %s %v", cmd.Name, cmd.Args)
		}
		profile = biliTestProfile(t, cmd)
		credPath := filepath.Join(profile, "credential.json")
		raw, err := os.ReadFile(credPath)
		if err != nil {
			t.Fatal(err)
		}
		var cred map[string]string
		if err := json.Unmarshal(raw, &cred); err != nil || cred["sessdata"] != cookieSentinel || cred["dedeuserid"] != "42" {
			t.Fatal("cookie was not converted to bili credential")
		}
		stat, _ := os.Stat(credPath)
		if stat.Mode().Perm() != 0o600 {
			t.Fatal("credential must be private")
		}
		for _, v := range append(append([]string{}, cmd.Args...), cmd.Env...) {
			if strings.Contains(v, cookieSentinel) || strings.Contains(v, cliKeySentinel) {
				t.Fatal("bili process received secret in argv/env")
			}
		}
		return biliTestEnvelope(t, map[string]any{
			"title": "微积分第二讲", "transcript": map[string]string{"source": "subtitle:ai-zh", "text": transcript},
			"frames_sheet": biliTestSheet(t, profile), "next": "UNTRUSTED-NEXT-COMMAND",
		}), nil
	})
	fake.Enqueue(func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if _, err := os.Stat(profile); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("credential profile must be removed before Claude starts")
		}
		material, err := os.ReadFile(filepath.Join(cmd.Dir, "bilibili_material.json"))
		if err != nil || strings.Contains(string(material), "UNTRUSTED-NEXT-COMMAND") || strings.Contains(string(material), cookieSentinel) {
			t.Fatal("Claude material must only contain selected content fields")
		}
		if _, err := os.Stat(filepath.Join(cmd.Dir, "bilibili_frames.jpg")); err != nil {
			t.Fatal(err)
		}
		rawTranscript, err := os.ReadFile(filepath.Join(cmd.Dir, "bilibili_transcript.md"))
		if err != nil || string(rawTranscript) != transcript {
			t.Fatal("Claude must be able to read the full transcript from its own file")
		}
		prompt := strings.Join(cmd.Args, " ")
		for _, want := range []string{"read,write", "bilibili_frames.jpg", "as an IMAGE", "untrusted", "综合总结", "画面内容"} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("summary prompt missing %q", want)
			}
		}
		if strings.Contains(prompt, "yt-dlp") || strings.Contains(prompt, cookieSentinel) {
			t.Fatal("Bilibili must use local multimodal material")
		}
		return cliEnvelopeJSON(t, map[string]any{
			"type": "result", "subtype": "success", "session_id": "bili-summary",
			"result": `{"title":"model title","raw_text_markdown":"## 画面内容\n[01:20] 板书包含导数公式。\n\n## 综合总结\n导数描述局部变化率。\n\n### 内容概览\n围绕极限展开学习。\n\n### 核心知识点\n极限描述趋近关系。\n\n### 知识结构\n从直观图像到概念。\n\n### 示例与应用\n资料未提供示例。\n\n### 结论与复习\n复习：什么是极限？\n\n### 来源与局限\n依据给定资料，无其他来源。","extraction_note":"基于采样画面"}`,
		}), nil
	})
	x := newCLIExtractor(keys, fake)
	x.Home = t.TempDir()
	x.SiteCookiesFile = cookie
	resp, err := x.Extract(context.Background(), t.TempDir(), cliExtractRequest(t, biliTestURL))
	if err != nil {
		t.Fatal(err)
	}
	result, err := parseClaudeURLResult(resp.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "微积分第二讲" || !strings.Contains(result.RawTextMarkdown, transcript) || !strings.Contains(result.RawTextMarkdown, "导数描述局部变化率") || !strings.Contains(result.ExtractionNote, "subtitle:ai-zh") {
		t.Fatal("full transcript and visual summary must both reach downstream Markdown")
	}
	if len(fake.Calls()) != 2 || resp.ProviderRequestID != "bili-summary" {
		t.Fatal("expected bili acquisition followed by one Claude summary")
	}
	if raw, err := os.ReadFile(cookie); err != nil || !strings.Contains(string(raw), cookieSentinel) {
		t.Fatal("source cookie must remain unchanged")
	}
}

func TestBilibiliHDFallbackAndMissingTranscript(t *testing.T) {
	fake := proc.NewFake()
	fake.Enqueue(func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if _, err := os.Stat(filepath.Join(biliTestProfile(t, cmd), "credential.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("anonymous request should not inherit credentials")
		}
		return biliTestEnvelope(t, map[string]any{"title": "演示", "frames_sheet": nil}), nil
	})
	fake.Enqueue(func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if strings.Join(cmd.Args, " ") != "video frames BV1v34y1j7Nu --page 2 --quality hd --count 9" {
			t.Fatalf("HD fallback missing: %v", cmd.Args)
		}
		return biliTestEnvelope(t, map[string]any{"sheet": biliTestSheet(t, biliTestProfile(t, cmd))}), nil
	})
	x := &PiURLExtractor{Proc: fake, Home: t.TempDir()}
	b, err := x.prepareBilibili(context.Background(), t.TempDir(), biliTestURL)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := b.mergeSummary([]byte(`{"title":"演示","raw_text_markdown":"## 画面内容\n画面展示实验。\n## 综合总结\n实验现象。\n\n### 内容概览\n围绕极限展开学习。\n\n### 核心知识点\n极限描述趋近关系。\n\n### 知识结构\n从直观图像到概念。\n\n### 示例与应用\n资料未提供示例。\n\n### 结论与复习\n复习：什么是极限？\n\n### 来源与局限\n依据给定资料，无其他来源。"}`))
	if err != nil || !strings.Contains(string(payload), "未获取到字幕或语音转写") {
		t.Fatal("missing transcript must be explicitly identified")
	}
}

func TestBilibiliCommandErrorsAreRedactedAndClassified(t *testing.T) {
	for _, tc := range []struct {
		code      string
		wantCode  string
		retryable bool
	}{
		{"network_error", worker.ErrCodeMaterialFetchFailed, true},
		{"rate_limited", worker.ErrCodeMaterialFetchFailed, true},
		{"risk_control", worker.ErrCodeMaterialFetchFailed, false},
		{"captcha_required", worker.ErrCodeMaterialFetchFailed, false},
		{"auth_expired", worker.ErrCodeBilibiliCookieInvalid, false},
		{"not_found", worker.ErrCodeMaterialFetchFailed, false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			fake := proc.NewFake()
			var profile string
			fake.Enqueue(func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
				profile = biliTestProfile(t, cmd)
				return &proc.Result{Stdout: []byte(`{"ok":false,"error":{"code":"` + tc.code + `","message":"SECRET-UPSTREAM"}}`)}, &proc.ExitError{Code: 1, StderrTail: "SECRET-UPSTREAM"}
			})
			x := &PiURLExtractor{Proc: fake, Home: t.TempDir()}
			_, err := x.prepareBilibili(context.Background(), t.TempDir(), biliTestURL)
			p := aipkg.AsProviderError(err)
			if p == nil || p.Code != tc.wantCode || p.Retryable != tc.retryable || strings.Contains(err.Error(), "SECRET-UPSTREAM") {
				t.Fatalf("unexpected error classification: %v", err)
			}
			if _, err := os.Stat(profile); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed acquisition must also clean credentials")
			}
		})
	}
}

func TestBilibiliRejectsInvalidVideoAndPageWithoutExecuting(t *testing.T) {
	for _, path := range []string{"/", "/video/BV1v34y1j7Nu?p=0", "/video/BV1v34y1j7Nu?p=bad", "/video/BV1v34y1j7Nu?p=1&p=2", "/video/BV1v34y1j7Nu;echo%20secret"} {
		fake := proc.NewFake()
		x := &PiURLExtractor{Proc: fake}
		if _, err := x.prepareBilibili(context.Background(), t.TempDir(), "https://www.bilibili.com"+path); err == nil || len(fake.Calls()) != 0 {
			t.Fatalf("invalid video must fail before executing: %s", path)
		}
	}
	if isBilibiliURL("https://bilibili.com.evil.example/video/BV1v34y1j7Nu") {
		t.Fatal("hostname suffix spoofing must not select Bilibili")
	}
}

func TestBilibiliMissingToolAndCancellation(t *testing.T) {
	t.Run("missing tool", func(t *testing.T) {
		fake := proc.NewFake()
		fake.Enqueue(proc.Static(nil, &proc.MissingToolError{Tool: "bili"}))
		x := &PiURLExtractor{Proc: fake, Home: t.TempDir()}
		_, err := x.prepareBilibili(context.Background(), t.TempDir(), biliTestURL)
		if p := aipkg.AsProviderError(err); p.Code != worker.ErrCodeToolMissing || p.Retryable {
			t.Fatalf("wrong missing-tool classification: %v", err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		fake := proc.NewFake()
		fake.Enqueue(proc.BlockUntilDone())
		keys := writeCLIGatewayKeys(t, cliTestKey("k", cliKeySentinel, 1, 1, true))
		x := newCLIExtractor(keys, fake)
		x.Home = t.TempDir()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := x.Extract(ctx, t.TempDir(), cliExtractRequest(t, biliTestURL))
		if !errors.Is(err, context.DeadlineExceeded) || len(fake.Calls()) != 1 {
			t.Fatalf("parent cancellation must stop before Claude: %v", err)
		}
	})
}

func TestBilibiliDoesNotPublishMetadataOrIncompleteSummary(t *testing.T) {
	fake := proc.NewFake()
	fake.Enqueue(proc.Static(biliTestEnvelope(t, map[string]any{"title": "只有简介", "desc": "视频简介"}), nil))
	fake.Enqueue(proc.Static(biliTestEnvelope(t, map[string]any{"frames": []any{}}), nil))
	x := &PiURLExtractor{Proc: fake, Home: t.TempDir()}
	if _, err := x.prepareBilibili(context.Background(), t.TempDir(), biliTestURL); err == nil {
		t.Fatal("metadata without frames must never become a successful video summary")
	}
	b := &biliMaterial{Title: "视频"}
	if _, err := b.mergeSummary([]byte(`{"title":"视频","raw_text_markdown":"无法读取图片。"}`)); err == nil {
		t.Fatal("a failure message must not be published as a visual summary")
	}
}
