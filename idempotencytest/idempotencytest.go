package idempotencytest

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bowlinedev/bowline"
)

type Factory func(t *testing.T) bowline.IdempotencyStore

func Verify(t *testing.T, newStore Factory) {
	t.Helper()
	checks := []struct {
		name string
		run  func(t *testing.T, store bowline.IdempotencyStore)
	}{
		{"a fresh key is new", freshKeyIsNew},
		{"a claimed key is in flight", claimedKeyIsInFlight},
		{"a completed key replays", completedKeyReplays},
		{"an aborted key is new again", abortedKeyIsNewAgain},
		{"keys do not interfere", keysDoNotInterfere},
		{"bodies round trip byte for byte", bodiesRoundTrip},
		{"one caller wins a concurrent claim", oneCallerWinsAConcurrentClaim},
		{"aborting an unknown key is not an error", abortingAnUnknownKeyIsNotAnError},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			check.run(t, newStore(t))
		})
	}
}

func key(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s/%d", t.Name(), time.Now().UnixNano())
}

func freshKeyIsNew(t *testing.T, store bowline.IdempotencyStore) {
	state, _, _, err := store.Begin(context.Background(), key(t))
	if err != nil {
		t.Fatal(err)
	}
	if state != bowline.IdempotencyNew {
		t.Fatalf("state %v, want New", state)
	}
}

func claimedKeyIsInFlight(t *testing.T, store bowline.IdempotencyStore) {
	ctx, k := context.Background(), key(t)
	if _, _, _, err := store.Begin(ctx, k); err != nil {
		t.Fatal(err)
	}
	state, _, _, err := store.Begin(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if state != bowline.IdempotencyInFlight {
		t.Fatalf("state %v, want InFlight; a second caller must not run the mutation again", state)
	}
}

func completedKeyReplays(t *testing.T, store bowline.IdempotencyStore) {
	ctx, k := context.Background(), key(t)
	if _, _, _, err := store.Begin(ctx, k); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"id":7,"total":"USD 1.00"}`)
	if err := store.Complete(ctx, k, 201, want, time.Hour); err != nil {
		t.Fatal(err)
	}
	state, status, body, err := store.Begin(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if state != bowline.IdempotencyStored {
		t.Fatalf("state %v, want Stored", state)
	}
	if status != 201 {
		t.Fatalf("status %d, want 201", status)
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("body %q, want %q", body, want)
	}
}

func abortedKeyIsNewAgain(t *testing.T, store bowline.IdempotencyStore) {
	ctx, k := context.Background(), key(t)
	if _, _, _, err := store.Begin(ctx, k); err != nil {
		t.Fatal(err)
	}
	if err := store.Abort(ctx, k); err != nil {
		t.Fatal(err)
	}
	state, _, _, err := store.Begin(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if state != bowline.IdempotencyNew {
		t.Fatalf("state %v, want New; a released key must be claimable again", state)
	}
}

func keysDoNotInterfere(t *testing.T, store bowline.IdempotencyStore) {
	ctx := context.Background()
	first, second := key(t)+"/a", key(t)+"/b"
	if _, _, _, err := store.Begin(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, first, 200, []byte(`"first"`), time.Hour); err != nil {
		t.Fatal(err)
	}
	state, _, _, err := store.Begin(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if state != bowline.IdempotencyNew {
		t.Fatalf("state %v for a different key, want New", state)
	}
}

func bodiesRoundTrip(t *testing.T, store bowline.IdempotencyStore) {
	ctx := context.Background()
	bodies := map[string][]byte{
		"empty":   {},
		"binary":  {0x00, 0x01, 0xff, 0xfe, '\n', '\''},
		"unicode": []byte(`{"name":"Ada ♥ Lovelace"}`),
	}
	for name, want := range bodies {
		t.Run(name, func(t *testing.T) {
			k := key(t) + "/" + name
			if _, _, _, err := store.Begin(ctx, k); err != nil {
				t.Fatal(err)
			}
			if err := store.Complete(ctx, k, 200, want, time.Hour); err != nil {
				t.Fatal(err)
			}
			_, _, body, err := store.Begin(ctx, k)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(body, want) {
				t.Fatalf("body %q, want %q", body, want)
			}
		})
	}
}

func oneCallerWinsAConcurrentClaim(t *testing.T, store bowline.IdempotencyStore) {
	ctx, k := context.Background(), key(t)
	var news, inFlight, failures atomic.Int64
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state, _, _, err := store.Begin(ctx, k)
			switch {
			case err != nil:
				failures.Add(1)
			case state == bowline.IdempotencyNew:
				news.Add(1)
			case state == bowline.IdempotencyInFlight:
				inFlight.Add(1)
			}
		}()
	}
	wg.Wait()
	if failures.Load() > 0 {
		t.Fatalf("%d callers failed", failures.Load())
	}
	if news.Load() != 1 {
		t.Fatalf("%d callers were told the key was new; exactly one may run the mutation", news.Load())
	}
	if inFlight.Load() != 15 {
		t.Fatalf("%d callers saw in flight, want 15", inFlight.Load())
	}
}

func abortingAnUnknownKeyIsNotAnError(t *testing.T, store bowline.IdempotencyStore) {
	if err := store.Abort(context.Background(), key(t)); err != nil {
		t.Fatalf("aborting a key that was never claimed returned %v", err)
	}
}
