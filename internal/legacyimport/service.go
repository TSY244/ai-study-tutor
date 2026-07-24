package legacyimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/course"
	"github.com/TSY244/ai-study-tutor/internal/document"
	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/mastery"
	"github.com/TSY244/ai-study-tutor/internal/quiz"
)

var (
	// ErrNeedsReview is returned only when the caller asks --fail-on-warning.
	// The report and reviewable batch have already been committed at that point.
	ErrNeedsReview = errors.New("legacy import requires review")
)

// Service is the application boundary for legacy import. It owns the explicit
// transaction containing business writes, audit rows, and reconciliation.
type Service struct {
	pool      *pgxpool.Pool
	repo      Repository
	courses   course.Repository
	lessons   lesson.Repository
	documents document.Repository
	quizzes   quiz.Repository
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool: pool, repo: Repository{}, courses: course.Repository{}, lessons: lesson.Repository{},
		documents: document.Repository{}, quizzes: quiz.Repository{},
	}
}

// Import scans again immediately before writing and then imports exactly that
// manifest. No API, Worker, or Claude adapter participates in this operation.
func (s *Service) Import(ctx context.Context, sourceRoot string, options ImportOptions) (*ImportReport, error) {
	manifest, scanErr := Scan(ctx, sourceRoot)
	if scanErr != nil {
		if manifest != nil {
			return &ImportReport{SchemaVersion: 1, Status: BatchFailed, Manifest: *manifest}, scanErr
		}
		return nil, scanErr
	}
	if err := validateResolution(*manifest, options.Resolution); err != nil {
		return &ImportReport{SchemaVersion: 1, Status: BatchFailed, Manifest: *manifest}, err
	}
	resolutionHash := ""
	if options.Resolution != nil {
		value, hashErr := options.Resolution.Hash()
		if hashErr != nil {
			return &ImportReport{SchemaVersion: 1, Status: BatchFailed, Manifest: *manifest}, hashErr
		}
		resolutionHash = value
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.repo.LockManifest(ctx, tx, manifest.ManifestSHA256); err != nil {
		return nil, fmt.Errorf("lock import manifest: %w", err)
	}
	if id, status, stored, found, err := s.repo.FindBatch(ctx, tx, manifest.ManifestSHA256, resolutionHash); err != nil {
		return nil, err
	} else if found {
		report := &ImportReport{SchemaVersion: 1, BatchID: id, Status: status, Reused: true, Manifest: *manifest}
		_ = json.Unmarshal(stored, report)
		report.BatchID, report.Status, report.Reused, report.Manifest, report.ResolutionSHA256 = id, status, true, *manifest, resolutionHash
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		if options.FailOnWarning && report.Status != BatchCompleted {
			return report, ErrNeedsReview
		}
		return report, nil
	}

	report := &ImportReport{SchemaVersion: 1, BatchID: uuid.New(), Status: BatchImporting, ResolutionSHA256: resolutionHash, Manifest: *manifest}
	if err := s.repo.CreateBatch(ctx, tx, report.BatchID, *manifest, resolutionHash); err != nil {
		return nil, fmt.Errorf("create import batch: %w", err)
	}
	state := newImportState()
	if err := s.importFiles(ctx, tx, sourceRoot, *manifest, report, state); err != nil {
		return nil, err
	}
	if err := s.repo.MarkSubmittedAttemptsNotResumable(ctx, tx, state.submittedAttempts); err != nil {
		return nil, err
	}
	if err := s.publishDocuments(ctx, tx, state); err != nil {
		return nil, err
	}

	if report.Records.Total != manifest.Counts.Files {
		return nil, fmt.Errorf("legacy import reconciliation mismatch: records=%d source_files=%d", report.Records.Total, manifest.Counts.Files)
	}
	if report.Records.Quarantined > 0 || hasUnacceptedFindings(*manifest, options.Resolution) {
		report.Status = BatchNeedsReview
	} else {
		report.Status = BatchCompleted
	}
	if err := s.repo.FinishBatch(ctx, tx, report.BatchID, report.Status, *report); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if options.FailOnWarning && report.Status != BatchCompleted {
		return report, ErrNeedsReview
	}
	return report, nil
}

type importState struct {
	courseIDs          map[string]uuid.UUID
	lessonIDs          map[string]uuid.UUID
	quizByLegacyKey    map[string]legacyQuizState
	rawActive          map[uuid.UUID]rawDocument
	lessonsWithContent map[uuid.UUID]struct{}
	attemptByFeedback  map[string]attemptReference
	attemptByTimestamp map[string][]uuid.UUID
	attemptScores      map[string][]attemptScore
	feedbackCandidates map[string]int
	courseAliases      map[string]string
	submittedAttempts  []uuid.UUID
}

type rawDocument struct {
	id       uuid.UUID
	priority int
}

type legacyQuizState struct {
	id            uuid.UUID
	questionIDs   map[string]uuid.UUID
	questionScore map[string]float64
	maxScore      float64
	hasSubjective bool
}

type attemptScore struct {
	id          uuid.UUID
	autoScore   float64
	maxScore    float64
	submittedAt time.Time
}

type attemptReference struct {
	id     uuid.UUID
	quiz   legacyQuizState
	status string
}

func newImportState() *importState {
	return &importState{
		courseIDs: map[string]uuid.UUID{}, lessonIDs: map[string]uuid.UUID{},
		quizByLegacyKey: map[string]legacyQuizState{}, rawActive: map[uuid.UUID]rawDocument{},
		lessonsWithContent: map[uuid.UUID]struct{}{}, attemptByFeedback: map[string]attemptReference{},
		attemptByTimestamp: map[string][]uuid.UUID{}, attemptScores: map[string][]attemptScore{},
		courseAliases: map[string]string{}, feedbackCandidates: map[string]int{},
	}
}

func (s *Service) importFiles(ctx context.Context, tx pgx.Tx, root string, manifest Manifest, report *ImportReport, state *importState) error {
	for _, file := range manifest.Files {
		if file.Kind == "attempt_feedback" {
			state.feedbackCandidates[feedbackKey(file.RelativePath)]++
		}
	}
	for _, kind := range []string{"course_subject", "lesson_info", "student_notes", "teacher_outline", "raw_text", "quiz", "attempt_answers", "attempt_feedback", "weaknesses", "scores", "wrong_questions", "legacy_task"} {
		for _, file := range manifest.Files {
			if file.Kind != kind {
				continue
			}
			if err := s.importOne(ctx, tx, root, file, report, state); err != nil {
				return err
			}
		}
	}
	for _, file := range manifest.Files {
		if isHandledKind(file.Kind) {
			continue
		}
		if err := s.record(ctx, tx, report, file, "policy", nil, "skipped_by_policy", strPtr("unsupported_schema")); err != nil {
			return err
		}
	}
	return nil
}

func isHandledKind(kind string) bool {
	switch kind {
	case "course_subject", "lesson_info", "student_notes", "teacher_outline", "raw_text", "quiz", "attempt_answers", "attempt_feedback", "weaknesses", "scores", "wrong_questions", "legacy_task":
		return true
	default:
		return false
	}
}

func (s *Service) importOne(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	// A malformed JSON file is quarantined as a source audit fact; it is never
	// parsed into a partial business entity.
	if strings.HasSuffix(file.RelativePath, ".json") && file.ParseStatus != "valid" {
		return s.record(ctx, tx, report, file, file.Kind, nil, "quarantined", strPtr("invalid_json"))
	}
	switch file.Kind {
	case "course_subject":
		return s.importCourse(ctx, tx, root, file, report, state)
	case "lesson_info":
		return s.importLesson(ctx, tx, root, file, report, state)
	case "student_notes", "teacher_outline", "raw_text":
		return s.importDocument(ctx, tx, root, file, report, state)
	case "quiz":
		return s.importQuiz(ctx, tx, root, file, report, state)
	case "attempt_answers":
		return s.importAttempt(ctx, tx, root, file, report, state)
	case "attempt_feedback":
		return s.importFeedback(ctx, tx, root, file, report, state)
	case "legacy_task":
		return s.importTaskAudit(ctx, tx, root, file, report)
	case "weaknesses":
		return s.importWeaknesses(ctx, tx, root, file, report, state)
	case "scores":
		return s.reconcileScores(ctx, tx, root, file, report, state)
	case "wrong_questions":
		return s.reconcileWrongQuestions(ctx, tx, root, file, report, state)
	default:
		return s.record(ctx, tx, report, file, "policy", nil, "skipped_by_policy", strPtr("unsupported_schema"))
	}
}

func (s *Service) importCourse(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	name, ok := courseFromPath(file.RelativePath)
	if !ok {
		return s.record(ctx, tx, report, file, "course", nil, "quarantined", strPtr("unsupported_schema"))
	}
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "course", nil, "quarantined", strPtr("unsupported_schema"))
	}
	title := limitText(strings.TrimSpace(string(body)), 200)
	// The archived Python application creates an empty .subject marker before
	// it has persisted a display title. The directory is still a stable source
	// identity; info.json.subject later becomes an alias for reconciliation.
	if title == "" {
		title = name
	}
	state.courseAliases[name] = name
	state.courseAliases[title] = name
	if id, found, err := s.sourceTarget(ctx, tx, file, "course"); err != nil {
		return err
	} else if found {
		state.courseIDs[name] = *id
		return s.record(ctx, tx, report, file, "course", id, "deduplicated", nil)
	}
	if changed, err := s.sourcePathWasImported(ctx, tx, file, "course"); err != nil {
		return err
	} else if changed {
		return s.record(ctx, tx, report, file, "course", nil, "quarantined", strPtr("duplicate_business_key"))
	}
	c := &course.Course{Slug: "legacy-" + shortHash(file.RelativePath), Title: title}
	if err := s.courses.Create(ctx, tx, c); err != nil {
		return s.record(ctx, tx, report, file, "course", nil, "quarantined", strPtr("duplicate_business_key"))
	}
	state.courseIDs[name] = c.ID
	return s.record(ctx, tx, report, file, "course", &c.ID, "imported", nil)
}

