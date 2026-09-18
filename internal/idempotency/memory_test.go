package idempotency

import (
	"context"
	"fmt"
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

func TestMemoryStaysBoundedUnderUniqueKeys(t *testing.T) {
	now := time.Now()
	store := NewMemoryWithLimit(func() time.Time { return now }, 64)
	for i := range 5000 {
		key := fmt.Sprintf("tenant/%d", i)
		if _, _, _, err := store.Begin(context.Background(), key); err != nil {
			t.Fatal(err)
		}
		if err := store.Complete(context.Background(), key, 200, []byte(`{}`), 24*time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if got := store.Len(); got > 64 {
		t.Fatalf("the store holds %d entries with a limit of 64; a caller sending unique keys grows it without bound", got)
	}
}

func TestMemoryNeverEvictsAnInFlightKey(t *testing.T) {
	now := time.Now()
	store := NewMemoryWithLimit(func() time.Time { return now }, 8)
	if _, _, _, err := store.Begin(context.Background(), "held"); err != nil {
		t.Fatal(err)
	}
	for i := range 200 {
		key := fmt.Sprintf("other/%d", i)
		store.Begin(context.Background(), key)
		store.Complete(context.Background(), key, 200, []byte(`{}`), time.Hour)
	}
	state, _, _, err := store.Begin(context.Background(), "held")
	if err != nil {
		t.Fatal(err)
	}
	if state != InFlight {
		t.Fatalf("the in-flight key was evicted: state %v; a concurrent retry would run the mutation twice", state)
	}
}
