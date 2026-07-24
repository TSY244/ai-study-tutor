// Package domaintest 汇总核心域 Repository 的约束集成测试，
// 共享单个测试 PostgreSQL 实例以控制总时长。
package domaintest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/attempt"
	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

func inTx(t *testing.T, pool *pgxpool.Pool, fn func(tx pgx.Tx)) {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	fn(tx)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func seedLesson(t *testing.T, pool *pgxpool.Pool) (courseID, lessonID uuid.UUID) {
	t.Helper()
	courseID, lessonID = uuid.New(), uuid.New()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO courses (id, slug, title) VALUES ($1,$2,'课程')`, courseID, "c-"+uuid.NewString()[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO lessons (id, course_id, ordinal, title) VALUES ($1,$2,1,'课时')`, lessonID, courseID); err != nil {
		t.Fatal(err)
	}
	return courseID, lessonID
}

func TestCourseConstraints(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	repo := course.Repository{}

	c := &course.Course{Slug: "math-basic", Title: "高等数学"}
	if err := repo.Create(ctx, pool, c); err != nil {
		t.Fatal(err)
	}
	if c.Version != 1 {
		t.Fatalf("初始 version = %d，期望 1", c.Version)
	}
	// slug 唯一。
	dup := &course.Course{Slug: "math-basic", Title: "同名不同课"}
	if err := repo.Create(ctx, pool, dup); !errors.Is(err, course.ErrSlugTaken) {
		t.Fatalf("重复 slug 应冲突，得到 %v", err)
	}
	// 空标题被 CHECK 拒绝。
	empty := &course.Course{Slug: "empty-title", Title: "   "}
	if err := repo.Create(ctx, pool, empty); err == nil {
		t.Fatal("空标题应被 CHECK 约束拒绝")
	}
	// 乐观锁。
	c.Title = "高等数学（上）"
	if err := repo.Update(ctx, pool, c); err != nil {
		t.Fatal(err)
	}
	if c.Version != 2 {
		t.Fatalf("更新后 version = %d，期望 2", c.Version)
	}
	stale := &course.Course{ID: c.ID, Version: 1, Title: "过期写入"}
	if err := repo.Update(ctx, pool, stale); !errors.Is(err, course.ErrVersionConflict) {
		t.Fatalf("过期版本应冲突，得到 %v", err)
	}
}

func TestLessonConstraints(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	courseID, _ := seedLesson(t, pool)
	repo := lesson.Repository{}

	l := &lesson.Lesson{CourseID: courseID, Ordinal: 5, Title: "极限"}
	if err := repo.Create(ctx, pool, l); err != nil {
		t.Fatal(err)
	}
	if l.Status != "draft" || l.Version != 1 {
		t.Fatalf("默认值不正确: %+v", l)
	}
	dup := &lesson.Lesson{CourseID: courseID, Ordinal: 5, Title: "连续"}
	if err := repo.Create(ctx, pool, dup); !errors.Is(err, lesson.ErrDuplicateOrdinal) {
		t.Fatalf("重复 ordinal 应冲突，得到 %v", err)
	}
	// 非法状态被 CHECK 拒绝。
	bad := &lesson.Lesson{CourseID: courseID, Ordinal: 6, Title: "导数"}
	if err := repo.Create(ctx, pool, bad); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE lessons SET status = 'bogus' WHERE id = $1`, bad.ID); err == nil {
		t.Fatal("非法 status 应被 CHECK 拒绝")
	}
}

func TestDocumentActivePartialUnique(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	_, lessonID := seedLesson(t, pool)
	repo := document.Repository{}

	v1 := &document.Document{LessonID: lessonID, Kind: "student_notes", ContentMD: "# v1"}
	v2 := &document.Document{LessonID: lessonID, Kind: "student_notes", ContentMD: "# v2"}
	inTx(t, pool, func(tx pgx.Tx) {
		if err := repo.CreateVersion(ctx, tx, v1); err != nil {
			t.Fatal(err)
		}
		if err := repo.CreateVersion(ctx, tx, v2); err != nil {
			t.Fatal(err)
		}
		if err := repo.Publish(ctx, tx, v1.ID); err != nil {
			t.Fatal(err)
		}
	})
	if v1.Revision != 1 || v2.Revision != 2 {
		t.Fatalf("revision = %d/%d，期望 1/2", v1.Revision, v2.Revision)
	}
	active, err := repo.GetActive(ctx, pool, lessonID, "student_notes")
	if err != nil || active.ID != v1.ID {
		t.Fatalf("active 应为 v1，得到 %+v, %v", active, err)
	}
	// 发布 v2：active 切换，旧版本保留。
	inTx(t, pool, func(tx pgx.Tx) {
		if err := repo.Publish(ctx, tx, v2.ID); err != nil {
			t.Fatal(err)
		}
	})
	active, _ = repo.GetActive(ctx, pool, lessonID, "student_notes")
	if active.ID != v2.ID {
		t.Fatalf("active 应切换为 v2，得到 %s", active.ID)
	}
	var activeCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM documents WHERE lesson_id = $1 AND kind = 'student_notes' AND is_active`,
		lessonID).Scan(&activeCount); err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 {
		t.Fatalf("active 文档数 = %d，期望 1", activeCount)
	}
	old, err := repo.GetByID(ctx, pool, v1.ID)
	if err != nil || old.IsActive {
		t.Fatalf("旧版本应保留且非 active: %+v", old)
	}
}