func (s *Service) importLesson(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	identity, ok := parseLessonPath(file.RelativePath)
	if !ok {
		return s.record(ctx, tx, report, file, "lesson", nil, "quarantined", strPtr("unsupported_schema"))
	}
	if id, found, err := s.sourceTarget(ctx, tx, file, "lesson"); err != nil {
		return err
	} else if found {
		state.lessonIDs[identity.key()] = *id
		return s.record(ctx, tx, report, file, "lesson", id, "deduplicated", nil)
	}
	courseID, exists := state.courseIDs[identity.course]
	if !exists {
		return s.record(ctx, tx, report, file, "lesson", nil, "quarantined", strPtr("course_identity_conflict"))
	}
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "lesson", nil, "quarantined", strPtr("source_changed"))
	}
	var info struct {
		Name    string `json:"name"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return s.record(ctx, tx, report, file, "lesson", nil, "quarantined", strPtr("invalid_json"))
	}
	title := strings.TrimSpace(info.Name)
	if title == "" {
		title = fmt.Sprintf("第 %d 课", identity.ordinal)
	}
	if subject := strings.TrimSpace(info.Subject); subject != "" {
		state.courseAliases[subject] = identity.course
	}
	l := &lesson.Lesson{CourseID: courseID, Ordinal: int32(identity.ordinal), Title: limitText(title, 200), Metadata: []byte(`{"importer":"legacy-import-v1"}`)}
	if err := s.lessons.Create(ctx, tx, l); err != nil {
		return s.record(ctx, tx, report, file, "lesson", nil, "quarantined", strPtr("duplicate_business_key"))
	}
	state.lessonIDs[identity.key()] = l.ID
	return s.record(ctx, tx, report, file, "lesson", &l.ID, "imported", nil)
}

func (s *Service) importDocument(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	identity, ok := parseLessonPath(file.RelativePath)
	if !ok {
		return s.record(ctx, tx, report, file, "document", nil, "quarantined", strPtr("unsupported_schema"))
	}
	lessonID, exists := state.lessonIDs[identity.key()]
	if !exists {
		return s.record(ctx, tx, report, file, "document", nil, "quarantined", strPtr("course_identity_conflict"))
	}
	if id, found, err := s.sourceTarget(ctx, tx, file, "document"); err != nil {
		return err
	} else if found {
		state.lessonsWithContent[lessonID] = struct{}{}
		return s.record(ctx, tx, report, file, "document", id, "deduplicated", nil)
	}
	if changed, err := s.sourcePathWasImported(ctx, tx, file, "document"); err != nil {
		return err
	} else if changed {
		return s.record(ctx, tx, report, file, "document", nil, "quarantined", strPtr("duplicate_business_key"))
	}
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "document", nil, "quarantined", strPtr("source_changed"))
	}
	kind := map[string]string{"student_notes": "student_notes", "teacher_outline": "teacher_outline", "raw_text": "raw_text"}[file.Kind]
	if file.Kind == "raw_text" {
		if err := s.repo.CreateLegacyMaterial(ctx, tx, lessonID, string(body)); err != nil {
			return err
		}
	}
	doc := &document.Document{LessonID: lessonID, Kind: kind, ContentMD: string(body)}
	if err := s.documents.CreateVersion(ctx, tx, doc); err != nil {
		return err
	}
	if file.Kind == "raw_text" {
		candidate := rawDocument{id: doc.ID, priority: rawPriority(file.RelativePath)}
		if current, ok := state.rawActive[lessonID]; !ok || candidate.priority > current.priority {
			state.rawActive[lessonID] = candidate
		}
	} else if err := s.documents.Publish(ctx, tx, doc.ID); err != nil {
		return err
	}
	state.lessonsWithContent[lessonID] = struct{}{}
	return s.record(ctx, tx, report, file, "document", &doc.ID, "imported", nil)
}

func (s *Service) importQuiz(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	identity, ok := parseLessonPath(file.RelativePath)
	if !ok {
		return s.record(ctx, tx, report, file, "quiz", nil, "quarantined", strPtr("unsupported_schema"))
	}
	lessonID, exists := state.lessonIDs[identity.key()]
	if !exists {
		return s.record(ctx, tx, report, file, "quiz", nil, "quarantined", strPtr("missing_quiz"))
	}
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "quiz", nil, "quarantined", strPtr("source_changed"))
	}
	legacy, err := parseQuiz(body, file.RelativePath)
	if err != nil {
		return s.record(ctx, tx, report, file, "quiz", nil, "quarantined", strPtr("unsupported_schema"))
	}
	key := identity.key() + "/" + legacy.ID
	if id, found, err := s.sourceTarget(ctx, tx, file, "quiz"); err != nil {
		return err
	} else if found {
		stateValue, err := s.loadQuizState(ctx, tx, *id, legacy)
		if err != nil {
			return err
		}
		state.quizByLegacyKey[key] = stateValue
		return s.record(ctx, tx, report, file, "quiz", id, "deduplicated", nil)
	}
	if changed, err := s.sourcePathWasImported(ctx, tx, file, "quiz"); err != nil {
		return err
	} else if changed {
		return s.record(ctx, tx, report, file, "quiz", nil, "quarantined", strPtr("duplicate_business_key"))
	}
	questions := make([]quiz.Question, 0, len(legacy.Questions))
	questionIDs := make(map[string]uuid.UUID, len(legacy.Questions))
	questionScores := make(map[string]float64, len(legacy.Questions))
	items := make([]quiz.Item, 0, len(legacy.Questions))
	maxScore, subjective := 0.0, false
	for i, q := range legacy.Questions {
		if err := validateQuestion(q); err != nil {
			return s.record(ctx, tx, report, file, "quiz", nil, "quarantined", strPtr("unsupported_schema"))
		}
		hash := questionHash(q)
		question := quiz.Question{LessonID: lessonID, Type: q.Type, Stem: q.Stem, Options: q.Options,
			CorrectAnswer: q.CorrectAnswer, Explanation: optionalText(q.Explanation), TestedConcepts: q.TestedConcepts,
			Difficulty: q.Difficulty, CognitiveLevel: q.CognitiveLevel, DefaultScore: q.Score, ContentHash: &hash}
		questions = append(questions, question)
		questionIDs[q.ID], questionScores[q.ID] = question.ID, q.Score
		items = append(items, quiz.Item{QuestionID: question.ID, Position: int32(i + 1), Score: q.Score})
		maxScore += q.Score
		if q.Type == "short_answer" || q.Type == "case_analysis" {
			subjective = true
		}
	}
	if err := s.quizzes.CreateQuestions(ctx, tx, questions); err != nil {
		return err
	}
	for i := range questions {
		questionIDs[legacy.Questions[i].ID] = questions[i].ID
		items[i].QuestionID = questions[i].ID
	}
	newQuiz := &quiz.Quiz{LessonID: lessonID, Title: limitText(legacy.Title, 300), Source: legacy.Source,
		PolicySnapshot: []byte(`{"importer":"legacy-import-v1"}`), Status: "ready", Items: items}
	if err := s.quizzes.Create(ctx, tx, newQuiz); err != nil {
		return err
	}
	state.quizByLegacyKey[key] = legacyQuizState{id: newQuiz.ID, questionIDs: questionIDs, questionScore: questionScores, maxScore: maxScore, hasSubjective: subjective}
	return s.record(ctx, tx, report, file, "quiz", &newQuiz.ID, "imported", nil)
}

func (s *Service) importAttempt(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	identity, ok := parseLessonPath(file.RelativePath)
	if !ok {
		return s.record(ctx, tx, report, file, "attempt", nil, "quarantined", strPtr("unsupported_schema"))
	}
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "attempt", nil, "quarantined", strPtr("source_changed"))
	}
	legacy, err := parseAttempt(body)
	if err != nil {
		return s.record(ctx, tx, report, file, "attempt", nil, "quarantined", strPtr("legacy_grading_input_incomplete"))
	}
	quizState, exists := state.quizByLegacyKey[identity.key()+"/"+legacy.QuizID]
	if !exists {
		return s.record(ctx, tx, report, file, "attempt", nil, "quarantined", strPtr("missing_quiz"))
	}
	if id, found, err := s.sourceTarget(ctx, tx, file, "attempt"); err != nil {
		return err
	} else if found {
		status, err := s.repo.AttemptStatus(ctx, tx, *id)
		if err != nil {
			return err
		}
		state.attemptByFeedback[feedbackKey(file.RelativePath)] = attemptReference{id: *id, quiz: quizState, status: status}
		return s.record(ctx, tx, report, file, "attempt", id, "deduplicated", nil)
	}
	if legacy.TotalPossible == nil || legacy.AutoScore == nil || math.Abs(*legacy.TotalPossible-quizState.maxScore) > 0.01 {
		return s.record(ctx, tx, report, file, "attempt", nil, "quarantined", strPtr("legacy_grading_input_incomplete"))
	}
	answers := make([]importedAnswer, 0, len(legacy.Answers))
	for _, answer := range legacy.Answers {
		qid, ok := quizState.questionIDs[answer.QuestionID]
		if !ok {
			return s.record(ctx, tx, report, file, "attempt", nil, "quarantined", strPtr("unknown_question"))
		}
		awarded := answer.AIScore
		if awarded == nil && answer.IsCorrect != nil {
			v := 0.0
			if *answer.IsCorrect {
				v = quizState.questionScore[answer.QuestionID]
			}
			awarded = &v
		}
		answers = append(answers, importedAnswer{QuestionID: qid, Answer: answer.UserAnswer, Correct: answer.IsCorrect, Awarded: awarded})
	}
	submittedAt := parseLegacyTime(legacy.CreatedAt)
	status, final, gradedAt, errorCode, errorDetail := "submitted", (*float64)(nil), (*time.Time)(nil), (*string)(nil), (*string)(nil)
	if legacy.AIStatus == "pending" || legacy.AIStatus == "failed" {
		status = "grading_failed"
		errorCode, errorDetail = strPtr("legacy_task_not_resumable"), strPtr("历史批阅任务不可恢复。")
	} else if !quizState.hasSubjective {
		status = "graded"
		final, gradedAt = legacy.AutoScore, &submittedAt
	}
	value := importedAttempt{ID: uuid.New(), QuizID: quizState.id, Status: status, AutoScore: *legacy.AutoScore,
		FinalScore: final, MaxScore: quizState.maxScore, ErrorCode: errorCode, ErrorDetail: errorDetail,
		SubmittedAt: submittedAt, GradedAt: gradedAt, Answers: answers}
	if err := s.repo.CreateAttempt(ctx, tx, value); err != nil {
		return err
	}
	state.attemptByFeedback[feedbackKey(file.RelativePath)] = attemptReference{id: value.ID, quiz: quizState, status: status}
	state.attemptByTimestamp[identity.key()+"/"+legacyTimestamp(file.RelativePath)] = append(
		state.attemptByTimestamp[identity.key()+"/"+legacyTimestamp(file.RelativePath)], value.ID)
	state.attemptScores[identity.key()] = append(state.attemptScores[identity.key()], attemptScore{
		id: value.ID, autoScore: *legacy.AutoScore, maxScore: quizState.maxScore, submittedAt: submittedAt,
	})
	if status == "submitted" {
		state.submittedAttempts = append(state.submittedAttempts, value.ID)
	}
	return s.record(ctx, tx, report, file, "attempt", &value.ID, "imported", nil)
}

func (s *Service) importFeedback(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	key := feedbackKey(file.RelativePath)
	if state.feedbackCandidates[key] != 1 {
		return s.record(ctx, tx, report, file, "attempt_feedback", nil, "quarantined", strPtr("ambiguous_feedback"))
	}
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "attempt_feedback", nil, "quarantined", strPtr("source_changed"))
	}
	legacy, err := parseFeedback(body)
	if err != nil {
		return s.record(ctx, tx, report, file, "attempt_feedback", nil, "quarantined", strPtr("unsupported_schema"))
	}
	attempt, exists := state.attemptByFeedback[key]
	if !exists {
		return s.record(ctx, tx, report, file, "attempt_feedback", nil, "quarantined", strPtr("ambiguous_feedback"))
	}
	if attempt.status == "grading_failed" {
		return s.record(ctx, tx, report, file, "attempt_feedback", &attempt.id, "quarantined", strPtr("legacy_task_not_resumable"))
	}
	if attempt.status != "submitted" {
		return s.record(ctx, tx, report, file, "attempt_feedback", &attempt.id, "quarantined", strPtr("ambiguous_feedback"))
	}
	feedback, err := validateFeedback(legacy, attempt.quiz)
	if err != nil {
		return s.record(ctx, tx, report, file, "attempt_feedback", &attempt.id, "quarantined", strPtr("invalid_score_range"))
	}
	completed, err := s.repo.CompleteFeedback(ctx, tx, attempt.id, legacy.TotalScore, parseLegacyTime(legacy.CreatedAt), feedback)
	if err != nil {
		return err
	}
	if !completed {
		return s.record(ctx, tx, report, file, "attempt_feedback", &attempt.id, "quarantined", strPtr("ambiguous_feedback"))
	}
	attempt.status = "graded"
	state.attemptByFeedback[key] = attempt
	return s.record(ctx, tx, report, file, "attempt_feedback", &attempt.id, "imported", nil)
}

func (s *Service) importTaskAudit(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport) error {
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "legacy_task", nil, "quarantined", strPtr("source_changed"))
	}
	var task struct {
		TaskType  string `json:"task_type"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(body, &task); err != nil {
		return s.record(ctx, tx, report, file, "legacy_task", nil, "quarantined", strPtr("invalid_json"))
	}
	reason := (*string)(nil)
	if task.Status == "pending" || task.Status == "failed" {
		reason = strPtr("legacy_task_not_resumable")
	}
	report.Tasks = append(report.Tasks, TaskSummary{RelativePath: file.RelativePath, TaskType: task.TaskType, Status: task.Status, CreatedAt: task.CreatedAt})
	return s.record(ctx, tx, report, file, "legacy_task", nil, "imported", reason)
}

