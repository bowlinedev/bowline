package bowline

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type faultyStore struct {
	inner      IdempotencyStore
	failBegin  bool
	failDone   bool
	failAbort  bool
	abortCalls atomic.Int64
}

var errStoreDown = errors.New("store is down")

func (f *faultyStore) Begin(ctx context.Context, key string) (IdempotencyState, int, []byte, error) {
	if f.failBegin {
		return IdempotencyNew, 0, nil, errStoreDown
	}
	return f.inner.Begin(ctx, key)
}

func (f *faultyStore) Complete(ctx context.Context, key string, status int, body []byte, ttl time.Duration) error {
	if f.failDone {
		return errStoreDown
	}
	return f.inner.Complete(ctx, key, status, body, ttl)
}

func (f *faultyStore) Abort(ctx context.Context, key string) error {
	f.abortCalls.Add(1)
	if f.failAbort {
		return errStoreDown
	}
	return f.inner.Abort(ctx, key)
}

func countingRouter(runs *atomic.Int64) *Router {
	create := func(ctx context.Context, in user) (user, error) {
		runs.Add(1)
		return in, nil
	}
	return NewRouter(Mutation("create", create, Idempotent()))
}

func TestIdempotencyStoreFailureNeverRunsTheMutation(t *testing.T) {
	var runs atomic.Int64
	store := &faultyStore{inner: MemoryIdempotencyStore(), failBegin: true}
	h := countingRouter(&runs).Handler(Idempotency(store, time.Hour), Logger(discardLogger()))
	rec := do(h, http.MethodPost, "/api/create", `{"id":1}`, map[string]string{"Idempotency-Key": "k1"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if runs.Load() != 0 {
		t.Fatalf("the mutation ran %d times while the store was down; it must fail closed", runs.Load())
	}
}

func TestIdempotencyCompleteFailureReleasesTheKey(t *testing.T) {
	var runs atomic.Int64
	store := &faultyStore{inner: MemoryIdempotencyStore(), failDone: true}
	h := countingRouter(&runs).Handler(Idempotency(store, time.Hour), Logger(discardLogger()))
	headers := map[string]string{"Idempotency-Key": "k1"}

	first := do(h, http.MethodPost, "/api/create", `{"id":1}`, headers)
	if first.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", first.Code, first.Body.String())
	}
	if store.abortCalls.Load() == 0 {
		t.Fatal("a failed Complete must release the key, or every later retry answers ABORTED forever")
	}
	second := do(h, http.MethodPost, "/api/create", `{"id":1}`, headers)
	if second.Code != http.StatusOK {
		t.Fatalf("the retry got %d; the key was never released: %s", second.Code, second.Body.String())
	}
	if runs.Load() != 2 {
		t.Fatalf("the mutation ran %d times; an unstored response must be re-executed rather than replayed", runs.Load())
	}
}

func TestIdempotencyAbortFailureDoesNotBreakTheResponse(t *testing.T) {
	var runs atomic.Int64
	store := &faultyStore{inner: MemoryIdempotencyStore(), failDone: true, failAbort: true}
	h := countingRouter(&runs).Handler(Idempotency(store, time.Hour), Logger(discardLogger()))
	rec := do(h, http.MethodPost, "/api/create", `{"id":1}`, map[string]string{"Idempotency-Key": "k1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

type blockingWriter struct {
	header  http.Header
	release chan struct{}
	writes  atomic.Int64
}

func (b *blockingWriter) Header() http.Header {
	if b.header == nil {
		b.header = http.Header{}
	}
	return b.header
}

func (b *blockingWriter) Write(p []byte) (int, error) {
	if b.writes.Add(1) > 1 {
		<-b.release
	}
	return len(p), nil
}

func (b *blockingWriter) WriteHeader(int) {}

func (b *blockingWriter) Flush() {}

func TestSlowSubscriberDoesNotPinTheHandlerForever(t *testing.T) {
	stream := func(ctx context.Context, in struct{}, out *Stream[tick]) error {
		for {
			if err := out.Send(tick{N: 1}); err != nil {
				return err
			}
		}
	}
	h := NewRouter(Subscription("watch", stream)).Handler(Logger(discardLogger()))
	writer := &blockingWriter{release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/watch?input=%7B%7D", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(writer, req)
		close(done)
	}()
	for writer.writes.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	close(writer.release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the handler never returned for a consumer that stopped reading")
	}
}
