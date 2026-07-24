package legacyimport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

func TestImportIsIdempotentAndDoesNotResumeLegacyGradingTask(t *testing.T) {
	pool := databasetest.New(t)
	root := t.TempDir()
	writeFixture(t, root)
	svc := NewService(pool)

	first, err := svc.Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatalf("first Import() error = %v", err)
	}
	if first.Status != BatchCompleted || first.Reused {
		t.Fatalf("first report = %+v, want completed fresh batch", first)
	}
	if first.Records.Total != first.Manifest.Counts.Files || first.Records.Quarantined != 0 {
		t.Fatalf("reconciliation = %+v manifest=%+v", first.Records, first.Manifest.Counts)
	}

	var courses, lessons, documents, quizzes, attempts, jobs, records int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM courses`).Scan(&courses); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM lessons`).Scan(&lessons); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM documents`).Scan(&documents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM quizzes`).Scan(&quizzes); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM attempts`).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM legacy_import_records`).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if courses != 1 || lessons != 1 || documents != 3 || quizzes != 1 || attempts != 1 || jobs != 0 || records != first.Manifest.Counts.Files {
		t.Fatalf("unexpected import rows: courses=%d lessons=%d documents=%d quizzes=%d attempts=%d jobs=%d records=%d", courses, lessons, documents, quizzes, attempts, jobs, records)
	}
	var status, errorCode string
	if err := pool.QueryRow(context.Background(), `SELECT status, grading_error_code FROM attempts`).Scan(&status, &errorCode); err != nil {
		t.Fatal(err)
	}
	if status != "grading_failed" || errorCode != "legacy_task_not_resumable" {
		t.Fatalf("legacy pending grading mapped to status=%q error=%q", status, errorCode)
	}
	var reportText string
	if err := pool.QueryRow(context.Background(), `SELECT report::text FROM legacy_import_batches WHERE id=$1`, first.BatchID).Scan(&reportText); err != nil {
		t.Fatal(err)
	}
	if containsSecretPrompt(reportText) {
		t.Fatal("batch report retained legacy prompt")
	}

	second, err := svc.Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatalf("second Import() error = %v", err)
	}
	if !second.Reused || second.BatchID != first.BatchID || second.Status != BatchCompleted {
		t.Fatalf("second report = %+v, want reuse of %s", second, first.BatchID)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM attempts`).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("repeat import created attempts=%d, want 1", attempts)
	}
}

func TestImportQuarantinesIncompleteAttemptInsteadOfInventingIt(t *testing.T) {
	pool := databasetest.New(t)
	root := t.TempDir()
	writeFixture(t, root)
	badAnswers := filepath.Join(root, "course/math/lesson_01/exam/20260620_115723_answers.json")
	if err := osWriteFile(badAnswers, []byte(`{"quiz_id":"quiz_chapter_01","created_at":"20260620_115723","total_possible":5,"auto_score":0,"answers":[{"question_id":"missing","user_answer":true}]}`)); err != nil {
		t.Fatal(err)
	}

	report, err := NewService(pool).Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if report.Status != BatchNeedsReview || report.Records.Quarantined != 1 {
		t.Fatalf("report = %+v, want needs_review with one quarantine", report)
	}
	var attempts, quarantined int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM attempts`).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM legacy_import_records WHERE status='quarantined' AND reason_code='unknown_question'`).Scan(&quarantined); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || quarantined != 1 {
		t.Fatalf("incomplete attempt was imported=%d; quarantine records=%d", attempts, quarantined)
	}
}

func TestImportAcceptsEmptySubjectMarkerAsDirectoryIdentity(t *testing.T) {
	pool := databasetest.New(t)
	root := t.TempDir()
	writeFixture(t, root)
	if err := osWriteFile(filepath.Join(root, "course/math/.subject"), nil); err != nil {
		t.Fatal(err)
	}
	report, err := NewService(pool).Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	var courseTitle string
	if err := pool.QueryRow(context.Background(), `SELECT title FROM courses`).Scan(&courseTitle); err != nil {
		t.Fatal(err)
	}
	if report.Status != BatchCompleted || courseTitle != "math" {
		t.Fatalf("report=%+v course_title=%q, want completed directory identity", report, courseTitle)
	}
}

func TestImportResolutionCreatesHashBoundFollowUpBatch(t *testing.T) {
	pool := databasetest.New(t)
	root := t.TempDir()
	writeFixture(t, root)
	if err := osWriteFile(filepath.Join(root, "course/math/lesson_01/info.json"), []byte(`{"subject":"Physics","lesson":2,"name":"Limits"}`)); err != nil {
		t.Fatal(err)
	}
	manifest, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewService(pool).Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != BatchNeedsReview {
		t.Fatalf("first status=%q, want needs_review", first.Status)
	}
	resolution := &Resolution{SchemaVersion: 1, ManifestSHA256: manifest.ManifestSHA256}
	for _, finding := range manifest.Findings {
		resolution.AcceptedFindings = append(resolution.AcceptedFindings, ResolutionEntry{Code: finding.Code, RelativePath: finding.RelativePath})
	}
	second, err := NewService(pool).Import(context.Background(), root, ImportOptions{Resolution: resolution})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != BatchCompleted || second.Reused || second.BatchID == first.BatchID || second.ResolutionSHA256 == "" {
		t.Fatalf("resolution follow-up=%+v first=%+v", second, first)
	}
	var storedHash string
	if err := pool.QueryRow(context.Background(), `SELECT resolution_sha256 FROM legacy_import_batches WHERE id=$1`, second.BatchID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash != second.ResolutionSHA256 {
		t.Fatalf("stored resolution hash=%q report=%q", storedHash, second.ResolutionSHA256)
	}
}

func TestImportReconcilesWeaknessAndWrongAnswerWithoutInventingScores(t *testing.T) {
	pool := databasetest.New(t)
	root := t.TempDir()
	writeFixture(t, root)
	if err := osWriteFile(filepath.Join(root, "course/weaknesses.json"), []byte(`[
		{"course":"Mathematics","chapter":1,"concept":"Limit definition","weakness_description":"Needs review","evidence":"q1","severity":3,"first_seen":"2026-06-20T11:57:23Z","last_seen":"2026-06-20T11:57:23Z","review_count":1,"mastered":false}
	]`)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "course/math/lesson_01/wrong_questions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(root, "course/math/lesson_01/wrong_questions/20260620_115723_wrong.json"), []byte(`[
		{"question_id":"q1","user_answer":true,"correct_answer":true}
	]`)); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(root, "course/scores.json"), []byte(`[
		{"subject":"Mathematics","lesson":1,"score":5,"total":5,"timestamp":"2026-06-20 11:57:23"},
		{"subject":"Unknown course","lesson":1,"score":1,"total":1,"timestamp":"2026-06-20 11:57:23"}
	]`)); err != nil {
		t.Fatal(err)
	}

	report, err := NewService(pool).Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	var weaknesses, wrongMatches, unmatchedScores int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM weaknesses`).Scan(&weaknesses); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM legacy_import_records WHERE source_type='wrong_questions' AND status='imported'`).Scan(&wrongMatches); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM legacy_import_records WHERE source_type='scores' AND reason_code='unmatched_score'`).Scan(&unmatchedScores); err != nil {
		t.Fatal(err)
	}
	if report.Status != BatchNeedsReview || report.Records.Quarantined != 1 || weaknesses != 1 || wrongMatches != 1 || unmatchedScores != 1 {
		t.Fatalf("weaknesses=%d wrong_matches=%d unmatched_scores=%d", weaknesses, wrongMatches, unmatchedScores)
	}
}

