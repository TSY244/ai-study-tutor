package ai

import (
	"context"
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
				t.Fatalf("错误 detail 泄露敏感内容: %q", err.Detail)
			}
		})
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
		t.Fatalf("脱敏日志字段包含学习正文: %s", encoded)
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
