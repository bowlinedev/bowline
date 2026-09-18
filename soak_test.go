package bowline

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/internal/idempotency"
)

func soakDuration(t *testing.T) time.Duration {
	t.Helper()
	raw := os.Getenv("BOWLINE_SOAK")
	if raw == "" {
		t.Skip("set BOWLINE_SOAK to a duration in seconds to run the soak")
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		t.Fatalf("BOWLINE_SOAK must be a positive number of seconds, got %q", raw)
	}
	return time.Duration(seconds) * time.Second
}

func heapInUse() uint64 {
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapInuse
}

func TestSoakUnaryCallsHoldSteady(t *testing.T) {
	duration := soakDuration(t)
	inner := idempotency.NewMemory(nil)
	h := NewRouter(
		Query("get", getUser),
		Mutation("create", createUser, Idempotent()),
	).Handler(Idempotency(memoryStore{inner: inner}, time.Hour), Production(true), Logger(discardLogger()))

	warm(h)
	baseHeap, baseGoroutines := heapInUse(), runtime.NumGoroutine()

	var wg sync.WaitGroup
	stop := time.Now().Add(duration)
	var calls int64
	var bad int64
	var mu sync.Mutex
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := 0
			for time.Now().Before(stop) {
				n++
				key := fmt.Sprintf("w%d-%d", worker, n)
				created := do(h, http.MethodPost, "/api/create", `{"id":1,"name":"ada"}`, map[string]string{"Idempotency-Key": key})
				fetched := do(h, http.MethodPost, "/api/get", `{"id":1}`, nil)
				if created.Code != http.StatusOK || fetched.Code != http.StatusOK {
					mu.Lock()
					bad++
					mu.Unlock()
					return
				}
			}
			mu.Lock()
			calls += int64(n) * 2
			mu.Unlock()
		}()
	}
	wg.Wait()

	if bad > 0 {
		t.Fatalf("%d workers saw a non-200 response; the soak was not exercising the success path", bad)
	}
	heap, goroutines, entries := heapInUse(), runtime.NumGoroutine(), inner.Len()
	t.Logf("%d calls in %s: heap %d -> %d bytes, goroutines %d -> %d, idempotency entries %d", calls, duration, baseHeap, heap, baseGoroutines, goroutines, entries)
	if entries > idempotency.DefaultMaxKeys {
		t.Fatalf("the idempotency store holds %d entries after %d unique keys; it grows without bound", entries, calls/2)
	}
	if goroutines > baseGoroutines+4 {
		t.Fatalf("goroutines grew from %d to %d", baseGoroutines, goroutines)
	}
	if heap > baseHeap*4 {
		t.Fatalf("heap grew from %d to %d bytes, which is more than a steady state should need", baseHeap, heap)
	}
}

func TestSoakSubscriptionsHoldSteady(t *testing.T) {
	duration := soakDuration(t)
	stream := func(ctx context.Context, in struct{}, out *Stream[tick]) error {
		for {
			if err := out.Send(tick{N: 1}); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Millisecond):
			}
		}
	}
	h := NewRouter(Subscription("watch", stream)).Handler(Heartbeat(5*time.Millisecond), Logger(discardLogger()))

	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "/api/watch?input=%7B%7D", nil).WithContext(ctx)
		req.Header.Set("Accept", "text/event-stream")
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	run()
	baseHeap, baseGoroutines := heapInUse(), runtime.NumGoroutine()

	streams := 0
	stop := time.Now().Add(duration)
	for time.Now().Before(stop) {
		run()
		streams++
	}

	heap, goroutines := heapInUse(), runtime.NumGoroutine()
	t.Logf("%d streams in %s: heap %d -> %d bytes, goroutines %d -> %d", streams, duration, baseHeap, heap, baseGoroutines, goroutines)
	if goroutines > baseGoroutines+4 {
		t.Fatalf("goroutines grew from %d to %d over %d streams", baseGoroutines, goroutines, streams)
	}
	if heap > baseHeap*4 {
		t.Fatalf("heap grew from %d to %d bytes over %d streams", baseHeap, heap, streams)
	}
}

func warm(h http.Handler) {
	for range 200 {
		do(h, http.MethodPost, "/api/get", `{"id":1}`, nil)
	}
}
