package idempotency_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/internal/platform/databasetest"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func beginInput(scope, key, body string) idempotency.BeginInput {
	return idempotency.BeginInput{
		Scope: scope, Key: key, Method: "POST",
		RouteTemplate: "/api/v1/courses", RequestHash: hashOf(body),
	}
}

// 回放/冲突/in_progress 全矩阵。
func TestBeginOutcomes(t *testing.T) {
	pool := databasetest.New(t)
	svc := idempotency.NewService(pool)
	ctx := context.Background()
	key := uuid.NewString()

	// 首次：fresh。
	rec, outcome, err := svc.Begin(ctx, beginInput("course:create", key, `{"title":"高数"}`))
	if err != nil || outcome != idempotency.OutcomeFresh {
		t.Fatalf("首次应为 fresh，得到 %s, %v", outcome, err)
	}

	// 未完成时同 Key 同体：in_progress。
	_, outcome, err = svc.Begin(ctx, beginInput("course:create", key, `{"title":"高数"}`))
	if err != nil || outcome != idempotency.OutcomeInProgress {
		t.Fatalf("处理中应为 in_progress，得到 %s, %v", outcome, err)
	}

	// 未完成时同 Key 异体：conflict。
	_, outcome, err = svc.Begin(ctx, beginInput("course:create", key, `{"title":"线代"}`))
	if err != nil || outcome != idempotency.OutcomeConflict {
		t.Fatalf("异体应为 conflict，得到 %s, %v", outcome, err)
	}

	// 完成（模拟业务同事务提交）。
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, tx, rec.ID, 201, []byte(`{"id":"c1"}`), nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// 完成后同体：replay，响应为首次保存内容。
	replayed, outcome, err := svc.Begin(ctx, beginInput("course:create", key, `{"title":"高数"}`))
	if err != nil || outcome != idempotency.OutcomeReplay {
		t.Fatalf("完成后同体应为 replay，得到 %s, %v", outcome, err)
	}
	if replayed.ResponseStatus == nil || *replayed.ResponseStatus != 201 {
		t.Fatalf("回放状态码不符: %+v", replayed.ResponseStatus)
	}
	var body map[string]string
	if err := json.Unmarshal(replayed.ResponseBody, &body); err != nil || body["id"] != "c1" {
		t.Fatalf("回放响应不符: %s", replayed.ResponseBody)
	}

	// 完成后异体：conflict。
	_, outcome, err = svc.Begin(ctx, beginInput("course:create", key, `{"title":"线代"}`))
	if err != nil || outcome != idempotency.OutcomeConflict {
		t.Fatalf("完成后异体应为 conflict，得到 %s, %v", outcome, err)
	}

	// 不同 scope 互不影响。
	_, outcome, err = svc.Begin(ctx, beginInput("material:upload", key, `{"title":"高数"}`))
	if err != nil || outcome != idempotency.OutcomeFresh {
		t.Fatalf("不同 scope 应为 fresh，得到 %s, %v", outcome, err)
	}
}

// 幂等并发：N 个并发 Begin 同 Key 同体，恰好一个 fresh，其余 in_progress。
func TestBeginConcurrentSingleWinner(t *testing.T) {
	pool := databasetest.New(t)
	svc := idempotency.NewService(pool)
	ctx := context.Background()
	key := uuid.NewString()

	const n = 16
	outcomes := make([]idempotency.Outcome, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, outcome, err := svc.Begin(ctx, beginInput("quiz:generate", key, `{"lesson_id":"l1"}`))
			if err != nil {
				t.Errorf("Begin 失败: %v", err)
				return
			}
			outcomes[i] = outcome
		}(i)
	}
	wg.Wait()
	fresh, inProgress := 0, 0
	for _, o := range outcomes {
		switch o {
		case idempotency.OutcomeFresh:
			fresh++
		case idempotency.OutcomeInProgress:
			inProgress++
		default:
			t.Errorf("意外 outcome: %s", o)
		}
	}
	if fresh != 1 || inProgress != n-1 {
		t.Fatalf("fresh=%d in_progress=%d，期望 1/%d", fresh, inProgress, n-1)
	}
}

