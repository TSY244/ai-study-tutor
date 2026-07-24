package apitest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
)

func TestAPIExamplesMatchPublishedContract(t *testing.T) {
	contract := apitest.Load(t)
	root := repoRoot(t)

	tests := []struct {
		name         string
		relativePath string
		method       string
		pathTemplate string
		status       int
	}{
		{
			name:         "active jobs",
			relativePath: "api/examples/jobs/list-active.json",
			method:       "GET",
			pathTemplate: "/api/v1/jobs",
			status:       200,
		},
		{
			name:         "job events",
			relativePath: "api/examples/jobs/events.json",
			method:       "GET",
			pathTemplate: "/api/v1/jobs/{job_id}/events",
			status:       200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := readExample(t, root, tt.relativePath)
			contract.ValidateJSONResponse(t, tt.method, tt.pathTemplate, tt.status, raw)
		})
	}
}

func TestSSEExamplesMatchEnvelopeSchemas(t *testing.T) {
	contract := apitest.Load(t)
	root := repoRoot(t)

	tests := []struct {
		name          string
		relativePath  string
		schemaRef     string
		expectNoSSEID bool
	}{
		{
			name:         "persisted job progress",
			relativePath: "api/examples/sse/job-progress.event",
			schemaRef:    "#/components/schemas/PersistedJobStreamEvent",
		},
		{
			name:         "resync required",
			relativePath: "api/examples/sse/resync-required.event",
			schemaRef:    "#/components/schemas/JobResyncRequiredStreamEvent",
		},
		{
			name:          "heartbeat",
			relativePath:  "api/examples/sse/heartbeat.event",
			schemaRef:     "#/components/schemas/HeartbeatStreamEvent",
			expectNoSSEID: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := string(readExample(t, root, tt.relativePath))
			envelope := parseSSEExample(t, raw)
			if tt.expectNoSSEID {
				if _, ok := envelope["id"]; ok {
					t.Fatalf("heartbeat 示例不能携带 SSE id: %+v", envelope)
				}
			}
			body, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			contract.ValidateJSONSchema(t, tt.schemaRef, body)
		})
	}
}

func parseSSEExample(t *testing.T, raw string) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "id:"):
			var id int64
			if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "id:"))), &id); err != nil {
				t.Fatalf("SSE id 不是整数: %s", line)
			}
			out["id"] = id
		case strings.HasPrefix(line, "event:"):
			out["event"] = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			var data any
			if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &data); err != nil {
				t.Fatalf("SSE data 不是 JSON: %s", line)
			}
			out["data"] = data
		}
	}
	return out
}

func readExample(t *testing.T, root, relativePath string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}
