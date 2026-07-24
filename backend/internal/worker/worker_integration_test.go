// worker_integration_test.go 用真实 PostgreSQL + 真实文件存储 + fake
// process runner 覆盖 B5 验收矩阵：成功发布、超时、取消、非零退出、
// SSRF 二次校验、重复文件、中断恢复（worker_lost 回收后重跑）。
package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/course"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/material"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/lessonimport"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

// ---------------------------------------------------------------------------
// 装配与播种辅助
// ---------------------------------------------------------------------------

type env struct {
	pool    *pgxpool.Pool
	store   *filestore.Store
	runner  *worker.Runner
	handler *lessonimport.Handler
	fake    *proc.Fake
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

var okTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
})

func publicResolver(context.Context, string) ([]net.IP, error) {
	return []net.IP{net.ParseIP("151.101.1.140")}, nil
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := databasetest.New(t)
	store, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	fake := proc.NewFake()
	h := &lessonimport.Handler{
		Pool:         pool,
		Files:        store,
		Proc:         fake,
		URLAllowlist: []string{"youtube.com", "youtu.be", "bilibili.com"},
		Resolver:     publicResolver,
		Transport:    okTransport,
	}
	reg := worker.NewRegistry()
	reg.Register(job.KindLessonImport, h)
	r := &worker.Runner{
		Pool: pool, Registry: reg, Files: store, WorkerID: "test-worker",
		Lease:              5 * time.Second,
		HeartbeatInterval:  200 * time.Millisecond,
		CancelPollInterval: 50 * time.Millisecond,
		PollInterval:       10 * time.Millisecond,
	}
	return &env{pool: pool, store: store, runner: r, handler: h, fake: fake}
}

func (e *env) newLesson(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	c := &course.Course{Slug: "c-" + uuid.NewString(), Title: "课程"}
	if err := (course.Repository{}).Create(ctx, e.pool, c); err != nil {
		t.Fatal(err)
	}
	l := &lesson.Lesson{CourseID: c.ID, Ordinal: 1, Title: "课时"}
	if err := (lesson.Repository{}).Create(ctx, e.pool, l); err != nil {
		t.Fatal(err)
	}
	return l.ID
}

func mustJSON(t *testing.T, v map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// newImportJob 直接经 Repository 播种 lesson_import Job（与 API 创建同构）。
func (e *env) newImportJob(t *testing.T, lessonID uuid.UUID, payload map[string]any) uuid.UUID {
	t.Helper()
	family := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindLessonImport,
		AggregateType:    "lesson",
		AggregateID:      lessonID,
		GenerationFamily: &family,
		Payload:          mustJSON(t, payload),
	}
	if err := (&job.Repository{}).Create(context.Background(), e.pool, j); err != nil {
		t.Fatal(err)
	}
	return j.ID
}

func (e *env) newTextImport(t *testing.T, lessonID uuid.UUID, text string) (materialID, jobID uuid.UUID) {
	t.Helper()
	m := &material.Material{LessonID: lessonID, Kind: material.KindText, InlineText: &text}
	if err := (material.Repository{}).Create(context.Background(), e.pool, m); err != nil {
		t.Fatal(err)
	}
	jobID = e.newImportJob(t, lessonID, map[string]any{
		"material_id": m.ID.String(), "lesson_id": lessonID.String(), "material_kind": "text",
	})
	return m.ID, jobID
}

// newUploadImport 经两阶段发布播种上传素材（Owner 为独立幂等 ID），再建导入 Job。
func (e *env) newUploadImport(t *testing.T, lessonID uuid.UUID, filename string, content []byte, extraPayload map[string]any) (materialID, jobID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	staging := filepath.Join(e.store.TmpDirFor(uuid.New()), "payload")
	if err := os.MkdirAll(filepath.Dir(staging), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staging, content, 0o644); err != nil {
		t.Fatal(err)
	}
	var mid uuid.UUID
	_, err := e.store.Publish(ctx, filestore.PublishInput{
		OwnerType:    filestore.OwnerTypeIdempotency,
		OwnerID:      uuid.New(),
		StagedPath:   staging,
		OriginalName: filename,
		MediaType:    "application/octet-stream",
	}, func(ctx context.Context, tx pgx.Tx, objectID uuid.UUID) error {
		m := &material.Material{
			LessonID:     lessonID,
			Kind:         material.KindUpload,
			FileObjectID: &objectID,
			OriginalName: &filename,
		}
		if err := (material.Repository{}).Create(ctx, tx, m); err != nil {
			return err
		}
		mid = m.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"material_id": mid.String(), "lesson_id": lessonID.String(), "material_kind": "upload",
		"original_name": filename,
	}
	for k, v := range extraPayload {
		payload[k] = v
	}
	jobID = e.newImportJob(t, lessonID, payload)
	return mid, jobID
}

// newURLImport 播种 URL 素材与导入 Job。
func (e *env) newURLImport(t *testing.T, lessonID uuid.UUID, rawURL string, extraPayload map[string]any) (materialID, jobID uuid.UUID) {
	t.Helper()
	m := &material.Material{LessonID: lessonID, Kind: material.KindURL, SourceURL: &rawURL}
	if err := (material.Repository{}).Create(context.Background(), e.pool, m); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"material_id": m.ID.String(), "lesson_id": lessonID.String(), "material_kind": "url",
		"source_url": rawURL,
	}
	for k, v := range extraPayload {
		payload[k] = v
	}
	jobID = e.newImportJob(t, lessonID, payload)
	return m.ID, jobID
}

// waitFor 轮询直到条件满足（最多 10 秒）。
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

// writeFile 在目录内写文件（fake 子进程落盘产物）。
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readDirStat 探测目录是否存在。
func readDirStat(dir string) (os.FileInfo, error) {
	return os.Stat(dir)
}

func (e *env) runOnce(t *testing.T) {
	t.Helper()
	claimed, err := e.runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("应领取到一个任务")
	}
}

func getJob(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *job.Job {
	t.Helper()
	j, err := (&job.Repository{}).GetByID(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func getMaterial(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *material.Material {
	t.Helper()
	m, err := (material.Repository{}).GetByID(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func getLesson(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *lesson.Lesson {
	t.Helper()
	l, err := (lesson.Repository{}).GetByID(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func jobStages(t *testing.T, pool *pgxpool.Pool, jobID uuid.UUID) []string {
	t.Helper()
	events, err := (&job.Repository{}).ListEvents(context.Background(), pool, jobID, 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	var stages []string
	for _, ev := range events {
		stages = append(stages, ev.EventType+":"+ev.Stage)
	}
	return stages
}