// Abandon：事务前失败删除 in_progress，客户端可用原 Key 重试（fresh）。
func TestAbandonAllowsSafeRetry(t *testing.T) {
	pool := databasetest.New(t)
	svc := idempotency.NewService(pool)
	ctx := context.Background()
	key := uuid.NewString()

	rec, outcome, err := svc.Begin(ctx, beginInput("course:create", key, `{"title":"高数"}`))
	if err != nil || outcome != idempotency.OutcomeFresh {
		t.Fatal(err)
	}
	if err := svc.Abandon(ctx, rec.ID); err != nil {
		t.Fatal(err)
	}
	_, outcome, err = svc.Begin(ctx, beginInput("course:create", key, `{"title":"高数"}`))
	if err != nil || outcome != idempotency.OutcomeFresh {
		t.Fatalf("Abandon 后应可重新执行，得到 %s, %v", outcome, err)
	}
}

// 文件发布恢复：in_progress 记录在其 temporary 对象被文件 Recovery 清理后删除；
// 宽限期内的在途记录保留。
func TestRecoverInProgress(t *testing.T) {
	pool := databasetest.New(t)
	svc := idempotency.NewService(pool)
	ctx := context.Background()

	old := beginInput("material:upload", uuid.NewString(), `{"name":"a.pdf"}`)
	recOld, _, err := svc.Begin(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	// 模拟发布从未完成：记录已超宽限期，且无关联 temporary 对象。
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_records SET created_at = now() - interval '2 hours' WHERE id = $1`, recOld.ID); err != nil {
		t.Fatal(err)
	}
	// 宽限期内的在途记录。
	if _, _, err := svc.Begin(ctx, beginInput("material:upload", uuid.NewString(), `{"name":"b.pdf"}`)); err != nil {
		t.Fatal(err)
	}

	deleted, err := svc.RecoverInProgress(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("应删除 1 条超期 in_progress 记录，实际 %d", deleted)
	}
	var remaining int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM idempotency_records WHERE state = 'in_progress'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("宽限期内记录应保留，剩余 %d", remaining)
	}
}

// 过期清理。
func TestCleanupExpired(t *testing.T) {
	pool := databasetest.New(t)
	svc := idempotency.NewService(pool)
	ctx := context.Background()
	rec, _, err := svc.Begin(ctx, beginInput("course:create", uuid.NewString(), `{"title":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE idempotency_records SET expires_at = now() - interval '1 second' WHERE id = $1`, rec.ID); err != nil {
		t.Fatal(err)
	}
	deleted, err := svc.CleanupExpired(ctx)
	if err != nil || deleted != 1 {
		t.Fatalf("过期清理 = %d, %v，期望 1", deleted, err)
	}
}

// 并发同 Key 完成后 replay 一致性：所有并发方最终得到同一响应。
func TestConcurrentCompleteThenReplay(t *testing.T) {
	pool := databasetest.New(t)
	svc := idempotency.NewService(pool)
	ctx := context.Background()
	key := uuid.NewString()
	in := beginInput("attempt:submit", key, `{"version":3}`)

	rec, outcome, err := svc.Begin(ctx, in)
	if err != nil || outcome != idempotency.OutcomeFresh {
		t.Fatal(err)
	}
	tx, _ := pool.Begin(ctx)
	if err := svc.Complete(ctx, tx, rec.ID, 200, []byte(`{"status":"graded"}`), nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			replayed, outcome, err := svc.Begin(ctx, in)
			if err != nil || outcome != idempotency.OutcomeReplay {
				t.Errorf("并发 replay 失败: %s, %v", outcome, err)
				return
			}
			var body map[string]string
			if err := json.Unmarshal(replayed.ResponseBody, &body); err != nil || body["status"] != "graded" {
				t.Errorf("replay 响应不一致: %s", replayed.ResponseBody)
			}
		}()
	}
	wg.Wait()
}
