package claudegateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
)

type legacySpy struct {
	mu    sync.Mutex
	calls int
}

func (s *legacySpy) Generate(context.Context, aipkg.Request) (aipkg.Response, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return aipkg.Response{JSON: json.RawMessage(`{"legacy":true}`)}, nil
}

func (s *legacySpy) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type gatewayRoundTripFunc func(*http.Request) (*http.Response, error)

func (f gatewayRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func routerTestKey(id string, priority, weight int) Key {
	return Key{
		ID: id, Name: id, APIKey: syntheticSecret(id), Model: "claude-test",
		Enabled: true, Priority: priority, Weight: weight, TimeoutSeconds: 1,
		CooldownSeconds: 60, MaxConsecutiveFailures: 1,
	}
}

func routerTestRequest() aipkg.Request {
	return aipkg.Request{
		Purpose: aipkg.PurposeConnectivity,
		Schema:  json.RawMessage(`{"type":"object"}`),
	}
}

func writeRouterKeys(t *testing.T, store *Store, keys []Key) {
	t.Helper()
	raw, err := json.Marshal(configFile{Keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(store.Path(), raw); err != nil {
		t.Fatal(err)
	}
}

func newTestRouter(t *testing.T, keys []Key, server *httptest.Server) (*Router, *StateTracker) {
	t.Helper()
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "keys.json"))
	writeRouterKeys(t, store, keys)
	state := NewStateTracker(filepath.Join(dir, "state.json"), nil)
	router := &Router{Store: store, State: state, DefaultModel: "claude-test", MaxOutputTokens: 64}
	if server != nil {
		router.Endpoint = server.URL
		router.HTTPClient = server.Client()
	}
	return router, state
}

func writeAnthropicSuccess(w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json")
	_, _ = io.WriteString(w, `{"model":"claude-test","stop_reason":"tool_use","content":[{"type":"tool_use","name":"structured_result","input":{"ok":true}}]}`)
}

func TestRouterSkipsDisabledKeysAndUsesAscendingPriority(t *testing.T) {
	disabled := routerTestKey("disabled", 0, 100)
	disabled.Enabled = false
	later := routerTestKey("later", 20, 100)
	first := routerTestKey("first", 10, 100)
	var selected string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("x-api-key") {
		case first.APIKey:
			selected = first.ID
		case later.APIKey:
			selected = later.ID
		case disabled.APIKey:
			selected = disabled.ID
		}
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	router, _ := newTestRouter(t, []Key{disabled, later, first}, server)

	if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
		t.Fatal(err)
	}
	if selected != first.ID {
		t.Fatalf("selected key ID = %q, want lowest enabled priority", selected)
	}
}

