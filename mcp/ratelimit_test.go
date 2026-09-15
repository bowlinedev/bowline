package mcp

import (
	"testing"
	"time"
)

func TestLimiterBurstAndRefill(t *testing.T) {
	now := time.Unix(0, 0)
	l := newLimiter(60, 3)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if !l.allow("a") {
			t.Fatalf("call %d within burst denied", i)
		}
	}
	if l.allow("a") {
		t.Fatal("fourth call allowed")
	}
	if !l.allow("b") {
		t.Fatal("other key shares bucket")
	}
	now = now.Add(500 * time.Millisecond)
	if l.allow("a") {
		t.Fatal("half a token allowed a call")
	}
	now = now.Add(500 * time.Millisecond)
	if !l.allow("a") {
		t.Fatal("refilled token denied")
	}
	now = now.Add(time.Hour)
	for i := 0; i < 3; i++ {
		if !l.allow("a") {
			t.Fatalf("call %d after long idle denied", i)
		}
	}
	if l.allow("a") {
		t.Fatal("bucket exceeded burst after long idle")
	}
}

func TestLimiterMinimumBurst(t *testing.T) {
	l := newLimiter(1, 0)
	if !l.allow("") {
		t.Fatal("first call denied")
	}
	if l.allow("") {
		t.Fatal("second call allowed with burst of one")
	}
}

func TestClientKey(t *testing.T) {
	if clientKey("") != "" {
		t.Error("empty authorization should map to the global bucket")
	}
	a, b := clientKey("Bearer a"), clientKey("Bearer b")
	if a == b || len(a) != 64 || a == "Bearer a" {
		t.Errorf("keys = %q %q", a, b)
	}
}