func TestImportValidatesFeedbackAndNeverRevivesLegacyGradingFailure(t *testing.T) {
	pool := databasetest.New(t)
	root := t.TempDir()
	writeFixture(t, root)
	quizPath := filepath.Join(root, "course/math/lesson_01/exam/quiz_chapter_01.json")
	if err := osWriteFile(quizPath, []byte(`{"id":"quiz_chapter_01","title":"Limits quiz","source":"regular","questions":[{"id":"q1","question":"Explain a limit.","type":"short_answer","correct_answer":"A value approached.","explanation":"A limit is approached.","tested_concepts":["limit"],"difficulty":"easy","cognitive_level":"explain","score":5}]}`)); err != nil {
		t.Fatal(err)
	}
	answersPath := filepath.Join(root, "course/math/lesson_01/exam/20260620_115723_answers.json")
	if err := osWriteFile(answersPath, []byte(`{"quiz_id":"quiz_chapter_01","created_at":"20260620_115723","total_possible":5,"auto_score":0,"answers":[{"question_id":"q1","user_answer":"It approaches a value."}],"ai_grading":{"status":"completed"}}`)); err != nil {
		t.Fatal(err)
	}
	feedbackPath := filepath.Join(root, "course/math/lesson_01/exam/20260620_115723_feedback.json")
	if err := osWriteFile(feedbackPath, []byte(`{"quiz_id":"quiz_chapter_01","created_at":"2026-06-20T11:57:24Z","total_score":4,"question_feedback":[{"question_id":"q1","score":4,"is_correct":true,"feedback":"Accurate explanation."}]}`)); err != nil {
		t.Fatal(err)
	}

	report, err := NewService(pool).Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if report.Status != BatchCompleted || report.Records.Quarantined != 0 {
		t.Fatalf("report = %+v, want completed with no quarantines", report)
	}
	var status string
	var finalScore float64
	var feedback string
	if err := pool.QueryRow(context.Background(), `
		SELECT a.status, a.final_score, aa.feedback::text
		FROM attempts a JOIN attempt_answers aa ON aa.attempt_id=a.id`).Scan(&status, &finalScore, &feedback); err != nil {
		t.Fatal(err)
	}
	if status != "graded" || finalScore != 4 || !strings.Contains(feedback, "Accurate explanation.") {
		t.Fatalf("feedback mapping status=%q final=%v feedback=%s", status, finalScore, feedback)
	}

	if err := osWriteFile(answersPath, []byte(`{"quiz_id":"quiz_chapter_01","created_at":"20260620_115723","total_possible":5,"auto_score":0,"answers":[{"question_id":"q1","user_answer":"It approaches a value."}],"ai_grading":{"status":"pending"}}`)); err != nil {
		t.Fatal(err)
	}
	rootPending := t.TempDir()
	writeFixture(t, rootPending)
	if err := osWriteFile(filepath.Join(rootPending, "course/math/lesson_01/exam/quiz_chapter_01.json"), []byte(`{"id":"quiz_chapter_01","title":"Limits quiz","source":"regular","questions":[{"id":"q1","question":"Explain a limit.","type":"short_answer","correct_answer":"A value approached.","explanation":"A limit is approached.","tested_concepts":["limit"],"difficulty":"easy","cognitive_level":"explain","score":5}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(rootPending, "course/math/lesson_01/exam/20260620_115723_answers.json"), []byte(`{"quiz_id":"quiz_chapter_01","created_at":"20260620_115723","total_possible":5,"auto_score":0,"answers":[{"question_id":"q1","user_answer":"It approaches a value."}],"ai_grading":{"status":"pending"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(rootPending, "course/math/lesson_01/exam/20260620_115723_feedback.json"), []byte(`{"quiz_id":"quiz_chapter_01","created_at":"2026-06-20T11:57:24Z","total_score":4,"question_feedback":[{"question_id":"q1","score":4,"is_correct":true,"feedback":"Accurate explanation."}]}`)); err != nil {
		t.Fatal(err)
	}
	pendingPool := databasetest.New(t)
	pendingReport, err := NewService(pendingPool).Import(context.Background(), rootPending, ImportOptions{})
	if err != nil {
		t.Fatalf("pending Import() error = %v", err)
	}
	var pendingStatus, pendingCode, feedbackReason string
	if err := pendingPool.QueryRow(context.Background(), `SELECT status, grading_error_code FROM attempts`).Scan(&pendingStatus, &pendingCode); err != nil {
		t.Fatal(err)
	}
	if err := pendingPool.QueryRow(context.Background(), `SELECT reason_code FROM legacy_import_records WHERE source_type='attempt_feedback'`).Scan(&feedbackReason); err != nil {
		t.Fatal(err)
	}
	if pendingReport.Status != BatchNeedsReview || pendingStatus != "grading_failed" || pendingCode != "legacy_task_not_resumable" || feedbackReason != "legacy_task_not_resumable" {
		t.Fatalf("pending report=%+v status=%q code=%q feedback_reason=%q", pendingReport, pendingStatus, pendingCode, feedbackReason)
	}
}

func TestActualLegacyDataPreCutoverImport(t *testing.T) {
	root := filepath.Join("..", "..", "legacy-data")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("legacy-data fixture is unavailable: %v", err)
	}
	pool := databasetest.New(t)
	report, err := NewService(pool).Import(context.Background(), root, ImportOptions{})
	if err != nil {
		t.Fatalf("actual legacy import error = %v", err)
	}
	t.Logf("pre-cutover batch=%s status=%s files=%d records=%d imported=%d deduplicated=%d quarantined=%d skipped=%d tasks=%d", report.BatchID, report.Status, report.Manifest.Counts.Files, report.Records.Total, report.Records.Imported, report.Records.Deduplicated, report.Records.Quarantined, report.Records.SkippedByPolicy, len(report.Tasks))
	rows, err := pool.Query(context.Background(), `SELECT target_type || ':' || COALESCE(reason_code, 'none'), count(*) FROM legacy_import_records WHERE batch_id=$1 GROUP BY 1 ORDER BY 1`, report.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var reasons []string
	for rows.Next() {
		var reason string
		var count int
		if err := rows.Scan(&reason, &count); err != nil {
			t.Fatal(err)
		}
		reasons = append(reasons, fmt.Sprintf("%s=%d", reason, count))
	}
	t.Logf("pre-cutover reason_counts=%s", strings.Join(reasons, ","))
	quarantined, err := pool.Query(context.Background(), `SELECT source_path, reason_code FROM legacy_import_records WHERE batch_id=$1 AND status='quarantined' ORDER BY source_path`, report.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	defer quarantined.Close()
	var isolated []string
	for quarantined.Next() {
		var sourcePath string
		var reason *string
		if err := quarantined.Scan(&sourcePath, &reason); err != nil {
			t.Fatal(err)
		}
		code := "none"
		if reason != nil {
			code = *reason
		}
		isolated = append(isolated, sourcePath+":"+code)
	}
	t.Logf("pre-cutover isolated_sources=%s", strings.Join(isolated, ","))
}

func containsSecretPrompt(report string) bool { return strings.Contains(report, "TOP SECRET PROMPT") }

func osWriteFile(path string, value []byte) error { return os.WriteFile(path, value, 0o600) }