func (s *Service) importWeaknesses(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "weakness_bundle", nil, "quarantined", strPtr("source_changed"))
	}
	var values []legacyWeakness
	if err := json.Unmarshal(body, &values); err != nil || len(values) == 0 {
		return s.record(ctx, tx, report, file, "weakness_bundle", nil, "quarantined", strPtr("unsupported_schema"))
	}
	for _, value := range values {
		courseKey, ok := state.courseAliases[strings.TrimSpace(value.Course)]
		if !ok || value.Severity < 1 || value.Severity > 5 || strings.TrimSpace(value.Concept) == "" {
			return s.record(ctx, tx, report, file, "weakness_bundle", nil, "quarantined", strPtr("unsupported_schema"))
		}
		courseID := state.courseIDs[courseKey]
		var lessonID *uuid.UUID
		if value.Chapter > 0 {
			if id, found := state.lessonIDs[lessonIdentity{course: courseKey, ordinal: value.Chapter}.key()]; found {
				lessonID = &id
			}
		}
		evidence, err := json.Marshal([]map[string]string{{"source": "legacy-import", "evidence": value.Evidence}})
		if err != nil {
			return err
		}
		weakness := &mastery.Weakness{CourseID: courseID, LessonID: lessonID, ConceptKey: normalizedConcept(value.Concept),
			ConceptLabel: value.Concept, Description: value.Description, Evidence: evidence, Severity: int16(value.Severity),
			ReviewCount: int32(value.ReviewCount), Mastered: value.Mastered}
		if err := (mastery.Repository{}).CreateWeakness(ctx, tx, weakness); err != nil {
			return err
		}
	}
	return s.record(ctx, tx, report, file, "weakness_bundle", nil, "imported", nil)
}

