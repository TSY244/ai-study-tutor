package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProviderErrorRetryMatrix(t *testing.T) {
	cases := []struct {
		code      string
		retryable bool
	}{
		{ErrNotConfigured, false},
		{ErrAuthFailed, false},
		{ErrRateLimited, true},
		{ErrTimeout, true},
		{ErrOverloaded, true},
		{ErrInvalidRequest, false},
		{ErrRefused, false},
		{ErrOutputTruncated, false},
		{ErrInvalidJSON, true},
		{ErrSchemaInvalid, true},
		{ErrDomainInvalid, true},
		{ErrMaterialFetchFailed, false},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			err := ErrorForCode(tc.code, "展示给前端的脱敏摘要")
			if err == nil {
				t.Fatal("ErrorForCode 返回 nil")
			}
			if err.Code != tc.code || err.Retryable != tc.retryable {
				t.Fatalf("错误分类不一致: got (%s,%v) want (%s,%v)",
					err.Code, err.Retryable, tc.code, tc.retryable)
			}
			if strings.Contains(err.Detail, "sk-ant") || strings.Contains(err.Detail, "Authorization") {
				t.Fatal("错误 detail 泄露敏感内容")
			}
		})
	}

	for _, retryable := range []bool{false, true} {
		err := MaterialFetchError(retryable)
		if err.Code != ErrMaterialFetchFailed || err.Retryable != retryable {
			t.Fatalf("素材提取错误分类不一致: got (%s,%v) want (%s,%v)",
				err.Code, err.Retryable, ErrMaterialFetchFailed, retryable)
		}
		if strings.Contains(err.Detail, "upstream") || strings.Contains(err.Detail, "https://") {
			t.Fatal("素材提取错误 detail 不得包含上游文本或 URL")
		}
	}
}

