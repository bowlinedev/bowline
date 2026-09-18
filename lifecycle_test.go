package bowline

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCallTimeoutEndsASlowProcedure(t *testing.T) {
	slow := func(ctx context.Context, in getInput) (user, error) {
		<-ctx.Done()
		return user{}, ctx.Err()
	}
	h := NewRouter(Query("slow", slow)).Handler(CallTimeout(20*time.Millisecond), Logger(discardLogger()))
	start := time.Now()
	rec := do(h, http.MethodPost, "/api/slow", `{"id":1}`, nil)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the call ran for %s; the deadline did not apply", elapsed)
	}
	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status %d, want 408: %s", rec.Code, rec.Body.String())
	}
	if got := envelopeOf(t, rec); got.Code != DeadlineExceeded {
		t.Fatalf("code %q, want %q", got.Code, DeadlineExceeded)
	}
}

func TestCallTimeoutLeavesFastProceduresAlone(t *testing.T) {
	h := NewRouter(Query("get", getUser)).Handler(CallTimeout(time.Minute), Logger(discardLogger()))
	rec := do(h, http.MethodPost, "/api/get", `{"id":1}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestCallTimeoutDoesNotApplyToSubscriptions(t *testing.T) {
	stream := func(ctx context.Context, in struct{}, out *Stream[tick]) error {
		time.Sleep(60 * time.Millisecond)
		return out.Send(tick{N: 1})
	}
	h := NewRouter(Subscription("watch", stream)).Handler(CallTimeout(10*time.Millisecond), Logger(discardLogger()))
	rec := streamRequest(h, http.MethodGet, "/watch", "text/event-stream", nil)
	if !strings.Contains(rec.Body.String(), `"n":1`) {
		t.Fatalf("a subscription must outlive the per-call deadline, got %q", rec.Body.String())
	}
}

func TestDrainEndsActiveSubscriptions(t *testing.T) {
	drain, shutdown := context.WithCancel(context.Background())
	defer shutdown()
	started := make(chan struct{})
	stream := func(ctx context.Context, in struct{}, out *Stream[tick]) error {
		close(started)
		<-ctx.Done()
		return nil
	}
	h := NewRouter(Subscription("watch", stream)).Handler(Drain(drain), Logger(discardLogger()))

	done := make(chan struct{})
	go func() {
		streamRequest(h, http.MethodGet, "/watch", "text/event-stream", nil)
		close(done)
	}()
	<-started
	shutdown()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the subscription outlived the drain signal; a server shutdown would hang on it")
	}
}

func TestSubscriptionsRunWithoutADrainSignal(t *testing.T) {
	stream := func(ctx context.Context, in struct{}, out *Stream[tick]) error {
		return out.Send(tick{N: 7})
	}
	h := NewRouter(Subscription("watch", stream)).Handler(Logger(discardLogger()))
	rec := streamRequest(h, http.MethodGet, "/watch", "text/event-stream", nil)
	if !strings.Contains(rec.Body.String(), `"n":7`) {
		t.Fatalf("body %q", rec.Body.String())
	}
}
