package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// FakeProvider is the default test provider. It records requests and returns
// queued responses/errors without network access.
type FakeProvider struct {
	mu         sync.Mutex
	requests   []Request
	queue      []fakeItem
	OnGenerate func(context.Context, Request) (Response, error)
}

type fakeItem struct {
	resp Response
	err  error
}

func NewFakeProvider() *FakeProvider { return &FakeProvider{} }

func (f *FakeProvider) Enqueue(resp Response, err ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var e error
	if len(err) > 0 {
		e = err[0]
	}
	f.queue = append(f.queue, fakeItem{resp: resp, err: e})
}

func (f *FakeProvider) EnqueueError(err error) {
	f.Enqueue(Response{}, err)
}

func (f *FakeProvider) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Request, len(f.requests))
	copy(out, f.requests)
	return out
}

func (f *FakeProvider) Generate(ctx context.Context, req Request) (Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	var item *fakeItem
	if len(f.queue) > 0 {
		next := f.queue[0]
		f.queue = f.queue[1:]
		item = &next
	}
	fn := f.OnGenerate
	f.mu.Unlock()
	if item != nil {
		return item.resp, item.err
	}
	if fn != nil {
		return fn(ctx, req)
	}
	return Response{}, fmt.Errorf("ai.FakeProvider: no response queued for %s", req.Purpose)
}

// DeterministicFakeProvider is used only by explicit local deployment drills.
// It supports connectivity so API -> queue -> Worker -> SSE can be verified
// without a key or network. It deliberately refuses all material-generating
// purposes rather than fabricating study content.
type DeterministicFakeProvider struct{}

func NewDeterministicFakeProvider() DeterministicFakeProvider { return DeterministicFakeProvider{} }

func (DeterministicFakeProvider) Generate(_ context.Context, req Request) (Response, error) {
	if req.Purpose != PurposeConnectivity {
		return Response{}, ErrorForCode(ErrInvalidRequest, "本机 Fake Provider 仅支持 Claude 连通性演练。")
	}
	return Response{
		JSON:              json.RawMessage(`{"ok":true}`),
		Model:             "fake-local",
		ProviderRequestID: "fake-connectivity",
		Latency:           time.Millisecond,
		StopReason:        "end_turn",
	}, nil
}