func TestQuizItemConstraints(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	_, lessonID := seedLesson(t, pool)
	repo := quiz.Repository{}

	q1 := &quiz.Question{LessonID: lessonID, Type: "single_choice", Stem: "1+1=?",
		Options:       []byte(`[{"key":"A","text":"2"},{"key":"B","text":"3"}]`),
		CorrectAnswer: []byte(`{"choice":"A"}`), Difficulty: "easy", CognitiveLevel: "recall",
		DefaultScore: 5}
	q2 := &quiz.Question{LessonID: lessonID, Type: "true_false", Stem: "地球是圆的",
		CorrectAnswer: []byte(`{"value":true}`), Difficulty: "easy", CognitiveLevel: "recall",
		DefaultScore: 5}
	for _, q := range []*quiz.Question{q1, q2} {
		if err := repo.CreateQuestion(ctx, pool, q); err != nil {
			t.Fatal(err)
		}
	}
	z := &quiz.Quiz{LessonID: lessonID, Title: "小测", Source: "regular", Status: "draft",
		Items: []quiz.Item{
			{QuestionID: q1.ID, Position: 1, Score: 5},
			{QuestionID: q2.ID, Position: 2, Score: 5},
		}}
	inTx(t, pool, func(tx pgx.Tx) {
		if err := repo.Create(ctx, tx, z); err != nil {
			t.Fatal(err)
		}
	})
	got, err := repo.GetByID(ctx, pool, z.ID)
	if err != nil || len(got.Items) != 2 || got.Items[0].QuestionID != q1.ID {
		t.Fatalf("试卷读取不符: %+v, %v", got, err)
	}
	// 复合主键：同题不能重复进入同一试卷。
	if _, err := pool.Exec(ctx, `
		INSERT INTO quiz_items (quiz_id, question_id, position, score)
		VALUES ($1,$2,3,5)`, z.ID, q1.ID); err == nil {
		t.Fatal("重复 question 应被主键拒绝")
	}
	// position 唯一。
	if _, err := pool.Exec(ctx, `
		INSERT INTO quiz_items (quiz_id, question_id, position, score)
		VALUES ($1,$2,1,5)`, z.ID, uuid.New()); err == nil {
		t.Fatal("重复 position 应被唯一约束拒绝")
	}
}

