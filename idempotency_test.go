package bowline

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type createInput struct {
	Name string `json:"name" validate:"required"`
}

type created struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type creator struct {
	calls atomic.Int64
	block chan struct{}
}

func (c *creator) create(ctx context.Context, in createInput) (created, error) {
	n := c.calls.Add(1)
	if c.block != nil {
		<-c.block
	}
	if in.Name == "boom" {
		return created{}, Errorf(Internal, "boom")
	}
	return created{ID: n, Name: in.Name}, nil
}

func idempotentHandler(c *creator, opts ...HandlerOption) http.Handler {
	base := []HandlerOption{Idempotency(MemoryIdempotencyStore(), time.Hour)}
	return NewRouter(Mount("invoices", NewRouter(Mutation("create", c.create, Idempotent())))).Handler(append(base, opts...)...)
}

func TestReplayReturnsStoredResponse(t *testing.T) {
	c := &creator{}
	h := idempotentHandler(c)
	first := do(h, http.MethodPost, "/invoices.create", `{"name":"a"}`, map[string]string{"Idempotency-Key": "k1"})
	second := do(h, http.MethodPost, "/invoices.create", `{"name":"a"}`, map[string]string{"Idempotency-Key": "k1"})
	if first.Code != 200 || second.Code != 200 || first.Body.String() != second.Body.String() {
		t.Fatalf("first %d %s second %d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
	}
	if second.Header().Get("Idempotent-Replayed") != "true" || first.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("replay header wrong: %v %v", first.Header(), second.Header())
	}
	if c.calls.Load() != 1 {
		t.Fatalf("procedure ran %d times", c.calls.Load())
	}
	third := do(h, http.MethodPost, "/invoices.create", `{"name":"b"}`, map[string]string{"Idempotency-Key": "k2"})
	if !strings.Contains(third.Body.String(), `"id":2`) {
		t.Fatalf("new key must run: %s", third.Body.String())
	}
}

func TestInFlightConflict(t *testing.T) {
	c := &creator{block: make(chan struct{})}
	h := idempotentHandler(c)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		do(h, http.MethodPost, "/invoices.create", `{"name":"a"}`, map[string]string{"Idempotency-Key": "k"})
	}()
	for c.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	rec := do(h, http.MethodPost, "/invoices.create", `{"name":"a"}`, map[string]string{"Idempotency-Key": "k"})
	if rec.Code != 409 || errorCode(t, rec) != Aborted {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	close(c.block)
	wg.Wait()
}

func TestScopeSeparatesKeys(t *testing.T) {
	c := &creator{}
	inner := idempotentHandler(c)
	scoped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r.WithContext(WithIdempotencyScope(r.Context(), r.Header.Get("X-Tenant"))))
	})
	do(scoped, http.MethodPost, "/invoices.create", `{"name":"a"}`, map[string]string{"Idempotency-Key": "k", "X-Tenant": "t1"})
	do(scoped, http.MethodPost, "/invoices.create", `{"name":"a"}`, map[string]string{"Idempotency-Key": "k", "X-Tenant": "t2"})
	if c.calls.Load() != 2 {
		t.Fatalf("procedure ran %d times, want 2", c.calls.Load())
	}
}

func TestRequireIdempotencyKey(t *testing.T) {
	c := &creator{}
	h := idempotentHandler(c, RequireIdempotencyKey())
	rec := do(h, http.MethodPost, "/invoices.create", `{"name":"a"}`, nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "Idempotency-Key") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	rec = do(idempotentHandler(c), http.MethodPost, "/invoices.create", `{"name":"a"}`, nil)
	if rec.Code != 200 {
		t.Fatalf("optional key: status %d", rec.Code)
	}
}

func TestFailuresAreNotStored(t *testing.T) {
	c := &creator{}
	h := idempotentHandler(c, Production(true))
	do(h, http.MethodPost, "/invoices.create", `{"name":"boom"}`, map[string]string{"Idempotency-Key": "k"})
	rec := do(h, http.MethodPost, "/invoices.create", `{"name":"boom"}`, map[string]string{"Idempotency-Key": "k"})
	if rec.Header().Get("Idempotent-Replayed") != "" || c.calls.Load() != 2 {
		t.Fatalf("a 5xx must not be replayed: calls %d headers %v", c.calls.Load(), rec.Header())
	}
	bad := do(h, http.MethodPost, "/invoices.create", `{"name":""}`, map[string]string{"Idempotency-Key": "v"})
	again := do(h, http.MethodPost, "/invoices.create", `{"name":""}`, map[string]string{"Idempotency-Key": "v"})
	if bad.Code != 400 || again.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("a 4xx is deterministic and replayed: %d %v", again.Code, again.Header())
	}
}

func TestIdempotentPanicsOnQuery(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "mutations only") {
			t.Fatalf("got %v", r)
		}
	}()
	NewRouter(Query("get", getUser, Idempotent()))
}