func TestRouterUsesDeterministicWeightedRoundRobinAtSamePriority(t *testing.T) {
	heavy := routerTestKey("heavy", 10, 3)
	light := routerTestKey("light", 10, 1)
	sequence := make([]string, 0, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == heavy.APIKey {
			sequence = append(sequence, heavy.ID)
		} else {
			sequence = append(sequence, light.ID)
		}
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	router, _ := newTestRouter(t, []Key{heavy, light}, server)

	for i := 0; i < 8; i++ {
		if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{heavy.ID, heavy.ID, light.ID, heavy.ID, heavy.ID, heavy.ID, light.ID, heavy.ID}
	if strings.Join(sequence, ",") != strings.Join(want, ",") {
		t.Fatalf("weighted selection sequence = %v, want %v", sequence, want)
	}
}

func TestRouterUsesDeterministicWeightedRoundRobinAcrossThreeKeys(t *testing.T) {
	first := routerTestKey("weighted-first", 10, 5)
	second := routerTestKey("weighted-second", 10, 3)
	third := routerTestKey("weighted-third", 10, 2)
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, key := range []Key{first, second, third} {
			if r.Header.Get("x-api-key") == key.APIKey {
				counts[key.ID]++
				break
			}
		}
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	router, _ := newTestRouter(t, []Key{first, second, third}, server)

	for i := 0; i < 30; i++ {
		if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]int{first.ID: 15, second.ID: 9, third.ID: 6}
	for _, key := range []Key{first, second, third} {
		if counts[key.ID] != want[key.ID] {
			t.Fatalf("key %s selections = %d, want %d", key.ID, counts[key.ID], want[key.ID])
		}
	}
}

func TestRouterAdvancesWeightedRoundRobinWhenFailoverEntersLaterPriority(t *testing.T) {
	primary := routerTestKey("spillover-primary", 10, 1)
	primary.MaxConsecutiveFailures = 1000
	first := routerTestKey("spillover-first", 20, 5)
	second := routerTestKey("spillover-second", 20, 3)
	third := routerTestKey("spillover-third", 20, 2)
	primaryCalls := 0
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == primary.APIKey {
			primaryCalls++
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		for _, key := range []Key{first, second, third} {
			if r.Header.Get("x-api-key") == key.APIKey {
				counts[key.ID]++
				break
			}
		}
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	router, _ := newTestRouter(t, []Key{primary, first, second, third}, server)

	for i := 0; i < 30; i++ {
		if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
			t.Fatal(err)
		}
	}
	if primaryCalls != 30 {
		t.Fatalf("priority-10 attempts = %d, want 30", primaryCalls)
	}
	want := map[string]int{first.ID: 15, second.ID: 9, third.ID: 6}
	for _, key := range []Key{first, second, third} {
		if counts[key.ID] != want[key.ID] {
			t.Fatalf("spillover key %s selections = %d, want %d", key.ID, counts[key.ID], want[key.ID])
		}
	}
}

func TestRouterAttemptsEachEligibleKeyAtMostOnce(t *testing.T) {
	keys := []Key{
		routerTestKey("one", 10, 1),
		routerTestKey("two", 10, 1),
		routerTestKey("three", 20, 1),
	}
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, key := range keys {
			if r.Header.Get("x-api-key") == key.APIKey {
				counts[key.ID]++
			}
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	router, _ := newTestRouter(t, keys, server)

	_, err := router.Generate(context.Background(), routerTestRequest())
	if err == nil {
		t.Fatal("expected all-key failure")
	}
	for _, key := range keys {
		if strings.Contains(err.Error(), key.APIKey) {
			t.Fatalf("returned error exposed the secret for key %s", key.ID)
		}
		if counts[key.ID] != 1 {
			t.Fatalf("key %s attempt count = %d, want 1", key.ID, counts[key.ID])
		}
	}
}

func TestRouterMarksAuthenticationFailuresAndFailsOver(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			first := routerTestKey("auth-first", 10, 1)
			second := routerTestKey("auth-second", 20, 1)
			secondCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") == first.APIKey {
					w.WriteHeader(status)
					return
				}
				secondCalls++
				writeAnthropicSuccess(w)
			}))
			defer server.Close()
			router, state := newTestRouter(t, []Key{first, second}, server)

			if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
				t.Fatal(err)
			}
			if secondCalls != 1 {
				t.Fatalf("next eligible key calls = %d, want 1", secondCalls)
			}
			st := state.Snapshot(first)
			if st.Status != StatusAuthFailed || st.LastErrorCode == nil || *st.LastErrorCode != aipkg.ErrAuthFailed {
				t.Fatalf("authentication state = %+v", st)
			}
		})
	}
}

