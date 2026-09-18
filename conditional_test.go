package bowline

import (
	"context"
	"net/http"
	"testing"
)

func etagHandler(t *testing.T, opts ...HandlerOption) http.Handler {
	t.Helper()
	options := append([]HandlerOption{Logger(discardLogger())}, opts...)
	return NewRouter(
		Query("get", getUser, Path("users/{id}")),
		Mutation("save", createUser, Path("users/{id}"), Method("PUT")),
	).Handler(options...)
}

func TestETagIsSetOnAReadAndRepliesNotModified(t *testing.T) {
	h := etagHandler(t, ETags())
	first := do(h, http.MethodGet, "/api/users/1", "", nil)
	if first.Code != http.StatusOK {
		t.Fatalf("status %d: %s", first.Code, first.Body.String())
	}
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag on a cacheable read")
	}
	second := do(h, http.MethodGet, "/api/users/1", "", map[string]string{"If-None-Match": tag})
	if second.Code != http.StatusNotModified {
		t.Fatalf("status %d, want 304", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("a 304 must carry no body, got %q", second.Body.String())
	}
	if second.Header().Get("ETag") != tag {
		t.Fatalf("a 304 must repeat the ETag, got %q", second.Header().Get("ETag"))
	}
}

func TestETagChangesWithTheBody(t *testing.T) {
	h := etagHandler(t, ETags())
	one := do(h, http.MethodGet, "/api/users/1", "", nil).Header().Get("ETag")
	two := do(h, http.MethodGet, "/api/users/2", "", nil).Header().Get("ETag")
	if one == two {
		t.Fatalf("two different bodies share the ETag %q", one)
	}
	if again := do(h, http.MethodGet, "/api/users/1", "", nil).Header().Get("ETag"); again != one {
		t.Fatalf("the same body produced %q then %q", one, again)
	}
}

func TestAStaleIfNoneMatchStillReturnsTheBody(t *testing.T) {
	h := etagHandler(t, ETags())
	rec := do(h, http.MethodGet, "/api/users/1", "", map[string]string{"If-None-Match": `"stale"`})
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestETagsAreOffUntilEnabled(t *testing.T) {
	if tag := do(etagHandler(t), http.MethodGet, "/api/users/1", "", nil).Header().Get("ETag"); tag != "" {
		t.Fatalf("ETag %q was set without the option", tag)
	}
}

func TestOnlyCacheableMethodsGetAnETag(t *testing.T) {
	h := etagHandler(t, ETags())
	rec := do(h, http.MethodPut, "/api/users/1", `{"id":1,"name":"ada"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if tag := rec.Header().Get("ETag"); tag != "" {
		t.Fatalf("a PUT answered with ETag %q; only a cacheable read should", tag)
	}
}

func TestAProcedureCanSetItsOwnETag(t *testing.T) {
	versioned := func(ctx context.Context, in getInput) (user, error) {
		CallFrom(ctx).SetETag("v7")
		return user{ID: in.ID, Name: "ada"}, nil
	}
	h := NewRouter(Query("get", versioned, Path("users/{id}"))).Handler(Logger(discardLogger()))
	rec := do(h, http.MethodGet, "/api/users/1", "", nil)
	if got := rec.Header().Get("ETag"); got != `"v7"` {
		t.Fatalf("ETag %q, want %q", got, `"v7"`)
	}
	again := do(h, http.MethodGet, "/api/users/1", "", map[string]string{"If-None-Match": `"v7"`})
	if again.Code != http.StatusNotModified {
		t.Fatalf("status %d, want 304 for a procedure-supplied ETag", again.Code)
	}
}

func TestIfMatchReachesTheProcedure(t *testing.T) {
	var seen []string
	guard := func(ctx context.Context, in user) (user, error) {
		seen = IfMatch(ctx)
		if !matchesETag(seen, `"v1"`) {
			return user{}, Errorf(FailedPrecondition, "the resource has changed")
		}
		return in, nil
	}
	h := NewRouter(Mutation("save", guard, Path("users/{id}"), Method("PUT"))).Handler(Logger(discardLogger()))
	stale := do(h, http.MethodPut, "/api/users/1", `{"id":1,"name":"ada"}`, map[string]string{"If-Match": `"v0"`})
	if stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("status %d, want 412: %s", stale.Code, stale.Body.String())
	}
	fresh := do(h, http.MethodPut, "/api/users/1", `{"id":1,"name":"ada"}`, map[string]string{"If-Match": `"v1", "v2"`})
	if fresh.Code != http.StatusOK {
		t.Fatalf("status %d: %s", fresh.Code, fresh.Body.String())
	}
	if len(seen) != 2 {
		t.Fatalf("a comma separated If-Match parsed to %v", seen)
	}
}

func TestStarMatchesAnyETag(t *testing.T) {
	if !matchesETag([]string{"*"}, `"anything"`) {
		t.Fatal("* must match any entity tag")
	}
}