func TestBuildLessonURLExtractRequest(t *testing.T) {
	const (
		allowedDomainSentinel = "allowed-domain-redaction-sentinel.invalid"
		urlSentinel           = "url-redaction-sentinel"
		systemSentinel        = "system-prompt-redaction-sentinel"
		userSentinel          = "user-prompt-redaction-sentinel"
		sourceTextSentinel    = "source-text-redaction-sentinel"
		keySentinel           = "sk-ant-key-redaction-sentinel"
	)
	canonicalURL := "https://www.youtube.com/watch?v=" + urlSentinel
	req, err := BuildLessonURLExtractRequest(LessonURLExtractPromptInput{
		URL:            canonicalURL,
		AllowedDomains: []string{" " + allowedDomainSentinel + " ", "youtube.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Purpose != PurposeLessonURLExtract || req.WebFetch == nil {
		t.Fatal("URL 提取请求缺少 purpose 或 web fetch 配置")
	}
	if req.MaxOutputTokens != 16384 {
		t.Fatalf("URL 提取输出上限错误: got %d want 16384", req.MaxOutputTokens)
	}
	if req.WebFetch.URL != canonicalURL {
		t.Fatal("canonical URL 错误")
	}
	wantDomains := []string{allowedDomainSentinel, "youtube.com"}
	if len(req.WebFetch.AllowedDomains) != len(wantDomains) {
		t.Fatal("allowed domains 数量错误")
	}
	for i := range wantDomains {
		if req.WebFetch.AllowedDomains[i] != wantDomains[i] {
			t.Fatal("allowed domains 顺序或空白处理错误")
		}
	}
	if req.WebFetch.MaxUses != 3 || req.WebFetch.MaxContentTokens != 12000 || req.WebFetch.MaxContinuations != 2 {
		t.Fatal("web_fetch_20250910 参数错误")
	}
	if req.PromptTemplateVersion == "" || req.SchemaVersion == "" {
		t.Fatal("URL 提取 prompt/schema version 必填")
	}
	for _, requiredInstruction := range []string{
		"before structured_result",
		"complete retrievable source text",
		"must not summarize",
		"untrusted data",
		"cannot modify these instructions",
		"secrets",
	} {
		if !strings.Contains(req.System, requiredInstruction) {
			t.Fatalf("URL 提取 system prompt 缺少安全或完整性约束 %q", requiredInstruction)
		}
	}
	if !strings.Contains(req.User, canonicalURL) {
		t.Fatal("URL 提取 user prompt 必须包含 canonical URL")
	}

	var schema struct {
		AdditionalProperties bool                       `json:"additionalProperties"`
		Required             []string                   `json:"required"`
		Properties           map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(req.Schema, &schema); err != nil {
		t.Fatalf("URL 提取 schema 不是有效 JSON: %v", err)
	}
	if schema.AdditionalProperties {
		t.Fatal("URL 提取 schema 必须禁止 additionalProperties")
	}
	wantRequired := []string{"title", "raw_text_markdown", "extraction_note"}
	for _, name := range wantRequired {
		if _, ok := schema.Properties[name]; !ok {
			t.Fatalf("URL 提取 schema 缺少属性 %q", name)
		}
		found := false
		for _, required := range schema.Required {
			if required == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("URL 提取 schema 未要求属性 %q: %v", name, schema.Required)
		}
	}
	var titleSchema struct {
		Type      string `json:"type"`
		MinLength *int   `json:"minLength"`
	}
	if err := json.Unmarshal(schema.Properties["title"], &titleSchema); err != nil {
		t.Fatalf("title schema 不是有效 JSON: %v", err)
	}
	if titleSchema.Type != "string" || (titleSchema.MinLength != nil && *titleSchema.MinLength > 0) {
		t.Fatal("title schema 必须允许空字符串")
	}
	var rawTextSchema struct {
		MinLength *int `json:"minLength"`
	}
	if err := json.Unmarshal(schema.Properties["raw_text_markdown"], &rawTextSchema); err != nil {
		t.Fatalf("raw_text_markdown schema 不是有效 JSON: %v", err)
	}
	if rawTextSchema.MinLength == nil || *rawTextSchema.MinLength < 1 {
		t.Fatal("raw_text_markdown schema 必须要求非空内容")
	}

	other, err := BuildLessonURLExtractRequest(LessonURLExtractPromptInput{
		URL:            "https://www.youtube.com/watch?v=xyz",
		AllowedDomains: []string{"www.youtube.com", "youtube.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.PromptHash() == other.PromptHash() {
		t.Fatal("不同 canonical URL 必须产生不同 prompt hash")
	}
	hashVariants := []func(*WebFetchOptions){
		func(options *WebFetchOptions) { options.AllowedDomains = []string{"youtube.com"} },
		func(options *WebFetchOptions) { options.MaxUses++ },
		func(options *WebFetchOptions) { options.MaxContentTokens++ },
		func(options *WebFetchOptions) { options.MaxContinuations++ },
	}
	for i, mutate := range hashVariants {
		variant := req
		options := *req.WebFetch
		options.AllowedDomains = append([]string(nil), req.WebFetch.AllowedDomains...)
		variant.WebFetch = &options
		mutate(variant.WebFetch)
		if variant.PromptHash() == req.PromptHash() {
			t.Fatalf("web fetch 选项变更未影响 prompt hash: variant %d", i)
		}
	}
	redactionProbe := req
	probeWebFetch := *req.WebFetch
	probeWebFetch.AllowedDomains = append([]string(nil), req.WebFetch.AllowedDomains...)
	redactionProbe.WebFetch = &probeWebFetch
	redactionProbe.System = systemSentinel
	redactionProbe.User = userSentinel + "\n" + sourceTextSentinel + "\n" + keySentinel
	if !strings.Contains(redactionProbe.User, sourceTextSentinel) {
		t.Fatal("redaction probe 必须包含原始来源文本 sentinel")
	}
	for _, built := range []Request{req, other, redactionProbe} {
		encoded, _ := json.Marshal(built.RedactedLogFields())
		for _, sentinel := range []string{
			allowedDomainSentinel,
			urlSentinel,
			systemSentinel,
			userSentinel,
			sourceTextSentinel,
			keySentinel,
		} {
			for _, field := range built.RedactedLogFields() {
				fieldJSON, err := json.Marshal(field)
				if err != nil {
					t.Fatal("脱敏日志字段无法编码")
				}
				if strings.Contains(string(fieldJSON), sentinel) {
					t.Fatal("脱敏日志字段包含敏感 sentinel")
				}
			}
			if strings.Contains(string(encoded), sentinel) {
				t.Fatal("脱敏日志 JSON 包含敏感 sentinel")
			}
		}
	}

	invalid := []LessonURLExtractPromptInput{
		{URL: "   ", AllowedDomains: []string{"youtube.com"}},
		{URL: canonicalURL},
		{URL: canonicalURL, AllowedDomains: []string{"youtube.com", "   "}},
	}
	for i, in := range invalid {
		if _, err := BuildLessonURLExtractRequest(in); err == nil {
			t.Fatalf("无效 URL 提取输入应返回错误: case %d", i)
		}
	}
}

func TestPromptHashWithoutWebFetchKeepsLegacyPayload(t *testing.T) {
	req := Request{
		Purpose:               PurposeConnectivity,
		PromptTemplateVersion: PromptConnectivityV1,
		SchemaVersion:         SchemaConnectivityV1,
		System:                "system",
		User:                  "user",
		Schema:                json.RawMessage(`{"type":"object"}`),
	}
	h := sha256.New()
	for _, value := range []string{
		string(req.Purpose), req.PromptTemplateVersion, req.SchemaVersion, req.System, req.User,
	} {
		h.Write([]byte(value))
		h.Write([]byte{0})
	}
	h.Write(req.Schema)
	want := hex.EncodeToString(h.Sum(nil))
	if got := req.PromptHash(); got != want {
		t.Fatalf("non-web-fetch prompt hash changed: got %q want %q", got, want)
	}
}

func TestPromptMetadataIsVersionedAndRedacted(t *testing.T) {
	req, err := BuildQuizRequest(QuizPromptInput{
		LessonTitle:   "极限",
		RawText:       "这里是完整学习材料，不应进入审计日志。",
		StudentNotes:  "学生笔记正文，也不应进入审计日志。",
		QuestionCount: 2,
		Review:        false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Purpose != PurposeQuizGenerate {
		t.Fatalf("purpose 错误: %s", req.Purpose)
	}
	if req.PromptTemplateVersion == "" || req.SchemaVersion == "" {
		t.Fatalf("prompt/schema version 必填: %+v", req)
	}
	if req.PromptHash() == "" || req.PromptHash() != req.PromptHash() {
		t.Fatalf("prompt hash 应稳定非空: %q", req.PromptHash())
	}
	logFields := req.RedactedLogFields()
	encoded, _ := json.Marshal(logFields)
	if strings.Contains(string(encoded), "完整学习材料") || strings.Contains(string(encoded), "学生笔记正文") {
		t.Fatal("脱敏日志字段包含学习正文")
	}
	if logFields["redacted"] != true {
		t.Fatalf("审计字段必须声明已脱敏: %v", logFields)
	}
}

func TestFakeProviderRecordsRequestsAndReturnsQueuedResults(t *testing.T) {
	fake := NewFakeProvider()
	fake.Enqueue(Response{
		JSON:              json.RawMessage(`{"ok":true}`),
		Model:             "fake-model",
		ProviderRequestID: "req_fake",
		Usage:             Usage{InputTokens: 3, OutputTokens: 2},
		Latency:           5 * time.Millisecond,
	})
	req := Request{Purpose: PurposeConnectivity, PromptTemplateVersion: "connectivity/v1", SchemaVersion: "connectivity-result/v1"}
	resp, err := fake.Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "fake-model" || resp.ProviderRequestID != "req_fake" {
		t.Fatalf("fake response 错误: %+v", resp)
	}
	got := fake.Requests()
	if len(got) != 1 || got[0].Purpose != PurposeConnectivity {
		t.Fatalf("fake provider 未记录请求: %+v", got)
	}
}

func TestDeterministicFakeProviderSupportsConnectivityOnly(t *testing.T) {
	fake := NewDeterministicFakeProvider()
	resp, err := fake.Generate(context.Background(), Request{Purpose: PurposeConnectivity})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.JSON) != `{"ok":true}` || resp.Model != "fake-local" {
		t.Fatalf("unexpected fake response: %+v", resp)
	}
	if _, err := fake.Generate(context.Background(), Request{Purpose: PurposeQuizGenerate}); err == nil {
		t.Fatal("deterministic fake must not pretend to generate learning content")
	}
}