func (s *Service) reconcileWrongQuestions(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	identity, ok := parseLessonPath(file.RelativePath)
	if !ok {
		return s.record(ctx, tx, report, file, "wrong_question_evidence", nil, "quarantined", strPtr("unsupported_schema"))
	}
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "wrong_question_evidence", nil, "quarantined", strPtr("source_changed"))
	}
	var values []struct {
		QuestionID string `json:"question_id"`
	}
	if err := json.Unmarshal(body, &values); err != nil || len(values) == 0 {
		return s.record(ctx, tx, report, file, "wrong_question_evidence", nil, "quarantined", strPtr("unsupported_schema"))
	}
	candidates := state.attemptByTimestamp[identity.key()+"/"+legacyTimestamp(file.RelativePath)]
	if len(candidates) != 1 {
		return s.record(ctx, tx, report, file, "wrong_question_evidence", nil, "quarantined", strPtr("missing_quiz"))
	}
	attemptID := candidates[0]
	for _, value := range values {
		matches := 0
		for key, quizState := range state.quizByLegacyKey {
			if strings.HasPrefix(key, identity.key()+"/") {
				if _, found := quizState.questionIDs[value.QuestionID]; found {
					matches++
				}
			}
		}
		if value.QuestionID == "" || matches != 1 {
			return s.record(ctx, tx, report, file, "wrong_question_evidence", nil, "quarantined", strPtr("unknown_question"))
		}
	}
	// The target is the verified attempt, not a new answer or weakness. This
	// preserves the old evidence without replaying runtime side effects.
	return s.record(ctx, tx, report, file, "wrong_question_evidence", &attemptID, "imported", nil)
}