func TestRouterCoolsTransientFailuresAndFailsOver(t *testing.T) {
	tests := []struct {
		name      string
		firstCall func(http.ResponseWriter, *http.Request)
		client    func(*httptest.Server, Key) *http.Client
		wantCode  string
	}{
		{
			name: "rate limit",
			firstCall: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantCode: aipkg.ErrRateLimited,
		},
		{
			name: "server error",
			firstCall: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			wantCode: aipkg.ErrOverloaded,
		},
		{
			name: "timeout",
			firstCall: func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(75 * time.Millisecond)
				writeAnthropicSuccess(w)
			},
			client: func(server *httptest.Server, _ Key) *http.Client {
				client := server.Client()
				client.Timeout = 20 * time.Millisecond
				return client
			},
			wantCode: aipkg.ErrTimeout,
		},
		{
			name:      "network",
			firstCall: func(http.ResponseWriter, *http.Request) {},
			client: func(server *httptest.Server, first Key) *http.Client {
				base := server.Client().Transport
				return &http.Client{Transport: gatewayRoundTripFunc(func(r *http.Request) (*http.Response, error) {
					if r.Header.Get("x-api-key") == first.APIKey {
						return nil, errors.New("synthetic network failure")
					}
					return base.RoundTrip(r)
				})}
			},
			wantCode: aipkg.ErrUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := routerTestKey("transient-first", 10, 1)
			second := routerTestKey("transient-second", 20, 1)
			secondCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") == first.APIKey {
					tc.firstCall(w, r)
					return
				}
				secondCalls++
				writeAnthropicSuccess(w)
			}))
			defer server.Close()
			router, state := newTestRouter(t, []Key{first, second}, server)
			if tc.client != nil {
				router.HTTPClient = tc.client(server, first)
			}

			if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
				t.Fatal(err)
			}
			if secondCalls != 1 {
				t.Fatalf("next eligible key calls = %d, want 1", secondCalls)
			}
			st := state.Snapshot(first)
			if st.Status != StatusCoolingDown || st.LastErrorCode == nil || *st.LastErrorCode != tc.wantCode {
				t.Fatalf("transient failure state = %+v", st)
			}
		})
	}
}

func TestRouterCancellationDoesNotPoisonKeyState(t *testing.T) {
	key := routerTestKey("cancelled", 10, 1)
	router, state := newTestRouter(t, []Key{key}, nil)
	router.Endpoint = "https://anthropic.test/v1/messages"
	router.HTTPClient = &http.Client{Transport: gatewayRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := router.Generate(ctx, routerTestRequest()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate error = %v, want context.Canceled", err)
	}
	st := state.Snapshot(key)
	if st.Status != StatusUnknown || st.ConsecutiveFailures != 0 ||
		st.LastFailureAt != nil || st.LastErrorCode != nil || st.CooldownUntil != nil {
		t.Fatalf("cancelled request changed key state: %+v", st)
	}
}

func TestRouterGenerateForKeyCancellationDoesNotPoisonKeyState(t *testing.T) {
	key := routerTestKey("target-cancelled", 10, 1)
	router, state := newTestRouter(t, []Key{key}, nil)
	router.Endpoint = "https://anthropic.test/v1/messages"
	router.HTTPClient = &http.Client{Transport: gatewayRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := router.GenerateForKey(ctx, key.ID, routerTestRequest()); !errors.Is(err, context.Canceled) {
		t.Fatalf("GenerateForKey error = %v, want context.Canceled", err)
	}
	st := state.Snapshot(key)
	if st.Status == StatusCoolingDown || st.Status == StatusAuthFailed ||
		st.ConsecutiveFailures != 0 || st.LastFailureAt != nil || st.LastErrorCode != nil {
		t.Fatalf("cancelled targeted request poisoned key state: %+v", st)
	}
}

func TestRouterDoesNotFailOverModelOutputFailures(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"refusal", `{"model":"claude-test","stop_reason":"refusal","content":[]}`, aipkg.ErrRefused},
		{"max tokens", `{"model":"claude-test","stop_reason":"max_tokens","content":[]}`, aipkg.ErrOutputTruncated},
		{"invalid JSON", `{"broken"`, aipkg.ErrInvalidJSON},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := routerTestKey("output-first", 10, 1)
			second := routerTestKey("output-second", 20, 1)
			secondCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") == first.APIKey {
					_, _ = io.WriteString(w, tc.body)
					return
				}
				secondCalls++
				writeAnthropicSuccess(w)
			}))
			defer server.Close()
			router, state := newTestRouter(t, []Key{first, second}, server)

			_, err := router.Generate(context.Background(), routerTestRequest())
			if got := aipkg.AsProviderError(err).Code; got != tc.wantCode {
				t.Fatalf("error code = %q, want %q", got, tc.wantCode)
			}
			if secondCalls != 0 {
				t.Fatalf("non-failover error attempted the next key %d times", secondCalls)
			}
			st := state.Snapshot(first)
			if st.Status != StatusUnknown || st.LastErrorCode == nil || *st.LastErrorCode != tc.wantCode {
				t.Fatalf("non-failover state = %+v", st)
			}
		})
	}
}

