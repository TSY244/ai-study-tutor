package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunScanWritesRedactedReportWithoutDatabase(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"course/math/.subject":            "Mathematics\n",
		"course/math/lesson_01/info.json": `{"subject":"Mathematics","lesson":1,"name":"Limits"}`,
		".tasks/pending/task.json":        `{"task_type":"grade_attempt","status":"pending","prompt":"DO NOT WRITE THIS"}`,
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	report := filepath.Join(t.TempDir(), "scan.json")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"scan", "--source", root, "--report", report}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run scan exit=%d stderr=%s", code, stderr.String())
	}
	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "DO NOT WRITE THIS") || !strings.Contains(string(body), "task.json") {
		t.Fatalf("report redaction failed: %s", body)
	}
}