func (s *Service) reconcileScores(ctx context.Context, tx pgx.Tx, root string, file SourceFile, report *ImportReport, state *importState) error {
	body, err := readChecked(root, file)
	if err != nil {
		return s.record(ctx, tx, report, file, "score_reconciliation", nil, "quarantined", strPtr("source_changed"))
	}
	var values []legacyScore
	if err := json.Unmarshal(body, &values); err != nil || len(values) == 0 {
		return s.record(ctx, tx, report, file, "score_reconciliation", nil, "quarantined", strPtr("unsupported_schema"))
	}
	for _, value := range values {
		courseKey, ok := state.courseAliases[strings.TrimSpace(value.Subject)]
		if !ok {
			return s.record(ctx, tx, report, file, "score_reconciliation", nil, "quarantined", strPtr("unmatched_score"))
		}
		key := lessonIdentity{course: courseKey, ordinal: value.Lesson}.key()
		matches := 0
		for _, candidate := range state.attemptScores[key] {
			if math.Abs(candidate.autoScore-value.Score) <= 0.01 && math.Abs(candidate.maxScore-value.Total) <= 0.01 && sameSecond(candidate.submittedAt, parseScoreTime(value.Timestamp)) {
				matches++
			}
		}
		if matches == 0 {
			return s.record(ctx, tx, report, file, "score_reconciliation", nil, "quarantined", strPtr("unmatched_score"))
		}
		if matches > 1 {
			return s.record(ctx, tx, report, file, "score_reconciliation", nil, "quarantined", strPtr("ambiguous_score"))
		}
	}
	return s.record(ctx, tx, report, file, "score_reconciliation", nil, "imported", nil)
}