func TestStateCooldownExpires(t *testing.T) {
	state := NewStateTracker(filepath.Join(t.TempDir(), "state.json"), nil)
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	state.now = func() time.Time { return now }
	key := routerTestKey("cooldown", 10, 1)
	key.CooldownSeconds = 30
	state.MarkTransientFailure(key, aipkg.ErrRateLimited, time.Millisecond)
	if state.Eligible(key, now) {
		t.Fatal("key remained eligible during cooldown")
	}
	now = now.Add(31 * time.Second)
	if !state.Eligible(key, now) {
		t.Fatal("key did not become eligible after cooldown expiry")
	}
	st := state.Snapshot(key)
	if st.Status != StatusUnknown || st.CooldownUntil != nil {
		t.Fatalf("expired cooldown state = %+v", st)
	}
}

func TestStateCooldownExpiryPersistsNormalizedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	state := NewStateTracker(path, nil)
	state.now = func() time.Time { return now }
	key := routerTestKey("persisted-cooldown", 10, 1)
	key.CooldownSeconds = 30
	state.MarkTransientFailure(key, aipkg.ErrRateLimited, time.Millisecond)
	now = now.Add(31 * time.Second)
	if got := state.Snapshot(key); got.Status != StatusUnknown || got.CooldownUntil != nil {
		t.Fatalf("expired cooldown view = %+v", got)
	}

	fresh := NewStateTracker(path, nil)
	fresh.now = func() time.Time { return now }
	fresh.mu.Lock()
	fresh.reloadLocked()
	persisted := fresh.states[key.ID]
	fresh.mu.Unlock()
	if persisted.Status != StatusUnknown || persisted.CooldownUntil != nil {
		t.Fatalf("expired cooldown remained persisted: %+v", persisted)
	}
}

func TestStateResetAfterSecretReplacementClearsFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	store := NewStore(path)
	created, err := store.Create(routerTestKey("replacement", 10, 1))
	if err != nil {
		t.Fatal(err)
	}
	state := NewStateTracker(StatePathFor(path), nil)
	state.MarkAuthFailed(created.ID, aipkg.ErrAuthFailed, time.Millisecond)
	if state.Eligible(created, state.Now()) {
		t.Fatal("auth-failed key remained eligible before replacement")
	}
	replacement := syntheticSecret("replacement-new")
	updated, err := store.Update(created.ID, KeyPatch{APIKey: &replacement})
	if err != nil {
		t.Fatal(err)
	}
	state.Reset(created.ID)
	if updated.APIKey != replacement {
		t.Fatal("store did not retain the replacement secret")
	}
	if !state.Eligible(updated, state.Now()) || state.Snapshot(updated).Status != StatusUnknown {
		t.Fatal("State.Reset did not recover the replacement key")
	}
}

func TestStateTrackersPreserveConcurrentUpdatesAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	firstTracker := NewStateTracker(path, nil)
	secondTracker := NewStateTracker(path, nil)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstTracker.beforePersist = func() {
		close(firstStarted)
		<-releaseFirst
	}

	firstDone := make(chan struct{})
	go func() {
		firstTracker.MarkSuccess("first", time.Millisecond)
		close(firstDone)
	}()
	<-firstStarted
	secondTracker.MarkSuccess("second", 2*time.Millisecond)
	close(releaseFirst)
	<-firstDone

	fresh := NewStateTracker(path, nil)
	firstState := fresh.Snapshot(Key{ID: "first", Enabled: true})
	secondState := fresh.Snapshot(Key{ID: "second", Enabled: true})
	if firstState.Status != StatusHealthy || secondState.Status != StatusHealthy {
		t.Fatalf("concurrent states were not preserved: first=%+v second=%+v", firstState, secondState)
	}
}

func TestStateTrackersSerializeSameKeyFailureMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	firstTracker := NewStateTracker(path, nil)
	secondTracker := NewStateTracker(path, nil)
	key := routerTestKey("shared", 10, 1)
	key.MaxConsecutiveFailures = 2
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstTracker.beforePersist = func() {
		close(firstStarted)
		<-releaseFirst
	}

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		firstTracker.MarkTransientFailure(key, aipkg.ErrRateLimited, time.Millisecond)
	}()
	<-firstStarted
	secondTracker.MarkTransientFailure(key, aipkg.ErrRateLimited, 2*time.Millisecond)
	close(releaseFirst)
	<-firstDone

	state := NewStateTracker(path, nil).Snapshot(key)
	if state.ConsecutiveFailures != 2 || state.Status != StatusCoolingDown {
		t.Fatalf("same-key concurrent failures = %+v, want count=2 cooling_down", state)
	}
}

