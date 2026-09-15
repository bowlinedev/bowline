package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectsWritesAddsAndRemoves(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	os.WriteFile(a, []byte("package x\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "node_modules", "m"), 0o755)
	os.WriteFile(filepath.Join(dir, "node_modules", "m", "ignored.go"), []byte("package m\n"), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan []Change, 10)
	go Run(ctx, dir, 20*time.Millisecond, func(c []Change) { events <- c })
	time.Sleep(60 * time.Millisecond)
	os.WriteFile(a, []byte("package x\n\nvar y = 1\n"), 0o644)
	got := receive(t, events)
	if len(got) != 1 || got[0].Path != a || got[0].Removed {
		t.Fatalf("write: %+v", got)
	}
	b := filepath.Join(dir, "b.go")
	os.WriteFile(b, []byte("package x\n"), 0o644)
	got = receive(t, events)
	if len(got) != 1 || got[0].Path != b {
		t.Fatalf("add: %+v", got)
	}
	os.Remove(a)
	got = receive(t, events)
	if len(got) != 1 || got[0].Path != a || !got[0].Removed {
		t.Fatalf("remove: %+v", got)
	}
	os.WriteFile(filepath.Join(dir, "c_test.go"), []byte("package x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "node_modules", "m", "ignored.go"), []byte("package m\n\nvar z = 1\n"), 0o644)
	select {
	case c := <-events:
		t.Fatalf("unexpected event %+v", c)
	case <-time.After(100 * time.Millisecond):
	}
}

func receive(t *testing.T, events chan []Change) []Change {
	t.Helper()
	select {
	case c := <-events:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for change")
		return nil
	}
}
