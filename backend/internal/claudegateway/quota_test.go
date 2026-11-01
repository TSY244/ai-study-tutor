package claudegateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
)

func TestQuotaExhaustionBlocksKeyAllowsFailoverAndRecoversThroughManualTest(t *testing.T) {
	key := routerTestKey("quota-key", 1, 100)
	backup := routerTestKey("backup-key", 2, 100)
	exhausted := true
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if exhausted && r.Header.Get("x-api-key") == key.APIKey {
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(403)
			_, _ = io.WriteString(w, `{"error":{"code":"insufficient_user_quota","type":"new_api_error","message":"balance -0.130036 token `+key.APIKey+`"}}`)
			return
		}
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	router, state := newTestRouter(t, []Key{key}, server)
	_, err := router.GenerateForKey(context.Background(), key.ID, routerTestRequest())
	if err == nil || aipkg.AsProviderError(err).Code != aipkg.ErrQuotaExhausted || aipkg.AsProviderError(err).Retryable {
		t.Fatalf("quota misclassified: %v", err)
	}
	if state.Snapshot(key).Status != StatusQuotaExhausted || state.Eligible(key, state.Now().Add(24*time.Hour)) {
		t.Fatal("quota must remain blocked until the account is fixed and manually tested")
	}
	_, err = router.Generate(context.Background(), routerTestRequest())
	if err == nil || aipkg.AsProviderError(err).Code != aipkg.ErrQuotaExhausted || calls != 1 {
		t.Fatal("blocked quota key was called again or converted into transient/auth failure")
	}
	writeRouterKeys(t, router.Store, []Key{key, backup})
	if _, err = router.Generate(context.Background(), routerTestRequest()); err != nil || calls != 2 {
		t.Fatalf("backup failover: %v, calls=%d", err, calls)
	}
	raw, err := os.ReadFile(StatePathFor(router.Store.Path()))
	if err != nil || strings.Contains(string(raw), key.APIKey) || strings.Contains(string(raw), "-0.130036") {
		t.Fatal("state retained upstream balance or credentials")
	}
	exhausted = false
	if _, err = router.GenerateForKey(context.Background(), key.ID, routerTestRequest()); err != nil || state.Snapshot(key).Status != StatusHealthy {
		t.Fatalf("manual test must restore eligibility after quota is replenished: %v", err)
	}
}