func TestRouterGenerateForKeySuccessRecoversAuthFailedState(t *testing.T) {
	key := routerTestKey("manual", 10, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	router, state := newTestRouter(t, []Key{key}, server)
	state.MarkAuthFailed(key.ID, aipkg.ErrAuthFailed, time.Millisecond)

	if _, err := router.GenerateForKey(context.Background(), key.ID, routerTestRequest()); err != nil {
		t.Fatal(err)
	}
	st := state.Snapshot(key)
	if st.Status != StatusHealthy || st.ConsecutiveFailures != 0 || st.LastSuccessAt == nil {
		t.Fatalf("manual test success state = %+v", st)
	}
}

func TestRouterDiscardsOutcomeFromSecretReplacedWhileInFlight(t *testing.T) {
	key := routerTestKey("in-flight", 10, 1)
	requestStarted := make(chan struct{})
	releaseResponse := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseResponse
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	router, state := newTestRouter(t, []Key{key}, server)
	result := make(chan error, 1)
	go func() {
		_, err := router.Generate(context.Background(), routerTestRequest())
		result <- err
	}()
	<-requestStarted

	replacement := syntheticSecret("in-flight-replacement")
	if _, err := router.Store.Update(key.ID, KeyPatch{APIKey: &replacement}); err != nil {
		t.Fatal(err)
	}
	state.Reset(key.ID)
	close(releaseResponse)
	if err := <-result; aipkg.AsProviderError(err).Code != aipkg.ErrAuthFailed {
		t.Fatalf("in-flight result error = %v, want auth failure", err)
	}
	updated, err := router.Store.Get(key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Snapshot(updated).Status != StatusUnknown {
		t.Fatalf("old in-flight outcome changed replacement state: %+v", state.Snapshot(updated))
	}
}

func TestRouterGenerateForKeyDiscardsOutcomeFromSecretReplacedWhileInFlight(t *testing.T) {
	key := routerTestKey("target-replaced-in-flight", 10, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"type":"authentication_error"}}`)
	}))
	defer server.Close()
	router, state := newTestRouter(t, []Key{key}, server)

	done := make(chan error, 1)
	go func() {
		_, err := router.GenerateForKey(context.Background(), key.ID, routerTestRequest())
		done <- err
	}()
	<-started
	replacement := syntheticSecret("target-replaced-new")
	updated, err := router.Store.Update(key.ID, KeyPatch{APIKey: &replacement})
	if err != nil {
		t.Fatal(err)
	}
	state.Reset(key.ID)
	close(release)
	if err := <-done; aipkg.AsProviderError(err).Code != aipkg.ErrAuthFailed {
		t.Fatalf("GenerateForKey error = %v, want auth failure from old request", err)
	}

	got := state.Snapshot(updated)
	if got.Status != StatusUnknown || got.LastErrorCode != nil || got.ConsecutiveFailures != 0 {
		t.Fatalf("old targeted outcome poisoned replacement state: %+v", got)
	}
}

func TestRouterUsesLegacyOnlyWithoutEnabledConfiguredKeys(t *testing.T) {
	t.Run("missing config", func(t *testing.T) {
		legacy := &legacySpy{}
		dir := t.TempDir()
		router := &Router{
			Store:  NewStore(filepath.Join(dir, "missing.json")),
			State:  NewStateTracker(filepath.Join(dir, "state.json"), nil),
			Legacy: legacy,
		}
		if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
			t.Fatal(err)
		}
		if legacy.callCount() != 1 {
			t.Fatalf("legacy calls = %d, want 1", legacy.callCount())
		}
	})

	t.Run("no enabled keys", func(t *testing.T) {
		key := routerTestKey("disabled-only", 10, 1)
		key.Enabled = false
		router, _ := newTestRouter(t, []Key{key}, nil)
		legacy := &legacySpy{}
		router.Legacy = legacy
		if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
			t.Fatal(err)
		}
		if legacy.callCount() != 1 {
			t.Fatalf("legacy calls = %d, want 1", legacy.callCount())
		}
	})

	tests := []struct {
		name  string
		block func(*StateTracker, Key)
	}{
		{
			name: "all auth failed",
			block: func(state *StateTracker, key Key) {
				state.MarkAuthFailed(key.ID, aipkg.ErrAuthFailed, time.Millisecond)
			},
		},
		{
			name: "all cooling",
			block: func(state *StateTracker, key Key) {
				state.MarkTransientFailure(key, aipkg.ErrRateLimited, time.Millisecond)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := routerTestKey("blocked", 10, 1)
			router, state := newTestRouter(t, []Key{key}, nil)
			legacy := &legacySpy{}
			router.Legacy = legacy
			tc.block(state, key)
			if _, err := router.Generate(context.Background(), routerTestRequest()); err == nil {
				t.Fatal("expected no-eligible-key error")
			}
			if legacy.callCount() != 0 {
				t.Fatalf("legacy used %d times despite an enabled configured key", legacy.callCount())
			}
		})
	}

	t.Run("malformed config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "keys.json")
		if err := os.WriteFile(path, []byte(`{"keys":`), 0o600); err != nil {
			t.Fatal(err)
		}
		legacy := &legacySpy{}
		router := &Router{
			Store: NewStore(path), State: NewStateTracker(filepath.Join(dir, "state.json"), nil), Legacy: legacy,
		}
		if _, err := router.Generate(context.Background(), routerTestRequest()); err == nil {
			t.Fatal("expected malformed-config error")
		}
		if legacy.callCount() != 0 {
			t.Fatalf("legacy used %d times for malformed gateway config", legacy.callCount())
		}
	})
}

// claudeURLRouterRequest 构造一次 lesson_url_extract 请求（Web Fetch 路径）。
func claudeURLRouterRequest(t *testing.T) aipkg.Request {
	t.Helper()
	req, err := aipkg.BuildLessonURLExtractRequest(aipkg.LessonURLExtractPromptInput{
		URL:            "https://www.youtube.com/watch?v=abc",
		AllowedDomains: []string{"www.youtube.com", "youtube.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// TestRouterDoesNotFailOverMaterialFetchFailures 证明来源访问错误
// （material_fetch_failed，无论可重试与否）不是 Key 故障：不尝试下一个
// Key、不把选中 Key 置 cooling/auth-failed，retryability 对调用方可见。
func TestRouterDoesNotFailOverMaterialFetchFailures(t *testing.T) {
	structured := `{"type":"tool_use","id":"toolu_1","name":"structured_result","input":{"title":"t","raw_text_markdown":"x","extraction_note":"n"}}`
	canonicalUse := `{"type":"server_tool_use","id":"srvtoolu_1","name":"web_fetch","input":{"url":"https://www.youtube.com/watch?v=abc"}}`
	tests := []struct {
		name          string
		content       string
		wantRetryable bool
	}{
		{
			name: "retryable temporary fetch error",
			content: `[` + canonicalUse + `,` +
				`{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_1","content":{"type":"web_fetch_tool_error","error_code":"too_many_requests"}},` +
				structured + `]`,
			wantRetryable: true,
		},
		{
			name:          "non-retryable missing fetch evidence",
			content:       `[` + structured + `]`,
			wantRetryable: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := routerTestKey("fetch-first", 10, 1)
			second := routerTestKey("fetch-second", 20, 1)
			secondCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-api-key") == first.APIKey {
					w.Header().Set("content-type", "application/json")
					_, _ = io.WriteString(w, `{"model":"claude-test","stop_reason":"tool_use","content":`+tc.content+`}`)
					return
				}
				secondCalls++
				writeAnthropicSuccess(w)
			}))
			defer server.Close()
			router, state := newTestRouter(t, []Key{first, second}, server)

			_, err := router.Generate(context.Background(), claudeURLRouterRequest(t))
			perr := aipkg.AsProviderError(err)
			if perr.Code != aipkg.ErrMaterialFetchFailed {
				t.Fatalf("error code = %q, want %q (err=%v)", perr.Code, aipkg.ErrMaterialFetchFailed, err)
			}
			if perr.Retryable != tc.wantRetryable {
				t.Fatalf("retryable = %v, want %v：Job runner 必须能看到 retryability", perr.Retryable, tc.wantRetryable)
			}
			if secondCalls != 0 {
				t.Fatalf("material_fetch_failed 不应故障转移，却尝试了下一个 Key %d 次", secondCalls)
			}
			st := state.Snapshot(first)
			if st.Status == StatusCoolingDown || st.Status == StatusAuthFailed {
				t.Fatalf("material_fetch_failed 不应把 Key 置 cooling/auth-failed: %+v", st)
			}
			if st.LastErrorCode == nil || *st.LastErrorCode != aipkg.ErrMaterialFetchFailed {
				t.Fatalf("Key 末次错误码应记录 material_fetch_failed: %+v", st)
			}
		})
	}
}

