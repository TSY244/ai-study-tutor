package legacyimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"course/math/.subject":                                    "Mathematics\n",
		"course/math/lesson_01/info.json":                         `{"subject":"Mathematics","lesson":1,"name":"Limits"}`,
		"course/math/lesson_01/student_notes/content.md":          "# Limits\n",
		"course/math/lesson_01/teacher_notes/content.md":          "# Teaching notes\n",
		"course/math/lesson_01/raw_src/source_text.md":            "A sequence approaches a value.\n",
		"course/math/lesson_01/exam/quiz_chapter_01.json":         `{"id":"quiz_chapter_01","title":"Limits quiz","source":"regular","questions":[{"id":"q1","question":"Is a limit a trend?","type":"true_false","correct_answer":true,"explanation":"Yes.","tested_concepts":["limit"],"difficulty":"easy","cognitive_level":"recall","score":5}]}`,
		"course/math/lesson_01/exam/20260620_115723_answers.json": `{"quiz_id":"quiz_chapter_01","created_at":"20260620_115723","total_possible":5,"auto_score":5,"answers":[{"question_id":"q1","user_answer":true,"is_correct":true}],"ai_grading":{"status":"pending"}}`,
		".tasks/pending/task_grade.json":                          `{"task_id":"task_grade","task_type":"grade_attempt","subject":"Mathematics","lesson":1,"status":"pending","prompt":"TOP SECRET PROMPT","created_at":"2026-06-20T11:57:23Z","extra":{"quiz_id":"quiz_chapter_01","answers_path":"course/math/lesson_01/exam/20260620_115723_answers.json"}}`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScanIsReadOnlyAndRedactsLegacyTaskPrompt(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root)
	taskPath := filepath.Join(root, ".tasks/pending/task_grade.json")
	before, err := os.Stat(taskPath)
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	after, err := os.Stat(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("scan changed source mtime: before=%s after=%s", before.ModTime(), after.ModTime())
	}
	if manifest.Counts.Courses != 1 || manifest.Counts.Lessons != 1 || manifest.Counts.Tasks != 1 {
		t.Fatalf("unexpected counts: %+v", manifest.Counts)
	}
	encoded, err := manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "TOP SECRET PROMPT") {
		t.Fatalf("manifest leaked legacy task prompt: %s", encoded)
	}
	if !strings.Contains(string(encoded), "task_grade.json") {
		t.Fatalf("manifest omitted task audit record: %s", encoded)
	}
}

func TestScanRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root)
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "course/math/lesson_01/exam/escaped.json")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	manifest, err := Scan(context.Background(), root)
	if !errors.Is(err, ErrUnsafeSource) {
		t.Fatalf("Scan() error = %v, want ErrUnsafeSource", err)
	}
	if manifest == nil || !manifest.HasFinding("path_escape") {
		t.Fatalf("unsafe scan must still return a redacted finding: %#v", manifest)
	}
}

func TestScanDetectsLessonIdentityConflict(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root)
	if err := os.WriteFile(filepath.Join(root, "course/math/lesson_01/info.json"), []byte(`{"subject":"Physics","lesson":2,"name":"Limits"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !manifest.HasFinding("course_identity_conflict") || !manifest.HasFinding("lesson_number_conflict") {
		t.Fatalf("findings = %#v", manifest.Findings)
	}
}

func TestScanDoesNotTreatRootAggregateFilesAsCourses(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root)
	if err := os.WriteFile(filepath.Join(root, "course/scores.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "course/weaknesses.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Counts.Courses != 1 {
		t.Fatalf("courses=%d, want 1", manifest.Counts.Courses)
	}
}

func TestValidateResolutionAcceptsOnlyUniqueManifestFindings(t *testing.T) {
	manifest := Manifest{ManifestSHA256: "abc", Findings: []Finding{{Code: "lesson_number_conflict", RelativePath: "course/math/lesson_01/info.json", Severity: "error"}}}
	valid := &Resolution{SchemaVersion: 1, ManifestSHA256: "abc", AcceptedFindings: []ResolutionEntry{{Code: "lesson_number_conflict", RelativePath: "course/math/lesson_01/info.json"}}}
	if err := validateResolution(manifest, valid); err != nil {
		t.Fatalf("valid resolution error = %v", err)
	}
	unknown := &Resolution{SchemaVersion: 1, ManifestSHA256: "abc", AcceptedFindings: []ResolutionEntry{{Code: "unknown", RelativePath: "elsewhere"}}}
	if err := validateResolution(manifest, unknown); err == nil {
		t.Fatal("unknown resolution entry was accepted")
	}
	duplicate := &Resolution{SchemaVersion: 1, ManifestSHA256: "abc", AcceptedFindings: []ResolutionEntry{{Code: "lesson_number_conflict", RelativePath: "course/math/lesson_01/info.json"}, {Code: "lesson_number_conflict", RelativePath: "course/math/lesson_01/info.json"}}}
	if err := validateResolution(manifest, duplicate); err == nil {
		t.Fatal("duplicate resolution entry was accepted")
	}
}
