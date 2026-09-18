package bowline

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
)

type patchStore struct {
	mu   sync.Mutex
	rows map[int64]user
}

func newPatchAPI() (http.Handler, *patchStore) {
	store := &patchStore{rows: map[int64]user{1: {ID: 1, Name: "ada"}}}
	read := func(ctx context.Context, in getInput) (user, error) {
		store.mu.Lock()
		defer store.mu.Unlock()
		row, ok := store.rows[in.ID]
		if !ok {
			return user{}, Errorf(NotFound, "user %d not found", in.ID)
		}
		return row, nil
	}
	write := func(ctx context.Context, in user) (user, error) {
		store.mu.Lock()
		defer store.mu.Unlock()
		store.rows[in.ID] = in
		return in, nil
	}
	h := NewRouter(
		Query("get", read, Path("users/{id}")),
		Mutation("save", write, Path("users/{id}"), Method("PUT")),
	).Handler(AutoPatch(), Logger(discardLogger()))
	return h, store
}

func patchRequest(t *testing.T, h http.Handler, target, mediaType, body string) (int, user) {
	t.Helper()
	rec := do(h, http.MethodPatch, target, body, map[string]string{"Content-Type": mediaType})
	var out user
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("body is not a user: %s", rec.Body.String())
		}
	}
	return rec.Code, out
}

func TestMergePatchUpdatesOneField(t *testing.T) {
	h, store := newPatchAPI()
	status, got := patchRequest(t, h, "/api/users/1", MergePatchMediaType, `{"name":"grace"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	if got.Name != "grace" || got.ID != 1 {
		t.Fatalf("patched to %+v; the untouched field must survive", got)
	}
	if store.rows[1].Name != "grace" {
		t.Fatalf("the store holds %+v", store.rows[1])
	}
}

func TestJSONPatchAppliesOperations(t *testing.T) {
	h, _ := newPatchAPI()
	status, got := patchRequest(t, h, "/api/users/1", JSONPatchMediaType, `[{"op":"replace","path":"/name","value":"hopper"}]`)
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	if got.Name != "hopper" {
		t.Fatalf("patched to %+v", got)
	}
}

func TestJSONPatchTestOperationCanReject(t *testing.T) {
	h, _ := newPatchAPI()
	status, _ := patchRequest(t, h, "/api/users/1", JSONPatchMediaType, `[{"op":"test","path":"/name","value":"nobody"},{"op":"replace","path":"/name","value":"x"}]`)
	if status != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 when a test operation fails", status)
	}
}

func TestPatchOnAMissingResourceKeepsTheReadError(t *testing.T) {
	h, _ := newPatchAPI()
	status, _ := patchRequest(t, h, "/api/users/99", MergePatchMediaType, `{"name":"x"}`)
	if status != http.StatusNotFound {
		t.Fatalf("status %d, want 404 from the underlying read", status)
	}
}

func TestPatchRejectsAnUnknownMediaType(t *testing.T) {
	h, _ := newPatchAPI()
	status, _ := patchRequest(t, h, "/api/users/1", "text/plain", `{"name":"x"}`)
	if status != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d, want 415", status)
	}
}

func TestPatchIsAbsentUntilEnabled(t *testing.T) {
	h := NewRouter(
		Query("get", getUser, Path("users/{id}")),
		Mutation("save", createUser, Path("users/{id}"), Method("PUT")),
	).Handler(Logger(discardLogger()))
	rec := do(h, http.MethodPatch, "/api/users/1", `{"name":"x"}`, map[string]string{"Content-Type": MergePatchMediaType})
	if rec.Code == http.StatusOK {
		t.Fatal("PATCH answered without AutoPatch")
	}
}

func TestPatchNeedsBothAReadAndAWrite(t *testing.T) {
	h := NewRouter(Query("get", getUser, Path("users/{id}"))).Handler(AutoPatch(), Logger(discardLogger()))
	rec := do(h, http.MethodPatch, "/api/users/1", `{"name":"x"}`, map[string]string{"Content-Type": MergePatchMediaType})
	if rec.Code == http.StatusOK {
		t.Fatal("PATCH was generated for a path with no PUT")
	}
}

func TestMergePatchRemovesWithNull(t *testing.T) {
	merged, err := applyMergePatch([]byte(`{"a":1,"b":{"c":2,"d":3}}`), []byte(`{"b":{"c":null},"a":9}`))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(merged, &got)
	inner := got["b"].(map[string]any)
	if _, present := inner["c"]; present {
		t.Fatalf("null must remove the member: %s", merged)
	}
	if inner["d"] != float64(3) || got["a"] != float64(9) {
		t.Fatalf("merge lost or mangled a member: %s", merged)
	}
}

func TestJSONPointerEscapes(t *testing.T) {
	merged, err := applyJSONPatch([]byte(`{"a/b":1,"c~d":2}`), []byte(`[{"op":"replace","path":"/a~1b","value":7},{"op":"replace","path":"/c~0d","value":8}]`))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(merged, &got)
	if got["a/b"] != float64(7) || got["c~d"] != float64(8) {
		t.Fatalf("pointer escaping is wrong: %s", merged)
	}
}

func TestPatchRunsBothProceduresThroughMiddleware(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	record := func(next Next) Next {
		return func(ctx context.Context, in any) (any, error) {
			mu.Lock()
			calls = append(calls, CallFrom(ctx).Procedure.Path)
			mu.Unlock()
			return next(ctx, in)
		}
	}
	read := func(ctx context.Context, in getInput) (user, error) { return user{ID: in.ID, Name: "ada"}, nil }
	write := func(ctx context.Context, in user) (user, error) { return in, nil }
	h := NewRouter(
		Query("get", read, Path("users/{id}")),
		Mutation("save", write, Path("users/{id}"), Method("PUT")),
	).Use(record).Handler(AutoPatch(), Logger(discardLogger()))

	rec := do(h, http.MethodPatch, "/api/users/1", `{"name":"grace"}`, map[string]string{"Content-Type": MergePatchMediaType})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 || calls[0] != "get" || calls[1] != "save" {
		t.Fatalf("middleware saw %v; a PATCH must read and write through the real chain", calls)
	}
}

func TestPatchRejectsAnOversizedBody(t *testing.T) {
	h, _ := newPatchAPI()
	big := `{"name":"` + string(make([]byte, 2<<20)) + `"}`
	rec := do(h, http.MethodPatch, "/api/users/1", big, map[string]string{"Content-Type": MergePatchMediaType})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413", rec.Code)
	}
}
