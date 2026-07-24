package legacyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Scan inventories source without opening it for writing or touching database
// state. It rejects symlinks instead of resolving them: the importer never
// follows a path which might escape the read-only /legacy mount.
func Scan(ctx context.Context, sourceRoot string) (*Manifest, error) {
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve source root: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("stat source root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: source root must be a real directory", ErrUnsafeSource)
	}

	m := &Manifest{SchemaVersion: manifestSchemaVersion, ImporterVersion: importerVersion}
	var unsafe bool
	casePaths := map[string]string{}
	courses := map[string]struct{}{}
	lessons := map[string]lessonIdentity{}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			unsafe = true
			m.Findings = append(m.Findings, Finding{Code: "path_escape", RelativePath: safeRelative(rel), Severity: "error"})
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&fs.ModeSymlink != 0 {
			unsafe = true
			m.Findings = append(m.Findings, Finding{Code: "path_escape", RelativePath: rel, Severity: "error"})
			return nil
		}
		if entry.IsDir() {
			if courseName, ok := courseFromPath(rel); ok {
				courses[courseName] = struct{}{}
			}
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			unsafe = true
			m.Findings = append(m.Findings, Finding{Code: "unsupported_source_entry", RelativePath: rel, Severity: "error"})
			return nil
		}
		lower := strings.ToLower(rel)
		if prior, ok := casePaths[lower]; ok && prior != rel {
			m.Findings = append(m.Findings, Finding{Code: "case_insensitive_path_conflict", RelativePath: rel, Severity: "error"})
		} else {
			casePaths[lower] = rel
		}
		sha, err := hashFile(path)
		if err != nil {
			return err
		}
		kind := classify(rel)
		parseStatus := "not_applicable"
		if strings.HasSuffix(strings.ToLower(rel), ".json") {
			parseStatus = jsonParseStatus(path)
			if parseStatus != "valid" {
				m.Findings = append(m.Findings, Finding{Code: "invalid_json", RelativePath: rel, Severity: "error"})
			}
		}
		m.Files = append(m.Files, SourceFile{RelativePath: rel, Size: fileInfo.Size(), SHA256: sha, Kind: kind, ParseStatus: parseStatus})
		m.Counts.Files++
		incrementCounts(&m.Counts, kind)
		if courseName, ok := courseFromPath(rel); ok {
			courses[courseName] = struct{}{}
		}
		if identity, ok := parseLessonPath(rel); ok {
			lessons[identity.key()] = identity
		}
		if kind == "lesson_info" {
			for _, finding := range inspectLessonInfo(path, rel) {
				m.Findings = append(m.Findings, finding)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan legacy source: %w", err)
	}
	m.Counts.Courses = len(courses)
	m.Counts.Lessons = len(lessons)
	for _, file := range m.Files {
		if file.Kind != "lesson_info" || file.ParseStatus != "valid" {
			continue
		}
		for _, finding := range compareLessonIdentity(root, file.RelativePath) {
			m.Findings = append(m.Findings, finding)
		}
	}
	m.normalize()
	m.SourceRootFingerprint = sourceFingerprint(m.Files)
	m.ManifestSHA256, err = manifestHash(*m)
	if err != nil {
		return nil, fmt.Errorf("hash manifest: %w", err)
	}
	if unsafe {
		return m, fmt.Errorf("%w: see manifest findings", ErrUnsafeSource)
	}
	return m, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func jsonParseStatus(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "invalid"
	}
	defer f.Close()
	var value any
	decoder := json.NewDecoder(f)
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "invalid"
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "invalid"
	}
	return "valid"
}