// TestGatewayOnlyURLImportNeverUsesLegacy 证明 URL 专用 Provider 只用网关
// 启用 Key：无启用 Key 时 claude_not_configured 且不发 HTTP、不回退 Legacy；
// 有启用 Key 时用该 Key；请求前移除 Key 再次失败且无 legacy 回退。
func TestGatewayOnlyURLImportNeverUsesLegacy(t *testing.T) {
	t.Run("no enabled keys returns claude_not_configured without HTTP or legacy", func(t *testing.T) {
		legacy := &legacySpy{}
		dir := t.TempDir()
		router := &Router{
			Store:  NewStore(filepath.Join(dir, "keys.json")),
			State:  NewStateTracker(filepath.Join(dir, "state.json"), nil),
			Legacy: legacy,
		}
		router.HTTPClient = &http.Client{Transport: gatewayRoundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("没有启用 Key 时 GatewayOnly 不得发起 HTTP")
			return nil, errors.New("unexpected http call")
		})}

		_, err := router.GatewayOnly().Generate(context.Background(), routerTestRequest())
		if got := aipkg.AsProviderError(err).Code; got != aipkg.ErrNotConfigured {
			t.Fatalf("error code = %q, want %q", got, aipkg.ErrNotConfigured)
		}
		if legacy.callCount() != 0 {
			t.Fatalf("GatewayOnly 回退了 Legacy %d 次", legacy.callCount())
		}
	})

	t.Run("enabled key serves the request", func(t *testing.T) {
		key := routerTestKey("gateway-only", 10, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-api-key") != key.APIKey {
				t.Fatal("GatewayOnly 使用了配置之外的 Key")
			}
			writeAnthropicSuccess(w)
		}))
		defer server.Close()
		router, _ := newTestRouter(t, []Key{key}, server)
		legacy := &legacySpy{}
		router.Legacy = legacy

		if _, err := router.GatewayOnly().Generate(context.Background(), routerTestRequest()); err != nil {
			t.Fatal(err)
		}
		if legacy.callCount() != 0 {
			t.Fatalf("有启用 Key 时 GatewayOnly 不应触碰 Legacy，调用 %d 次", legacy.callCount())
		}
	})

	t.Run("key removed before request fails without legacy fallback", func(t *testing.T) {
		key := routerTestKey("removed", 10, 1)
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("Key 移除后 GatewayOnly 不得发起 HTTP（含 legacy 回退）")
		}))
		defer server.Close()
		router, _ := newTestRouter(t, []Key{key}, server)
		legacy := &legacySpy{}
		router.Legacy = legacy
		if err := router.Store.Delete(key.ID); err != nil {
			t.Fatal(err)
		}

		_, err := router.GatewayOnly().Generate(context.Background(), routerTestRequest())
		if got := aipkg.AsProviderError(err).Code; got != aipkg.ErrNotConfigured {
			t.Fatalf("error code = %q, want %q", got, aipkg.ErrNotConfigured)
		}
		if legacy.callCount() != 0 {
			t.Fatalf("Key 移除后 GatewayOnly 回退了 Legacy %d 次", legacy.callCount())
		}
	})
}