func (s *Service) publishDocuments(ctx context.Context, tx pgx.Tx, state *importState) error {
	for _, candidate := range state.rawActive {
		if err := s.documents.Publish(ctx, tx, candidate.id); err != nil {
			return err
		}
	}
	for lessonID := range state.lessonsWithContent {
		if err := s.repo.SetLessonReady(ctx, tx, lessonID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) sourceTarget(ctx context.Context, tx pgx.Tx, file SourceFile, targetType string) (*uuid.UUID, bool, error) {
	return s.repo.SourceTarget(ctx, tx, file.RelativePath, file.SHA256, targetType)
}

func (s *Service) sourcePathWasImported(ctx context.Context, tx pgx.Tx, file SourceFile, targetType string) (bool, error) {
	return s.repo.SourcePathWasImported(ctx, tx, file.RelativePath, targetType)
}

func (s *Service) record(ctx context.Context, tx pgx.Tx, report *ImportReport, file SourceFile, targetType string, targetID *uuid.UUID, status string, reason *string) error {
	if err := s.repo.Record(ctx, tx, report.BatchID, sourceRecord{SourcePath: file.RelativePath, SourceHash: file.SHA256,
		SourceType: file.Kind, TargetType: targetType, TargetID: targetID, Status: status, ReasonCode: reason}); err != nil {
		return err
	}
	report.Records.Total++
	switch status {
	case "imported":
		report.Records.Imported++
	case "deduplicated":
		report.Records.Deduplicated++
	case "quarantined":
		report.Records.Quarantined++
	case "skipped_by_policy":
		report.Records.SkippedByPolicy++
	}
	return nil
}

func (s *Service) loadQuizState(ctx context.Context, tx pgx.Tx, id uuid.UUID, legacy legacyQuiz) (legacyQuizState, error) {
	stored, questions, err := s.quizzes.GetWithQuestions(ctx, tx, id)
	if err != nil {
		return legacyQuizState{}, err
	}
	byHash := map[string]quiz.QuizQuestion{}
	for _, item := range questions {
		if item.Question.ContentHash != nil {
			byHash[*item.Question.ContentHash] = item
		}
	}
	result := legacyQuizState{id: stored.ID, questionIDs: map[string]uuid.UUID{}, questionScore: map[string]float64{}}
	for _, question := range legacy.Questions {
		item, ok := byHash[questionHash(question)]
		if !ok {
			return legacyQuizState{}, fmt.Errorf("legacy quiz source mapping missing question")
		}
		result.questionIDs[question.ID], result.questionScore[question.ID] = item.Question.ID, item.Score
		result.maxScore += item.Score
		if question.Type == "short_answer" || question.Type == "case_analysis" {
			result.hasSubjective = true
		}
	}
	return result, nil
}

type legacyQuiz struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	QuizTitle string           `json:"quiz_title"`
	Source    string           `json:"source"`
	Questions []legacyQuestion `json:"questions"`
}

type legacyQuestion struct {
	ID             string          `json:"id"`
	Stem           string          `json:"question"`
	Type           string          `json:"type"`
	Options        json.RawMessage `json:"options"`
	CorrectAnswer  json.RawMessage `json:"correct_answer"`
	Explanation    string          `json:"explanation"`
	TestedConcepts json.RawMessage `json:"tested_concepts"`
	Difficulty     string          `json:"difficulty"`
	CognitiveLevel string          `json:"cognitive_level"`
	Score          float64         `json:"score"`
}

func parseQuiz(body []byte, path string) (legacyQuiz, error) {
	var value legacyQuiz
	if err := json.Unmarshal(body, &value); err != nil {
		return legacyQuiz{}, err
	}
	if value.ID == "" {
		value.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if value.Title == "" {
		value.Title = value.QuizTitle
	}
	if value.Source == "" {
		value.Source = "regular"
	}
	if value.ID == "" || strings.TrimSpace(value.Title) == "" || len(value.Questions) == 0 {
		return legacyQuiz{}, errors.New("incomplete legacy quiz")
	}
	if value.Source != "regular" && value.Source != "retest" && value.Source != "assembled" {
		return legacyQuiz{}, errors.New("unsupported quiz source")
	}
	return value, nil
}

func validateQuestion(q legacyQuestion) error {
	if q.ID == "" || strings.TrimSpace(q.Stem) == "" || q.Score <= 0 || !json.Valid(q.CorrectAnswer) || len(q.CorrectAnswer) == 0 {
		return errors.New("incomplete question")
	}
	if q.Options != nil && !json.Valid(q.Options) {
		return errors.New("invalid options")
	}
	if q.TestedConcepts == nil || !json.Valid(q.TestedConcepts) {
		return errors.New("invalid concepts")
	}
	validTypes := map[string]bool{"single_choice": true, "multiple_choice": true, "true_false": true, "short_answer": true, "case_analysis": true}
	validDifficulty := map[string]bool{"easy": true, "medium": true, "hard": true, "challenge": true}
	validCognitive := map[string]bool{"recall": true, "explain": true, "apply": true, "transfer": true, "reflect": true}
	if !validTypes[q.Type] || !validDifficulty[q.Difficulty] || !validCognitive[q.CognitiveLevel] {
		return errors.New("unsupported question value")
	}
	return nil
}

type legacyAttempt struct {
	QuizID        string   `json:"quiz_id"`
	CreatedAt     string   `json:"created_at"`
	AutoScore     *float64 `json:"auto_score"`
	TotalPossible *float64 `json:"total_possible"`
	AIGrading     struct {
		Status string `json:"status"`
	} `json:"ai_grading"`
	Answers  []legacyAnswer `json:"answers"`
	AIStatus string         `json:"-"`
}

type legacyAnswer struct {
	QuestionID string          `json:"question_id"`
	UserAnswer json.RawMessage `json:"user_answer"`
	IsCorrect  *bool           `json:"is_correct"`
	AIScore    *float64        `json:"ai_score"`
}

type legacyWeakness struct {
	Course      string `json:"course"`
	Chapter     int    `json:"chapter"`
	Concept     string `json:"concept"`
	Description string `json:"weakness_description"`
	Evidence    string `json:"evidence"`
	Severity    int    `json:"severity"`
	ReviewCount int    `json:"review_count"`
	Mastered    bool   `json:"mastered"`
}

type legacyScore struct {
	Subject   string  `json:"subject"`
	Lesson    int     `json:"lesson"`
	Score     float64 `json:"score"`
	Total     float64 `json:"total"`
	Timestamp string  `json:"timestamp"`
}

func parseAttempt(body []byte) (legacyAttempt, error) {
	var value legacyAttempt
	if err := json.Unmarshal(body, &value); err != nil {
		return legacyAttempt{}, err
	}
	value.AIStatus = value.AIGrading.Status
	if value.QuizID == "" || value.CreatedAt == "" || len(value.Answers) == 0 {
		return legacyAttempt{}, errors.New("incomplete attempt")
	}
	for _, answer := range value.Answers {
		if answer.QuestionID == "" || !json.Valid(answer.UserAnswer) {
			return legacyAttempt{}, errors.New("invalid answer")
		}
	}
	return value, nil
}

type legacyFeedback struct {
	CreatedAt        string                   `json:"created_at"`
	TotalScore       float64                  `json:"total_score"`
	QuestionFeedback []legacyFeedbackQuestion `json:"question_feedback"`
}

type legacyFeedbackQuestion struct {
	QuestionID           string          `json:"question_id"`
	Score                float64         `json:"score"`
	IsCorrect            *bool           `json:"is_correct"`
	Feedback             string          `json:"feedback"`
	CorrectUnderstanding string          `json:"correct_understanding"`
	WeakConcepts         json.RawMessage `json:"weak_concepts"`
}

func parseFeedback(body []byte) (legacyFeedback, error) {
	var value legacyFeedback
	if err := json.Unmarshal(body, &value); err != nil || value.TotalScore < 0 || len(value.QuestionFeedback) == 0 {
		if err == nil {
			err = errors.New("invalid feedback")
		}
		return legacyFeedback{}, err
	}
	return value, nil
}

func validateFeedback(value legacyFeedback, quizState legacyQuizState) (map[uuid.UUID]json.RawMessage, error) {
	if value.TotalScore > quizState.maxScore+0.01 {
		return nil, errors.New("feedback total is above quiz maximum")
	}
	feedback := make(map[uuid.UUID]json.RawMessage, len(value.QuestionFeedback))
	total := 0.0
	for _, item := range value.QuestionFeedback {
		questionID, found := quizState.questionIDs[item.QuestionID]
		if !found || item.Score < 0 || item.Score > quizState.questionScore[item.QuestionID]+0.01 {
			return nil, errors.New("feedback question is invalid")
		}
		if _, duplicate := feedback[questionID]; duplicate {
			return nil, errors.New("feedback question is duplicated")
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		feedback[questionID] = encoded
		total += item.Score
	}
	if math.Abs(total-value.TotalScore) > 0.01 {
		return nil, errors.New("feedback question total does not match")
	}
	return feedback, nil
}

func readChecked(root string, file SourceFile) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(file.RelativePath))
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != file.SHA256 {
		return nil, errors.New("legacy source changed after scan")
	}
	return body, nil
}

func rawPriority(path string) int {
	switch {
	case strings.HasSuffix(path, "/raw/raw_text.md"):
		return 4
	case strings.HasSuffix(path, "/raw_src/all_raw_text.txt"):
		return 3
	case strings.HasSuffix(path, "/raw_src/source_text.md"):
		return 2
	case strings.HasSuffix(path, "/raw_src/source_material.md"):
		return 1
	default:
		return 0
	}
}

func feedbackKey(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{"_answers.json", "_feedback.json", "_grade_result.json", "_wrong.json"} {
		if strings.HasSuffix(base, suffix) {
			return filepath.ToSlash(filepath.Join(filepath.Dir(path), strings.TrimSuffix(base, suffix)))
		}
	}
	return path
}

func legacyTimestamp(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{"_answers.json", "_feedback.json", "_grade_result.json", "_wrong.json"} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return base
}

func parseLegacyTime(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("20060102_150405", value, time.Local); err == nil {
		return parsed
	}
	return time.Now().UTC()
}

func parseScoreTime(value string) time.Time {
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed
	}
	return parseLegacyTime(value)
}

