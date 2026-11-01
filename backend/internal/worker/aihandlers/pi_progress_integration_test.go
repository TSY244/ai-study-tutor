package aihandlers_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/document"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/piruntime"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

type progressingQuizGenerator struct {
	fake   fakeQuizCLIGenerator
	during func(context.Context) error
}

func (g *progressingQuizGenerator) Generate(ctx context.Context, dir string, req ai.Request) (ai.Response, error) {
	if err := g.during(ctx); err != nil {
		return ai.Response{}, err
	}
	return g.fake.Generate(ctx, dir, req)
}

func TestPiProgressReachesJobBeforeGenerationFinishes(t *testing.T) {
	gen := &progressingQuizGenerator{fake: fakeQuizCLIGenerator{resp: ai.Response{Model: "pi-test", JSON: json.RawMessage(`{"title":"流式测验","questions":[{"type":"single_choice","stem":"极限描述什么？","options":{"A":"趋势","B":"随机数"},"correct_answer":"A","explanation":"极限描述趋势。","tested_concepts":["极限"],"difficulty":"easy","cognitive_level":"recall","score":5}]}`)}}}
	e := newEnvWithQuizCLI(t, gen)
	_, lessonID := seedLesson(t, e.pool)
	publishDoc(t, e.pool, lessonID, document.KindRawText, "极限描述趋势。")
	family := job.GenerationFamilyLessonGeneration
	jobID := newJob(t, e.pool, &job.Job{Kind: job.KindQuizGenerate, AggregateType: "lesson", AggregateID: lessonID, GenerationFamily: &family, Payload: mustJSON(t, map[string]any{"lesson_id": lessonID.String(), "question_count": 1}), MaxAttempts: 1})
	gen.during = func(ctx context.Context) error {
		fake := proc.NewFake()
		fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
			if cmd.OnStderrLine == nil {
				t.Fatal("stream callback missing")
			}
			cmd.OnStderrLine([]byte(`{"type":"pi_progress","phase":"generating","characters":123,"turns":1,"message":"private-source"}`))
			var status, message string
			if err := e.pool.QueryRow(ctx, `SELECT j.status,ev.message FROM jobs j JOIN job_events ev ON ev.job_id=j.id WHERE j.id=$1 ORDER BY ev.id DESC LIMIT 1`, jobID).Scan(&status, &message); err != nil {
				t.Fatal(err)
			}
			if status != job.StatusRunning || !strings.Contains(message, "123 字符") || strings.Contains(message, "private-source") {
				t.Fatalf("not live or unsafe: %s %q", status, message)
			}
			return &proc.Result{}, nil
		}
		_, err := piruntime.Run(ctx, fake, proc.Cmd{Name: "node"})
		return err
	}
	e.runOnce(t)
	if j := getJob(t, e.pool, jobID); j.Status != job.StatusSucceeded {
		t.Fatalf("job did not finish: %s %v", j.Status, j.ErrorCode)
	}
}