// TestRouterAttemptUsesKeyBaseURL：Key.base_url 优先于 Router.Endpoint；
// 基础 URL 自动补全 /v1/messages 后缀。同优先级同权重按文件顺序首选，
// srvA 的 Key 必须排第一以保证断言确定；httptest 是 http —— 绕开 handler
// 校验直接写 keys.json（store 层不校验 scheme）。
func TestRouterAttemptUsesKeyBaseURL(t *testing.T) {
	var mu sync.Mutex
	var pathsA, pathsB []string
	mk := func(paths *[]string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			*paths = append(*paths, r.URL.Path)
			mu.Unlock()
			writeAnthropicSuccess(w)
		}))
	}
	srvA := mk(&pathsA)
	srvB := mk(&pathsB)
	defer srvA.Close()
	defer srvB.Close()

	keyA := routerTestKey("base-url-a", 10, 1)
	keyA.BaseURL = srvA.URL
	keyB := routerTestKey("base-url-b", 10, 1)
	keyB.BaseURL = srvB.URL
	router, _ := newTestRouter(t, []Key{keyA, keyB}, nil)
	router.Endpoint = "http://unused.example/v1/messages"

	if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
		t.Fatal(err)
	}
	if len(pathsA) != 1 || pathsA[0] != "/v1/messages" {
		t.Fatalf("应请求 Key 的 base_url 并补全 /v1/messages: %v", pathsA)
	}
	if len(pathsB) != 0 {
		t.Fatalf("不应请求第二个 Key 的 base_url: %v", pathsB)
	}
}

