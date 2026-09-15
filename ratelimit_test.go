package bowline

import (
	"container/list"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

type testClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

func keyFrom(key string) func(context.Context) string {
	return func(context.Context) string { return key }
}

func limitedRouter(opts RateLimitOptions) http.Handler {
	return NewRouter(Query("get", getUser, Use(RateLimit(opts)))).Handler(Logger(discardLogger()))
}

func call(h http.Handler) *httptest.ResponseRecorder {
	return do(h, http.MethodGet, "/api/get?input=%7B%22id%22%3A1%7D", "", nil)
}

func TestRateLimitBucketRefill(t *testing.T) {
	clock := &testClock{at: time.Unix(1700000000, 0)}
	l := &limiter{
		key: keyFrom("a"), rate: 2, burst: 3, maxKeys: 8,
		now: clock.now, buckets: map[string]*bucket{}, order: list.New(),
	}
	for i := range 3 {
		if _, ok := l.allow("a"); !ok {
			t.Fatalf("burst token %d was refused", i)
		}
	}
	if wait, ok := l.allow("a"); ok {
		t.Fatal("a fourth call passed with an empty bucket")
	} else if wait <= 0 || wait > 500*time.Millisecond {
		t.Fatalf("wait %v, want up to 500ms at 2 per second", wait)
	}
	clock.advance(500 * time.Millisecond)
	if _, ok := l.allow("a"); !ok {
		t.Fatal("half a second did not refill one token at 2 per second")
	}
	if _, ok := l.allow("a"); ok {
		t.Fatal("the refill produced more than one token")
	}
	clock.advance(time.Hour)
	for i := range 3 {
		if _, ok := l.allow("a"); !ok {
			t.Fatalf("token %d was refused after a long idle period", i)
		}
	}
	if _, ok := l.allow("a"); ok {
		t.Fatal("the bucket refilled past its burst")
	}
}

func TestRateLimitReturnsResourceExhausted(t *testing.T) {
	clock := &testClock{at: time.Unix(1700000000, 0)}
	h := limitedRouter(RateLimitOptions{Key: keyFrom("a"), Rate: 1, Burst: 1, Now: clock.now})
	if rec := call(h); rec.Code != 200 {
		t.Fatalf("the first call was refused: %d %s", rec.Code, rec.Body.String())
	}
	rec := call(h)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429, body %s", rec.Code, rec.Body.String())
	}
	if got := envelopeOf(t, rec).Code; got != ResourceExhausted {
		t.Fatalf("code %q, want %q", got, ResourceExhausted)
	}
	retry := rec.Header().Get("Retry-After")
	if seconds, err := strconv.Atoi(retry); err != nil || seconds < 1 {
		t.Fatalf("Retry-After %q, want a positive number of seconds", retry)
	}
	clock.advance(2 * time.Second)
	if rec := call(h); rec.Code != 200 {
		t.Fatalf("the limiter never refilled: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRateLimitEvictsIdleKeys(t *testing.T) {
	clock := &testClock{at: time.Unix(1700000000, 0)}
	l := &limiter{
		key: keyFrom(""), rate: 1, burst: 1, maxKeys: 2,
		now: clock.now, buckets: map[string]*bucket{}, order: list.New(),
	}
	l.allow("a")
	l.allow("b")
	l.allow("a")
	l.allow("c")
	if len(l.buckets) != 2 {
		t.Fatalf("%d buckets, want 2", len(l.buckets))
	}
	if _, ok := l.buckets["b"]; ok {
		t.Fatal("the least recently used key survived eviction")
	}
	for _, key := range []string{"a", "c"} {
		if _, ok := l.buckets[key]; !ok {
			t.Fatalf("%q was evicted instead of the idle key", key)
		}
	}
}

func TestRateLimitEmptyKeyBypasses(t *testing.T) {
	h := limitedRouter(RateLimitOptions{Key: keyFrom(""), Rate: 1, Burst: 1})
	for i := range 20 {
		if rec := call(h); rec.Code != 200 {
			t.Fatalf("call %d with an empty key was limited: %d", i, rec.Code)
		}
	}
}

func TestRateLimitConcurrentKeys(t *testing.T) {
	h := limitedRouter(RateLimitOptions{
		Key:   func(ctx context.Context) string { return CallFrom(ctx).Request.Header.Get("X-Tenant") },
		Rate:  1000,
		Burst: 4,
	})
	var wg sync.WaitGroup
	refused := make([]int, 8)
	for tenant := range 8 {
		wg.Add(1)
		go func(tenant int) {
			defer wg.Done()
			for range 4 {
				rec := do(h, http.MethodGet, "/api/get?input=%7B%22id%22%3A1%7D", "", map[string]string{"X-Tenant": strconv.Itoa(tenant)})
				if rec.Code != 200 {
					refused[tenant]++
				}
			}
		}(tenant)
	}
	wg.Wait()
	for tenant, count := range refused {
		if count != 0 {
			t.Fatalf("tenant %d was refused %d of its 4 burst calls", tenant, count)
		}
	}
}

func TestRateLimitPanicsOnBadOptions(t *testing.T) {
	for name, opts := range map[string]RateLimitOptions{
		"no key":        {Rate: 1},
		"zero rate":     {Key: keyFrom("a")},
		"negative rate": {Key: keyFrom("a"), Rate: -1},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic")
				}
			}()
			RateLimit(opts)
		})
	}
}

func TestResponseHeaderReachesTheClientOnSuccess(t *testing.T) {
	stamp := func(next Next) Next {
		return func(ctx context.Context, in any) (any, error) {
			CallFrom(ctx).ResponseHeader().Set("X-Stamp", "here")
			return next(ctx, in)
		}
	}
	h := NewRouter(Query("get", getUser, Use(stamp))).Handler(Logger(discardLogger()))
	rec := call(h)
	if got := rec.Header().Get("X-Stamp"); got != "here" {
		t.Fatalf("X-Stamp %q, want here", got)
	}
}

func BenchmarkRateLimitAllow(b *testing.B) {
	l := &limiter{
		key: keyFrom("a"), rate: 1e9, burst: 1e9, maxKeys: 1024,
		now: time.Now, buckets: map[string]*bucket{}, order: list.New(),
	}
	l.allow("tenant")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		l.allow("tenant")
	}
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	next := RateLimit(RateLimitOptions{Key: keyFrom("tenant"), Rate: 1e9, Burst: 1e9})(
		func(ctx context.Context, in any) (any, error) { return nil, nil },
	)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		next(ctx, nil)
	}
}