func sameSecond(left, right time.Time) bool {
	return left.UTC().Truncate(time.Second).Equal(right.UTC().Truncate(time.Second))
}

func normalizedConcept(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func questionHash(question legacyQuestion) string {
	encoded, _ := json.Marshal(question)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func optionalText(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func limitText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func strPtr(value string) *string { return &value }

func hasUnacceptedFindings(manifest Manifest, resolution *Resolution) bool {
	for _, finding := range manifest.Findings {
		if finding.Severity != "error" {
			continue
		}
		accepted := false
		if resolution != nil {
			for _, entry := range resolution.AcceptedFindings {
				if entry.Code == finding.Code && entry.RelativePath == finding.RelativePath {
					accepted = true
					break
				}
			}
		}
		if !accepted {
			return true
		}
	}
	return false
}

func validateResolution(manifest Manifest, resolution *Resolution) error {
	if resolution == nil {
		return nil
	}
	if resolution.SchemaVersion != 1 || resolution.ManifestSHA256 != manifest.ManifestSHA256 {
		return errors.New("resolution does not match manifest")
	}
	findings := make(map[string]struct{}, len(manifest.Findings))
	for _, finding := range manifest.Findings {
		if finding.Severity == "error" {
			findings[finding.Code+"\x00"+finding.RelativePath] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(resolution.AcceptedFindings))
	for _, entry := range resolution.AcceptedFindings {
		key := entry.Code + "\x00" + entry.RelativePath
		if _, duplicate := seen[key]; duplicate {
			return errors.New("resolution contains a duplicate finding")
		}
		seen[key] = struct{}{}
		if _, exists := findings[key]; !exists {
			return errors.New("resolution accepts a finding absent from manifest")
		}
	}
	return nil
}