// TestRouterAttemptFallsBackToRouterEndpoint：Key 无 base_url 时原样使用
// Router.Endpoint（进程级默认端点，行为与现状一致）。
func TestRouterAttemptFallsBackToRouterEndpoint(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	router, _ := newTestRouter(t, []Key{routerTestKey("no-base-url", 10, 1)}, server)
	router.Endpoint = server.URL + "/v1/messages"

	if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/v1/messages" {
		t.Fatalf("Key 无 base_url 时应回退 Router.Endpoint 原样端点: %v", paths)
	}
}

// TestRouterGenerateForKeyUsesKeyBaseURL：GenerateForKey 经 attempt 继承
// Key 的 base_url。
func TestRouterGenerateForKeyUsesKeyBaseURL(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		writeAnthropicSuccess(w)
	}))
	defer server.Close()
	key := routerTestKey("target-base-url", 10, 1)
	key.BaseURL = server.URL
	router, _ := newTestRouter(t, []Key{key}, nil)
	router.Endpoint = "http://unused.example/v1/messages"

	if _, err := router.GenerateForKey(context.Background(), key.ID, routerTestRequest()); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/v1/messages" {
		t.Fatalf("GenerateForKey 应使用 Key 的 base_url: %v", paths)
	}
}

// TestRouterFailoverReResolvesEndpointPerAttempt：故障转移时端点按各 Key
// 的 base_url 逐次重解析 —— 首选 Key 的 base_url 返回 503（可转移错误，
// 对齐既有 server error 用例）后，下一个 Key 的请求发向它自己的 base_url。
func TestRouterFailoverReResolvesEndpointPerAttempt(t *testing.T) {
	var mu sync.Mutex
	var pathsA, pathsB []string
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pathsA = append(pathsA, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pathsB = append(pathsB, r.URL.Path)
		mu.Unlock()
		writeAnthropicSuccess(w)
	}))
	defer srvB.Close()

	// 同优先级同权重按文件顺序首选：srvA 的 Key 排第一，先 503 再转移。
	keyA := routerTestKey("failover-base-url-a", 10, 1)
	keyA.BaseURL = srvA.URL
	keyB := routerTestKey("failover-base-url-b", 10, 1)
	keyB.BaseURL = srvB.URL
	router, _ := newTestRouter(t, []Key{keyA, keyB}, nil)
	router.Endpoint = "http://unused.example/v1/messages"

	if _, err := router.Generate(context.Background(), routerTestRequest()); err != nil {
		t.Fatal(err)
	}
	if len(pathsA) != 1 || pathsA[0] != "/v1/messages" {
		t.Fatalf("首选 Key 应请求自己的 base_url: %v", pathsA)
	}
	if len(pathsB) != 1 || pathsB[0] != "/v1/messages" {
		t.Fatalf("故障转移应请求下一个 Key 自己的 base_url: %v", pathsB)
	}
}