func sourceFingerprint(files []SourceFile) string {
	h := sha256.New()
	for _, file := range files {
		_, _ = io.WriteString(h, file.RelativePath+"\x00"+file.SHA256+"\x00"+strconv.FormatInt(file.Size, 10)+"\n")
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func classify(rel string) string {
	parts := strings.Split(rel, "/")
	base := strings.ToLower(filepath.Base(rel))
	switch {
	case strings.HasPrefix(rel, ".tasks/") && strings.HasSuffix(base, ".json"):
		return "legacy_task"
	case rel == "course/scores.json":
		return "scores"
	case rel == "course/weaknesses.json":
		return "weaknesses"
	case strings.Contains(rel, "/wrong_questions/") && strings.HasSuffix(base, ".json"):
		return "wrong_questions"
	case strings.HasSuffix(rel, "/info.json"):
		return "lesson_info"
	case strings.HasSuffix(rel, "/.subject"):
		return "course_subject"
	case strings.Contains(rel, "/student_notes/") && base == "content.md":
		return "student_notes"
	case strings.Contains(rel, "/teacher_notes/") && base == "content.md":
		return "teacher_outline"
	case strings.Contains(rel, "/raw/") || strings.Contains(rel, "/raw_src/"):
		if base == "readme.md" || base == "collected_info.txt" || base == "video_info.json" {
			return "raw_metadata"
		}
		return "raw_text"
	case strings.Contains(rel, "/exam/") && strings.HasSuffix(base, "_answers.json"):
		return "attempt_answers"
	case strings.Contains(rel, "/exam/") && (strings.HasSuffix(base, "_feedback.json") || strings.HasSuffix(base, "_grade_result.json")):
		return "attempt_feedback"
	case strings.Contains(rel, "/exam/") && strings.HasSuffix(base, ".json") && len(parts) >= 5:
		return "quiz"
	default:
		return "unknown"
	}
}

func incrementCounts(counts *Counts, kind string) {
	switch kind {
	case "quiz":
		counts.Quizzes++
	case "attempt_answers":
		counts.Attempts++
	case "wrong_questions":
		counts.WrongQuestionFiles++
	case "weaknesses":
		counts.Weaknesses++
	case "scores":
		counts.Scores++
	case "legacy_task":
		counts.Tasks++
	}
}

type lessonIdentity struct {
	course  string
	ordinal int
}

func (l lessonIdentity) key() string { return l.course + "/" + strconv.Itoa(l.ordinal) }

func courseFromPath(rel string) (string, bool) {
	parts := strings.Split(rel, "/")
	// course/scores.json and course/weaknesses.json are root aggregates, not
	// subject directories. A course is only established by a nested entry.
	if len(parts) < 3 || parts[0] != "course" || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}

func parseLessonPath(rel string) (lessonIdentity, bool) {
	parts := strings.Split(rel, "/")
	if len(parts) < 3 || parts[0] != "course" || !strings.HasPrefix(parts[2], "lesson_") {
		return lessonIdentity{}, false
	}
	ordinal, err := strconv.Atoi(strings.TrimPrefix(parts[2], "lesson_"))
	if err != nil || ordinal <= 0 {
		return lessonIdentity{}, false
	}
	return lessonIdentity{course: parts[1], ordinal: ordinal}, true
}

func inspectLessonInfo(path, rel string) []Finding {
	f, err := os.Open(path)
	if err != nil {
		return []Finding{{Code: "invalid_json", RelativePath: rel, Severity: "error"}}
	}
	defer f.Close()
	var value struct {
		Lesson  json.Number `json:"lesson"`
		Subject string      `json:"subject"`
	}
	decoder := json.NewDecoder(f)
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return []Finding{{Code: "invalid_json", RelativePath: rel, Severity: "error"}}
	}
	findings := []Finding{}
	identity, ok := parseLessonPath(rel)
	if !ok || strings.TrimSpace(value.Subject) == "" || value.Lesson == "" {
		return append(findings, Finding{Code: "unsupported_schema", RelativePath: rel, Severity: "error"})
	}
	n, err := strconv.Atoi(value.Lesson.String())
	if err != nil || n != identity.ordinal {
		findings = append(findings, Finding{Code: "lesson_number_conflict", RelativePath: rel, Severity: "error"})
	}
	return findings
}

func compareLessonIdentity(root, rel string) []Finding {
	identity, ok := parseLessonPath(rel)
	if !ok {
		return nil
	}
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return []Finding{{Code: "invalid_json", RelativePath: rel, Severity: "error"}}
	}
	defer f.Close()
	var value struct {
		Subject string `json:"subject"`
	}
	if err := json.NewDecoder(f).Decode(&value); err != nil {
		return nil
	}
	subjectPath := filepath.Join(root, "course", identity.course, ".subject")
	subject, err := os.ReadFile(subjectPath)
	if err != nil || strings.TrimSpace(string(subject)) == "" {
		return nil
	}
	if strings.TrimSpace(value.Subject) != strings.TrimSpace(string(subject)) {
		return []Finding{{Code: "course_identity_conflict", RelativePath: rel, Severity: "error"}}
	}
	return nil
}

func safeRelative(rel string) string {
	if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}
