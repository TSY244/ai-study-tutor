// Package job_test 覆盖 B7 的 Job REST/SSE 联调契约：
// 真实 PostgreSQL + Gin 中间件 + 发布 OpenAPI JSON Schema。
package job_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

const (
	jobsPath       = "/api/v1/jobs"
	jobPath        = "/api/v1/jobs/{job_id}"
	jobCancelPath  = "/api/v1/jobs/{job_id}/cancel"
	jobRetryPath   = "/api/v1/jobs/{job_id}/retry"
	jobEventsPath  = "/api/v1/jobs/{job_id}/events"
	jobStreamPath  = "/api/v1/jobs/stream"
	workerForTests = "job-handler-test-worker"
)

func newJobEngine(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	e := apitest.NewEngine(t)
	h := job.NewHandler(job.NewService(pool),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	h.RegisterRoutes(e.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"))
	return e
}

func createHTTPJob(t *testing.T, pool *pgxpool.Pool, kind string, aggregateType string) *job.Job {
	t.Helper()
	j := &job.Job{
		Kind:          kind,
		AggregateType: aggregateType,
		AggregateID:   uuid.New(),
		Payload:       []byte(`{"fixture":true}`),
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := (&job.Repository{}).Create(context.Background(), tx, j); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return j
}

func claimHTTPJob(t *testing.T, pool *pgxpool.Pool) *job.Job {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	j, err := (&job.Repository{}).Claim(context.Background(), tx, workerForTests, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return j
}

func TestJobRESTContractListDetailEventsAndCancel(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	contract := apitest.Load(t)

	activeJob := createHTTPJob(t, pool, job.KindLessonImport, "lesson")
	failedJob := createHTTPJob(t, pool, job.KindAttemptGrade, "attempt")
	seedRetryTarget(t, pool, "attempt", failedJob.AggregateID)
	if _, err := pool.Exec(context.Background(), `UPDATE jobs SET max_attempts = 1 WHERE id = $1`, failedJob.ID); err != nil {
		t.Fatal(err)
	}
	if claimed := claimHTTPJob(t, pool); claimed == nil || claimed.ID != activeJob.ID {
		t.Fatalf("第一条任务应被领取: %+v", claimed)
	}
	if err := (&job.Repository{}).UpdateProgress(context.Background(), pool,
		activeJob.ID, workerForTests, "normalizing", 35, "正在规范化文本"); err != nil {
		t.Fatal(err)
	}
	if err := (&job.Repository{}).Complete(context.Background(), pool,
		activeJob.ID, workerForTests, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if claimed := claimHTTPJob(t, pool); claimed == nil || claimed.ID != failedJob.ID {
		t.Fatalf("第二条任务应被领取: %+v", claimed)
	}
	if err := (&job.Repository{}).Fail(context.Background(), pool,
		failedJob.ID, workerForTests, "claude_timeout", "Claude 调用超时", true); err != nil {
		t.Fatal(err)
	}
	queuedJob := createHTTPJob(t, pool, job.KindSummaryEvaluate, "lesson")

	rec := apitest.Do(t, engine, http.MethodGet, jobsPath+"?limit=2", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list jobs status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, jobsPath, http.StatusOK, rec.Body.Bytes())
	var page struct {
		Items    []map[string]any `json:"items"`
		PageInfo httpx.PageInfo   `json:"page_info"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.PageInfo.HasMore || page.PageInfo.NextCursor == nil {
		t.Fatalf("第一页应分页: %+v", page)
	}
	if page.Items[0]["id"] != queuedJob.ID.String() {
		t.Fatalf("任务列表应按最近创建优先，首项=%v want=%s", page.Items[0]["id"], queuedJob.ID)
	}

	rec = apitest.Do(t, engine, http.MethodGet, jobsPath+"?active=true", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("active list status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, jobsPath, http.StatusOK, rec.Body.Bytes())
	var activePage struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &activePage); err != nil {
		t.Fatal(err)
	}
	if len(activePage.Items) != 1 || activePage.Items[0]["id"] != queuedJob.ID.String() {
		t.Fatalf("active=true 应只返回活动任务: %+v", activePage.Items)
	}

	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/jobs/"+failedJob.ID.String(), nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get job status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, jobPath, http.StatusOK, rec.Body.Bytes())
	var failedDTO map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &failedDTO); err != nil {
		t.Fatal(err)
	}
	errObj, _ := failedDTO["error"].(map[string]any)
	if errObj["code"] != "claude_timeout" || errObj["retryable"] != true {
		t.Fatalf("失败错误语义不稳定: %+v", errObj)
	}

	rec = apitest.Do(t, engine, http.MethodGet,
		"/api/v1/jobs/"+failedJob.ID.String()+"/events?limit=1", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list events status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodGet, jobEventsPath, http.StatusOK, rec.Body.Bytes())
	var eventsPage struct {
		Items    []map[string]any `json:"items"`
		PageInfo httpx.PageInfo   `json:"page_info"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &eventsPage); err != nil {
		t.Fatal(err)
	}
	if len(eventsPage.Items) != 1 || !eventsPage.PageInfo.HasMore || eventsPage.PageInfo.NextCursor == nil {
		t.Fatalf("事件第一页应带 next_cursor: %+v", eventsPage)
	}
	if _, leaked := eventsPage.Items[0]["status"]; leaked {
		t.Fatalf("REST 审计 JobEvent 不能伪装成 SSE JobStreamData: %+v", eventsPage.Items[0])
	}

	rec = apitest.Do(t, engine, http.MethodPost,
		"/api/v1/jobs/"+queuedJob.ID.String()+"/cancel",
		map[string]string{"Idempotency-Key": uuid.NewString()}, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("cancel status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodPost, jobCancelPath, http.StatusAccepted, rec.Body.Bytes())

	rec = apitest.Do(t, engine, http.MethodPost,
		"/api/v1/jobs/"+failedJob.ID.String()+"/retry",
		map[string]string{"Idempotency-Key": uuid.NewString()}, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("retry status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract.ValidateJSONResponse(t, http.MethodPost, jobRetryPath, http.StatusAccepted, rec.Body.Bytes())

	rec = apitest.Do(t, engine, http.MethodGet, jobsPath+"?active=true&status=queued", nil, "")
	contract.ValidateProblem(t, rec.Body.Bytes(), http.StatusBadRequest, "bad_request")
}

func TestJobSSEHeartbeatHasNoID(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	server := httptest.NewServer(engine)
	defer server.Close()

	resp := openJobStream(t, server.URL, nil)
	defer resp.Body.Close()
	frame := readNextSSEFrame(t, resp, false)
	if frame.Event != "heartbeat" {
		t.Fatalf("首帧 event=%q，期望 heartbeat", frame.Event)
	}
	if frame.ID != "" {
		t.Fatalf("heartbeat 不应携带 SSE id，得到 %q", frame.ID)
	}
	if !strings.Contains(frame.Data, "server_time") {
		t.Fatalf("heartbeat data 缺少 server_time: %s", frame.Data)
	}
}

func TestJobSSEReplaysPersistedEventsAsStreamData(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	repo := &job.Repository{}
	j := createHTTPJob(t, pool, job.KindLessonImport, "lesson")
	events, err := repo.ListEvents(context.Background(), pool, j.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	firstID := events[0].ID
	claimHTTPJob(t, pool)
	if err := repo.UpdateProgress(context.Background(), pool,
		j.ID, workerForTests, "chunking", 40, "正在切片"); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(engine)
	defer server.Close()
	resp := openJobStream(t, server.URL, map[string]string{"Last-Event-ID": intString(firstID)})
	defer resp.Body.Close()

	started := readNextJobFrame(t, resp)
	progress := readNextJobFrame(t, resp)
	startedID := parseSSEID(t, started)
	progressID := parseSSEID(t, progress)
	if startedID <= firstID || progressID <= startedID {
		t.Fatalf("回放 id 未单调递增: first=%d started=%d progress=%d", firstID, startedID, progressID)
	}
	if started.Event != job.EventStarted || progress.Event != job.EventProgress {
		t.Fatalf("回放事件顺序错误: %+v then %+v", started, progress)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(progress.Data), &data); err != nil {
		t.Fatal(err)
	}
	if data["job_id"] != j.ID.String() || data["status"] != job.StatusRunning || data["stage"] != "chunking" {
		t.Fatalf("PersistedJobStreamEvent.data 不符合 JobStreamData: %+v", data)
	}
	if strings.Contains(progress.Data, "fixture") || strings.Contains(progress.Data, "answer") {
		t.Fatalf("SSE data 泄露了 payload/答案类字段: %s", progress.Data)
	}
}

func TestJobSSEResyncRequiredForExpiredLastEventID(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	repo := &job.Repository{}
	oldJob := createHTTPJob(t, pool, job.KindLessonImport, "lesson")
	oldEvents, err := repo.ListEvents(context.Background(), pool, oldJob.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE job_events SET created_at = now() - interval '31 days' WHERE id = $1`, oldEvents[0].ID); err != nil {
		t.Fatal(err)
	}
	freshJob := createHTTPJob(t, pool, job.KindSummaryEvaluate, "lesson")
	freshEvents, err := repo.ListEvents(context.Background(), pool, freshJob.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	beforeCount := countJobEvents(t, pool)

	server := httptest.NewServer(engine)
	defer server.Close()
	resp := openJobStream(t, server.URL, map[string]string{"Last-Event-ID": intString(oldEvents[0].ID)})
	defer resp.Body.Close()

	frame := readNextSSEFrame(t, resp, false)
	if frame.Event != "stream.resync_required" {
		t.Fatalf("event=%q，期望 stream.resync_required；frame=%+v", frame.Event, frame)
	}
	if frame.ID != intString(freshEvents[0].ID) {
		t.Fatalf("resync SSE id=%q，期望高水位 %d", frame.ID, freshEvents[0].ID)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(frame.Data), &data); err != nil {
		t.Fatal(err)
	}
	if data["reason"] != "history_expired" ||
		data["high_watermark_event_id"] != float64(freshEvents[0].ID) ||
		data["active_jobs_url"] != "/api/v1/jobs?active=true" {
		t.Fatalf("resync data 不稳定: %+v", data)
	}
	if afterCount := countJobEvents(t, pool); afterCount != beforeCount {
		t.Fatalf("resync 不能写入 job_events: before=%d after=%d", beforeCount, afterCount)
	}
}

func TestJobSSESlowConsumerContinuesFromDatabaseCursor(t *testing.T) {
	pool := databasetest.New(t)
	engine := newJobEngine(t, pool)
	repo := &job.Repository{}
	j := createHTTPJob(t, pool, job.KindLessonImport, "lesson")

	server := httptest.NewServer(engine)
	defer server.Close()
	resp := openJobStream(t, server.URL, map[string]string{"Last-Event-ID": "0"})
	defer resp.Body.Close()

	queued := readNextJobFrame(t, resp)
	if queued.Event != job.EventQueued {
		t.Fatalf("首个持久化事件应为 queued: %+v", queued)
	}

	claimHTTPJob(t, pool)
	if err := repo.UpdateProgress(context.Background(), pool,
		j.ID, workerForTests, "extracting", 25, "正在提取"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateProgress(context.Background(), pool,
		j.ID, workerForTests, "validating", 90, "正在校验"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(context.Background(), pool, j.ID, workerForTests, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}

	var got []sseFrame
	for len(got) < 4 {
		got = append(got, readNextJobFrame(t, resp))
	}
	for i := 1; i < len(got); i++ {
		if parseSSEID(t, got[i]) <= parseSSEID(t, got[i-1]) {
			t.Fatalf("慢消费者事件乱序: %+v", got)
		}
	}
	if got[len(got)-1].Event != job.EventSucceeded {
		t.Fatalf("最后一帧应为 succeeded: %+v", got)
	}
}

type sseFrame struct {
	ID    string
	Event string
	Data  string
}

type jobStreamResponse struct {
	*http.Response
	reader *bufio.Reader
}

func openJobStream(t *testing.T, baseURL string, headers map[string]string) *jobStreamResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+jobStreamPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		t.Fatalf("stream status=%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		defer resp.Body.Close()
		t.Fatalf("Content-Type=%q", ct)
	}
	if resp.Header.Get("Cache-Control") != "no-cache" {
		defer resp.Body.Close()
		t.Fatalf("Cache-Control=%q", resp.Header.Get("Cache-Control"))
	}
	return &jobStreamResponse{Response: resp, reader: bufio.NewReader(resp.Body)}
}

func readNextJobFrame(t *testing.T, resp *jobStreamResponse) sseFrame {
	t.Helper()
	for {
		frame := readNextSSEFrame(t, resp, true)
		if strings.HasPrefix(frame.Event, "job.") {
			return frame
		}
	}
}

func readNextSSEFrame(t *testing.T, resp *jobStreamResponse, allowHeartbeat bool) sseFrame {
	t.Helper()
	for {
		var frame sseFrame
		for {
			line, err := resp.reader.ReadString('\n')
			if err != nil {
				t.Fatalf("读取 SSE 帧失败: %v", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			switch {
			case strings.HasPrefix(line, "id:"):
				frame.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "event:"):
				frame.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				frame.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
		if frame.Event == "heartbeat" && allowHeartbeat {
			continue
		}
		return frame
	}
}

func parseSSEID(t *testing.T, frame sseFrame) int64 {
	t.Helper()
	var n int64
	if err := json.NewDecoder(strings.NewReader(frame.ID)).Decode(&n); err != nil {
		t.Fatalf("SSE id 不是整数 %q: %v", frame.ID, err)
	}
	return n
}

func countJobEvents(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM job_events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func intString(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
