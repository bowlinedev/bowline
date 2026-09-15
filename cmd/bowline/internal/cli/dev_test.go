package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/watch"
)

func TestDevRegeneratesOnChange(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes","targets":{"ts":{"out":"out/bowline.ts"}}}`), 0o644)
	changes := make(chan []watch.Change)
	stop := make(chan struct{})
	var out, errOut syncBuffer
	opts := DevOptions{
		Options:  Options{Dir: dir, Env: append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"), Stdout: &out, Stderr: &errOut},
		Changes:  changes,
		Stop:     stop,
		Interval: time.Hour,
	}
	done := make(chan int)
	go func() { done <- Dev(opts) }()
	waitFor(t, func() bool { return strings.Contains(out.String(), "wrote out/bowline.ts") })
	file := filepath.Join(dir, "rows", "routing", "api.go")
	src, _ := os.ReadFile(file)
	os.WriteFile(file, bytes.Replace(src, []byte(`json:"name"`), []byte(`json:"label"`), 1), 0o644)
	changes <- []watch.Change{{Path: file}}
	waitFor(t, func() bool { return strings.Count(out.String(), "wrote out/bowline.ts") == 2 })
	generated, _ := os.ReadFile(filepath.Join(dir, "out", "bowline.ts"))
	if !strings.Contains(string(generated), "label: string") {
		t.Fatalf("regenerated file stale:\n%s", generated)
	}
	os.WriteFile(file, bytes.Replace(src, []byte("Item{}, nil }"), []byte("Item{} }"), 1), 0o644)
	changes <- []watch.Change{{Path: file}}
	waitFor(t, func() bool { return strings.Contains(errOut.String(), "rows/routing/api.go:") })
	close(stop)
	if code := <-done; code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "regenerated in") {
		t.Fatalf("missing timing line in %q", out.String())
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
