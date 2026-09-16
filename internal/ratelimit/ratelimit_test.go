package ratelimit

import (
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

func TestRateLimitBucketRefill(t *testing.T) {
	clock := &testClock{at: time.Unix(1700000000, 0)}
	l := New(2, 3, 8, clock.now)
	for i := range 3 {
		if _, ok := l.Allow("a"); !ok {
			t.Fatalf("burst token %d was refused", i)
		}
	}
	if wait, ok := l.Allow("a"); ok {
		t.Fatal("a fourth call passed with an empty bucket")
	} else if wait <= 0 || wait > 500*time.Millisecond {
		t.Fatalf("wait %v, want up to 500ms at 2 per second", wait)
	}
	clock.advance(500 * time.Millisecond)
	if _, ok := l.Allow("a"); !ok {
		t.Fatal("half a second did not refill one token at 2 per second")
	}
	if _, ok := l.Allow("a"); ok {
		t.Fatal("the refill produced more than one token")
	}
	clock.advance(time.Hour)
	for i := range 3 {
		if _, ok := l.Allow("a"); !ok {
			t.Fatalf("token %d was refused after a long idle period", i)
		}
	}
	if _, ok := l.Allow("a"); ok {
		t.Fatal("the bucket refilled past its burst")
	}
}

func TestRateLimitEvictsIdleKeys(t *testing.T) {
	clock := &testClock{at: time.Unix(1700000000, 0)}
	l := New(1, 1, 2, clock.now)
	l.Allow("a")
	l.Allow("b")
	l.Allow("a")
	l.Allow("c")
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

func BenchmarkRateLimitAllow(b *testing.B) {
	l := New(1e9, 1e9, 1024, time.Now)
	l.Allow("tenant")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		l.Allow("tenant")
	}
}
