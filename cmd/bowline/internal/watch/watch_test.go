package watch

import (
	"fmt"
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
	ctx := t.Context()
	events := make(chan []Change, 10)
	go Run(ctx, dir, 20*time.Millisecond, func(c []Change) { events <- c })
	waitForWatcher(t, dir, events)
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

func waitForWatcher(t *testing.T, dir string, events chan []Change) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for i := 0; time.Now().Before(deadline); i++ {
		probe := filepath.Join(dir, fmt.Sprintf("probe%d.go", i))
		if err := os.WriteFile(probe, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		select {
		case <-events:
			os.Remove(probe)
			drain(events)
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
	t.Fatal("the watcher never reported a change, so the baseline scan never ran")
}

func drain(events chan []Change) {
	for {
		select {
		case <-events:
		case <-time.After(300 * time.Millisecond):
			return
		}
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
