package idempotency

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	m := NewMemory(func() time.Time { return now })
	ctx := context.Background()
	state, _, _, _ := m.Begin(ctx, "k")
	if state != New {
		t.Fatalf("first begin %v", state)
	}
	if state, _, _, _ := m.Begin(ctx, "k"); state != InFlight {
		t.Fatalf("second begin %v", state)
	}
	m.Complete(ctx, "k", 200, []byte("body"), time.Hour)
	state, status, body, _ := m.Begin(ctx, "k")
	if state != Stored || status != 200 || string(body) != "body" {
		t.Fatalf("stored %v %d %s", state, status, body)
	}
	now = now.Add(2 * time.Hour)
	if state, _, _, _ := m.Begin(ctx, "k"); state != New {
		t.Fatalf("after expiry %v", state)
	}
	m.Abort(ctx, "k")
	if state, _, _, _ := m.Begin(ctx, "k"); state != New {
		t.Fatalf("after abort %v", state)
	}
}
