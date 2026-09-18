package bowline

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

type twoField struct {
	ID int64  `json:"id"`
	A  string `json:"a"`
	B  string `json:"b"`
}

func barrierAPI(t *testing.T, barrier *sync.WaitGroup, opts ...PatchOption) (http.Handler, func() twoField) {
	t.Helper()
	var mu sync.Mutex
	row := twoField{ID: 1, A: "start", B: "start"}
	read := func(ctx context.Context, in getInput) (twoField, error) {
		mu.Lock()
		snapshot := row
		mu.Unlock()
		if barrier != nil {
			barrier.Done()
			barrier.Wait()
		}
		return snapshot, nil
	}
	write := func(ctx context.Context, in twoField) (twoField, error) {
		mu.Lock()
		defer mu.Unlock()
		row = in
		return in, nil
	}
	h := NewRouter(
		Query("get", read, Path("things/{id}")),
		Mutation("save", write, Path("things/{id}"), Method("PUT")),
	).Handler(AutoPatch(opts...), Logger(discardLogger()))
	return h, func() twoField {
		mu.Lock()
		defer mu.Unlock()
		return row
	}
}

func racePair(h http.Handler) (ok int) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, patch := range []string{`{"a":"A"}`, `{"b":"B"}`} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := do(h, http.MethodPatch, "/api/things/1", patch, map[string]string{"Content-Type": MergePatchMediaType})
			if rec.Code == http.StatusOK {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return ok
}

func TestConcurrentPatchLosesAnUpdateWithoutIfMatch(t *testing.T) {
	var barrier sync.WaitGroup
	barrier.Add(2)
	h, final := barrierAPI(t, &barrier)
	if accepted := racePair(h); accepted != 2 {
		t.Fatalf("%d patches accepted, want 2", accepted)
	}
	got := final()
	if got.A == "A" && got.B == "B" {
		t.Fatalf("both survived %+v; the interleaving did not happen so this test proves nothing", got)
	}
	t.Logf("as expected, an unconditional PATCH lost one update: %+v", got)
}

func TestRequireIfMatchRefusesTheRacingPatches(t *testing.T) {
	var barrier sync.WaitGroup
	barrier.Add(2)
	h, final := barrierAPI(t, &barrier, RequireIfMatch())
	if accepted := racePair(h); accepted != 0 {
		t.Fatalf("%d unconditional patches were accepted under RequireIfMatch", accepted)
	}
	if got := final(); got.A != "start" || got.B != "start" {
		t.Fatalf("a refused PATCH still wrote: %+v", got)
	}
}

func TestPatchHonoursIfMatch(t *testing.T) {
	h, store := newPatchAPI()
	tag := do(h, http.MethodGet, "/api/users/1", "", nil).Header().Get("ETag")
	if tag == "" {
		read := do(h, http.MethodGet, "/api/users/1", "", nil)
		tag = etagOf(read.Body.Bytes())
	}
	stale := do(h, http.MethodPatch, "/api/users/1", `{"name":"x"}`, map[string]string{"Content-Type": MergePatchMediaType, "If-Match": `"nope"`})
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("a stale If-Match answered %d, want 412", stale.Code)
	}
	if store.rows[1].Name != "ada" {
		t.Fatalf("a refused PATCH still wrote: %+v", store.rows[1])
	}
	fresh := do(h, http.MethodPatch, "/api/users/1", `{"name":"grace"}`, map[string]string{"Content-Type": MergePatchMediaType, "If-Match": tag})
	if fresh.Code != http.StatusOK {
		t.Fatalf("a matching If-Match answered %d: %s", fresh.Code, fresh.Body.String())
	}
	if store.rows[1].Name != "grace" {
		t.Fatalf("the write did not happen: %+v", store.rows[1])
	}
}

func TestRequireIfMatchRefusesAnUnconditionalPatch(t *testing.T) {
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
	h := NewRouter(
		Query("get", read, Path("users/{id}")),
		Mutation("save", write, Path("users/{id}"), Method("PUT")),
	).Handler(AutoPatch(RequireIfMatch()), Logger(discardLogger()))
	rec := do(h, http.MethodPatch, "/api/users/1", `{"name":"x"}`, map[string]string{"Content-Type": MergePatchMediaType})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status %d, want 428", rec.Code)
	}
	if rec.Header().Get("ETag") == "" {
		t.Fatal("a 428 must tell the caller the current entity tag")
	}
	if store.rows[1].Name != "ada" {
		t.Fatalf("it wrote anyway: %+v", store.rows[1])
	}
}

func TestPatchRejectsAWeakIfMatch(t *testing.T) {
	h, store := newPatchAPI()
	read := do(h, http.MethodGet, "/api/users/1", "", nil)
	tag := etagOf(read.Body.Bytes())
	weak := "W/" + tag
	rec := do(h, http.MethodPatch, "/api/users/1", `{"name":"x"}`, map[string]string{"Content-Type": MergePatchMediaType, "If-Match": weak})
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status %d, want 412; If-Match uses strong comparison so a weak tag must not match", rec.Code)
	}
	if store.rows[1].Name != "ada" {
		t.Fatalf("it wrote anyway: %+v", store.rows[1])
	}
}

func TestPatchHonoursIfNoneMatch(t *testing.T) {
	h, store := newPatchAPI()
	read := do(h, http.MethodGet, "/api/users/1", "", nil)
	tag := etagOf(read.Body.Bytes())
	rec := do(h, http.MethodPatch, "/api/users/1", `{"name":"x"}`, map[string]string{"Content-Type": MergePatchMediaType, "If-None-Match": tag})
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status %d, want 412; an unsafe method whose If-None-Match matches must be refused", rec.Code)
	}
	if store.rows[1].Name != "ada" {
		t.Fatalf("it wrote anyway: %+v", store.rows[1])
	}
}
