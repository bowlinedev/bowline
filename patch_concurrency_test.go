package bowline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPatchIsGuardedByCSRF(t *testing.T) {
	h, _ := newPatchAPI2(CSRF(CSRFOptions{}))
	req := httptest.NewRequest(http.MethodPatch, "/api/users/1", strings.NewReader(`{"name":"evil"}`))
	req.Header.Set("Content-Type", MergePatchMediaType)
	req.Host = "app.example.com"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a forged cross-origin PATCH answered %d, want 403", rec.Code)
	}
}

func TestPatchHonoursTheIdempotencyKey(t *testing.T) {
	var runs atomic.Int64
	h, _ := newPatchAPICounting(&runs)
	headers := map[string]string{"Content-Type": MergePatchMediaType, "Idempotency-Key": "k1"}
	do(h, http.MethodPatch, "/api/users/1", `{"name":"a"}`, headers)
	do(h, http.MethodPatch, "/api/users/1", `{"name":"a"}`, headers)
	if runs.Load() != 1 {
		t.Fatalf("the same idempotency key ran the write %d times", runs.Load())
	}
}

func newPatchAPI2(opts ...HandlerOption) (http.Handler, *patchStore) {
	store := &patchStore{rows: map[int64]user{1: {ID: 1, Name: "ada"}}}
	read := func(ctx context.Context, in getInput) (user, error) {
		store.mu.Lock()
		defer store.mu.Unlock()
		return store.rows[in.ID], nil
	}
	write := func(ctx context.Context, in user) (user, error) {
		store.mu.Lock()
		defer store.mu.Unlock()
		store.rows[in.ID] = in
		return in, nil
	}
	options := append([]HandlerOption{AutoPatch(), Logger(discardLogger())}, opts...)
	return NewRouter(
		Query("get", read, Path("users/{id}")),
		Mutation("save", write, Path("users/{id}"), Method("PUT"), Idempotent()),
	).Handler(options...), store
}

func newPatchAPICounting(runs *atomic.Int64) (http.Handler, *patchStore) {
	store := &patchStore{rows: map[int64]user{1: {ID: 1, Name: "ada"}}}
	read := func(ctx context.Context, in getInput) (user, error) { return store.rows[in.ID], nil }
	write := func(ctx context.Context, in user) (user, error) {
		runs.Add(1)
		return in, nil
	}
	return NewRouter(
		Query("get", read, Path("users/{id}")),
		Mutation("save", write, Path("users/{id}"), Method("PUT"), Idempotent()),
	).Handler(AutoPatch(), Idempotency(MemoryIdempotencyStore(), time.Hour), Logger(discardLogger())), store
}
