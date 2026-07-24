// Package legacyimport implements the one-way, audited import of the archived
// Python data directory. It deliberately has no HTTP surface.
package legacyimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
)

// BatchStatus is deliberately internal. Legacy batch state is not exposed by
// the browser API.
type BatchStatus string

const (
	BatchScanned     BatchStatus = "scanned"
	BatchImporting   BatchStatus = "importing"
	BatchNeedsReview BatchStatus = "needs_review"
	BatchCompleted   BatchStatus = "completed"
	BatchFailed      BatchStatus = "failed"
)

// ImportOptions controls the command-side policy. A resolution is an explicit
// audit artifact; it never causes an implicit overwrite of current data.
type ImportOptions struct {
	Resolution    *Resolution
	FailOnWarning bool
}

// Resolution lets an operator explicitly accept a scanner finding after
// reviewing its report. It contains no source text and is hash-bound to one
// manifest, so it cannot accidentally be replayed against new source data.
type Resolution struct {
	SchemaVersion    int               `json:"schema_version"`
	ManifestSHA256   string            `json:"manifest_sha256"`
	AcceptedFindings []ResolutionEntry `json:"accepted_findings"`
}

type ResolutionEntry struct {
	Code         string `json:"code"`
	RelativePath string `json:"relative_path"`
}

// Hash returns a stable identifier for the operator decision without retaining
// source bodies. Entry order is deliberately not meaningful.
func (r Resolution) Hash() (string, error) {
	entries := append([]ResolutionEntry(nil), r.AcceptedFindings...)
	if entries == nil {
		entries = []ResolutionEntry{}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RelativePath == entries[j].RelativePath {
			return entries[i].Code < entries[j].Code
		}
		return entries[i].RelativePath < entries[j].RelativePath
	})
	r.AcceptedFindings = entries
	body, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// RecordCounts is the reconciliation summary. Total must equal Manifest.Files
// before a batch can be marked completed.
type RecordCounts struct {
	Total           int `json:"total"`
	Imported        int `json:"imported"`
	Deduplicated    int `json:"deduplicated"`
	Quarantined     int `json:"quarantined"`
	SkippedByPolicy int `json:"skipped_by_policy"`
}

// TaskSummary is audit metadata for one archived task. Prompt, source_text,
// command, output paths and upstream response bodies are intentionally absent.
type TaskSummary struct {
	RelativePath string `json:"relative_path"`
	TaskType     string `json:"task_type"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// ImportReport is persisted in legacy_import_batches.report and emitted as the
// report JSON. It is safe to keep outside /legacy because it has no source
// bodies, answers, task prompts, credentials, or absolute paths.
type ImportReport struct {
	SchemaVersion    int           `json:"schema_version"`
	BatchID          uuid.UUID     `json:"batch_id"`
	Status           BatchStatus   `json:"status"`
	Reused           bool          `json:"reused"`
	ResolutionSHA256 string        `json:"resolution_sha256,omitempty"`
	Manifest         Manifest      `json:"manifest"`
	Records          RecordCounts  `json:"records"`
	Tasks            []TaskSummary `json:"tasks,omitempty"`
}

const (
	manifestSchemaVersion = 1
	importerVersion       = "legacy-import-v1"
)

var (
	// ErrUnsafeSource means the source tree contains a symlink or other entry
	// that cannot be proven to remain under the read-only source root.
	ErrUnsafeSource = errors.New("legacy import source is unsafe")
)

// Counts is the non-sensitive source inventory. It is used by reconciliation,
// never as a replacement for per-record audit rows.
type Counts struct {
	Courses            int `json:"courses"`
	Lessons            int `json:"lessons"`
	Files              int `json:"files"`
	Quizzes            int `json:"quizzes"`
	Attempts           int `json:"attempts"`
	WrongQuestionFiles int `json:"wrong_question_files"`
	Weaknesses         int `json:"weaknesses"`
	Scores             int `json:"scores"`
	Tasks              int `json:"tasks"`
}

// SourceFile contains only metadata safe to retain in a report. Content,
// prompts, answers, and absolute source paths must never be added here.
type SourceFile struct {
	RelativePath string `json:"relative_path"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	Kind         string `json:"kind"`
	ParseStatus  string `json:"parse_status"`
}

// Finding is a stable, compact conflict or warning. Detail is intentionally
// absent so reports cannot accidentally retain study material or prompts.
type Finding struct {
	Code         string `json:"code"`
	RelativePath string `json:"relative_path"`
	Severity     string `json:"severity"`
}

// Manifest is a deterministic, redacted inventory of a legacy source tree.
// SourceRootFingerprint and ManifestHash are content-derived and can therefore
// be used to replay a batch without exposing the source's local path.
type Manifest struct {
	SchemaVersion         int          `json:"schema_version"`
	ImporterVersion       string       `json:"importer_version"`
	SourceRootFingerprint string       `json:"source_root_fingerprint"`
	ManifestSHA256        string       `json:"manifest_sha256"`
	Counts                Counts       `json:"counts"`
	Files                 []SourceFile `json:"files"`
	Findings              []Finding    `json:"findings"`
}

func (m *Manifest) normalize() {
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].RelativePath < m.Files[j].RelativePath })
	sort.Slice(m.Findings, func(i, j int) bool {
		if m.Findings[i].RelativePath == m.Findings[j].RelativePath {
			return m.Findings[i].Code < m.Findings[j].Code
		}
		return m.Findings[i].RelativePath < m.Findings[j].RelativePath
	})
}

// JSON returns the report representation. The type does not contain source
// bodies by construction, so callers may persist this value as audit metadata.
func (m Manifest) JSON() ([]byte, error) {
	m.normalize()
	return json.MarshalIndent(m, "", "  ")
}

// HasFinding reports whether a stable finding code is present.
func (m Manifest) HasFinding(code string) bool {
	for _, finding := range m.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func manifestHash(m Manifest) (string, error) {
	m.ManifestSHA256 = ""
	m.normalize()
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
