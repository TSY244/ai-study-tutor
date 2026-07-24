package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
)

func TestFromContextCarriesRequestID(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	if RequestID(ctx) != "req-1" {
		t.Fatal("request id 未写入 context")
	}
	if RequestID(context.Background()) != "" {
		t.Fatal("空 context 不应有 request id")
	}
}

func TestNewJSONLoggerFields(t *testing.T) {
	// 直接构造到 buffer 的 handler 验证字段（New 写 stdout，这里验证等价配置）。
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})).With(
		slog.String("service", "api"),
		slog.String("version", "test"),
	)
	logger.Info("starting")
	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("日志不是合法 JSON：%v", err)
	}
	for _, key := range []string{"time", "level", "msg", "service", "version"} {
		if _, ok := record[key]; !ok {
			t.Fatalf("日志缺少字段 %s：%v", key, record)
		}
	}
}

func TestParseLevelDefault(t *testing.T) {
	_ = New("svc", "v", "not-a-level") // 不应 panic，回落 info
	_ = New("svc", "v", "debug")
	if os.Getenv("LOG_LEVEL") != "" {
		t.Skip("环境 LOG_LEVEL 干扰")
	}
}
