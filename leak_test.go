package bowline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func goroutinesAtRest(t *testing.T) int {
	t.Helper()
	settled := runtime.NumGoroutine()
	for range 100 {
		time.Sleep(10 * time.Millisecond)
		now := runtime.NumGoroutine()
		if now >= settled {
			return settled
		}
		settled = now
	}
	return settled
}

func TestSubscriptionsReleaseTheirGoroutines(t *testing.T) {
	stream := func(ctx context.Context, in struct{}, out *Stream[tick]) error {
		<-ctx.Done()
		return nil
	}
	h := NewRouter(Subscription("watch", stream)).Handler(Heartbeat(time.Millisecond), Logger(discardLogger()))

	run := func() {
		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/watch?input=%7B%7D", nil).WithContext(ctx)
		req.Header.Set("Accept", "text/event-stream")
		done := make(chan struct{})
		go func() {
			h.ServeHTTP(httptest.NewRecorder(), req)
			close(done)
		}()
		time.Sleep(5 * time.Millisecond)
		cancel()
		<-done
	}

	run()
	before := goroutinesAtRest(t)
	for range 50 {
		run()
	}
	after := goroutinesAtRest(t)
	if after > before+2 {
		t.Fatalf("goroutines grew from %d to %d over 50 streams; each subscription leaks", before, after)
	}
}