func TestAttemptAnswerVersioning(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	_, lessonID := seedLesson(t, pool)
	qrepo := quiz.Repository{}
	q1 := &quiz.Question{LessonID: lessonID, Type: "single_choice", Stem: "q1",
		CorrectAnswer: []byte(`{"choice":"A"}`), Difficulty: "easy", CognitiveLevel: "recall", DefaultScore: 4}
	q2 := &quiz.Question{LessonID: lessonID, Type: "single_choice", Stem: "q2",
		CorrectAnswer: []byte(`{"choice":"B"}`), Difficulty: "easy", CognitiveLevel: "recall", DefaultScore: 6}
	for _, q := range []*quiz.Question{q1, q2} {
		if err := qrepo.CreateQuestion(ctx, pool, q); err != nil {
			t.Fatal(err)
		}
	}
	z := &quiz.Quiz{LessonID: lessonID, Title: "测", Source: "regular", Status: "ready",
		Items: []quiz.Item{{QuestionID: q1.ID, Position: 1, Score: 4}, {QuestionID: q2.ID, Position: 2, Score: 6}}}
	inTx(t, pool, func(tx pgx.Tx) {
		if err := qrepo.Create(ctx, tx, z); err != nil {
			t.Fatal(err)
		}
	})

	arepo := attempt.Repository{}
	a := &attempt.Attempt{QuizID: z.ID}
	inTx(t, pool, func(tx pgx.Tx) {
		if err := arepo.Create(ctx, tx, a); err != nil {
			t.Fatal(err)
		}
	})
	if a.Status != "in_progress" || a.Version != 1 {
		t.Fatalf("初始作答不正确: %+v", a)
	}
	if a.MaxScore == nil || *a.MaxScore != 10 {
		t.Fatalf("max_score 快照 = %+v，期望 10", a.MaxScore)
	}

	// 首次保存 expected=0 → answer_version=1。
	res, err := arepo.SaveAnswer(ctx, pool, a.ID, q1.ID, []byte(`{"choice":"A"}`), 0)
	if err != nil || res.AnswerVersion != 1 {
		t.Fatalf("首次保存 = %+v, %v，期望 answer_version 1", res, err)
	}
	// 重复 expected=0（同题已有答案）→ 冲突。
	if _, err := arepo.SaveAnswer(ctx, pool, a.ID, q1.ID, []byte(`{"choice":"B"}`), 0); !errors.Is(err, attempt.ErrAnswerVersionConflict) {
		t.Fatalf("过期版本应冲突，得到 %v", err)
	}
	// 正确版本 → 2。
	res, err = arepo.SaveAnswer(ctx, pool, a.ID, q1.ID, []byte(`{"choice":"B"}`), 1)
	if err != nil || res.AnswerVersion != 2 {
		t.Fatalf("二次保存 = %+v, %v，期望 answer_version 2", res, err)
	}
	// 不同题可并发保存。
	var wg sync.WaitGroup
	for _, qid := range []uuid.UUID{q1.ID, q2.ID} {
		wg.Add(1)
		go func(qid uuid.UUID, expected int64) {
			defer wg.Done()
			if _, err := arepo.SaveAnswer(context.Background(), pool, a.ID, qid, []byte(`{"choice":"A"}`), expected); err != nil {
				t.Errorf("并发保存失败: %v", err)
			}
		}(qid, map[uuid.UUID]int64{q1.ID: 2, q2.ID: 0}[qid])
	}
	wg.Wait()
	// 全局 version 已随保存递增（创建 1 + 4 次成功保存）。
	got, err := arepo.GetByID(ctx, pool, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 5 {
		t.Fatalf("attempt version = %d，期望 5", got.Version)
	}
	// 提交后拒绝自动保存。
	if _, err := pool.Exec(ctx,
		`UPDATE attempts SET status = 'submitted', submitted_at = now(), auto_score = 4 WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := arepo.SaveAnswer(ctx, pool, a.ID, q2.ID, []byte(`{"choice":"A"}`), 1); !errors.Is(err, attempt.ErrNotInProgress) {
		t.Fatalf("已提交作答应拒绝保存，得到 %v", err)
	}
	// CHECK：未提交不得有 auto_score。
	if _, err := pool.Exec(ctx, `
		INSERT INTO attempts (id, quiz_id, auto_score) VALUES ($1,$2,5)`, uuid.New(), z.ID); err == nil {
		t.Fatal("未提交的 auto_score 应被 CHECK 拒绝")
	}
}

func TestWeaknessAndSnapshotConstraints(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	courseID, lessonID := seedLesson(t, pool)
	repo := mastery.Repository{}

	w := &mastery.Weakness{CourseID: courseID, LessonID: &lessonID,
		ConceptKey: "limit-definition", ConceptLabel: "极限定义", Severity: 3}
	if err := repo.CreateWeakness(ctx, pool, w); err != nil {
		t.Fatal(err)
	}
	// 同一概念重复发现：upsert 更新而非新建。
	again := &mastery.Weakness{CourseID: courseID, LessonID: &lessonID,
		ConceptKey: "limit-definition", ConceptLabel: "极限定义", Severity: 4}
	if err := repo.CreateWeakness(ctx, pool, again); err != nil {
		t.Fatal(err)
	}
	if again.ID != w.ID || again.Version != 2 {
		t.Fatalf("重复发现应更新同一条: id=%s version=%d", again.ID, again.Version)
	}
	// 跨课时（lesson NULL）独立成键。
	cross := &mastery.Weakness{CourseID: courseID, LessonID: nil,
		ConceptKey: "limit-definition", ConceptLabel: "极限定义", Severity: 2}
	if err := repo.CreateWeakness(ctx, pool, cross); err != nil {
		t.Fatal(err)
	}
	if cross.ID == w.ID {
		t.Fatal("跨课时薄弱点应独立成键")
	}

	// 掌握度快照：追加与重复消费去重。
	quizID, attemptID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO quizzes (id, lesson_id, title, source, status) VALUES ($1,$2,'q','regular','ready')`, quizID, lessonID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO attempts (id, quiz_id) VALUES ($1,$2)`, attemptID, quizID); err != nil {
		t.Fatal(err)
	}
	s1 := &mastery.Snapshot{ScopeType: "lesson", CourseID: courseID, LessonID: &lessonID,
		FormulaVersion: "mastery/v1", Value: 62.5, SourceAttemptID: &attemptID}
	if err := repo.AppendSnapshot(ctx, pool, s1); err != nil {
		t.Fatal(err)
	}
	dup := &mastery.Snapshot{ScopeType: "lesson", CourseID: courseID, LessonID: &lessonID,
		FormulaVersion: "mastery/v1", Value: 62.5, SourceAttemptID: &attemptID}
	if err := repo.AppendSnapshot(ctx, pool, dup); !errors.Is(err, mastery.ErrDuplicateSnapshot) {
		t.Fatalf("重复消费应去重，得到 %v", err)
	}
	// scope 一致性 CHECK：course 范围不得带 lesson。
	bad := &mastery.Snapshot{ScopeType: "course", CourseID: courseID, LessonID: &lessonID,
		FormulaVersion: "mastery/v1", Value: 50}
	if err := repo.AppendSnapshot(ctx, pool, bad); err == nil {
		t.Fatal("course 范围带 lesson 应被 CHECK 拒绝")
	}
	latest, err := repo.LatestSnapshot(ctx, pool, "lesson", courseID, &lessonID)
	if err != nil || latest.Value != 62.5 {
		t.Fatalf("最新快照不符: %+v, %v", latest, err)
	}
}
