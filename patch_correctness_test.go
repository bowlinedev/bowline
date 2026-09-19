package bowline

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type postRow struct {
	ID     int64  `json:"id"`
	PostID int64  `json:"postId"`
	Owner  string `json:"owner"`
	Body   string `json:"body"`
}

type postKey struct {
	ID     int64 `json:"id"`
	PostID int64 `json:"postId"`
}

func overlappingPatchAPI(t *testing.T) http.Handler {
	t.Helper()
	bare := func(ctx context.Context, in postKey) (postRow, error) {
		return postRow{ID: in.ID, PostID: in.PostID, Owner: "bare", Body: "bare"}, nil
	}
	nested := func(ctx context.Context, in postKey) (postRow, error) {
		return postRow{ID: in.ID, PostID: in.PostID, Owner: "nested", Body: "nested"}, nil
	}
	saveBare := func(ctx context.Context, in postRow) (postRow, error) {
		in.Owner = "bare"
		return in, nil
	}
	saveNested := func(ctx context.Context, in postRow) (postRow, error) {
		in.Owner = "nested"
		return in, nil
	}
	return NewRouter(
		Query("getPost", bare, Path("posts/{postId}")),
		Mutation("savePost", saveBare, Path("posts/{postId}"), Method("PUT")),
		Query("getUserPost", nested, Path("users/{id}/posts/{postId}")),
		Mutation("saveUserPost", saveNested, Path("users/{id}/posts/{postId}"), Method("PUT")),
	).Handler(AutoPatch(), Logger(discardLogger()))
}

func TestPatchPicksTheSameRouteAsTheRequestMethod(t *testing.T) {
	const path = "/api/users/7/posts/3"
	want := do(overlappingPatchAPI(t), http.MethodGet, path, "", nil).Body.String()
	seen := map[string]int{}
	for range 200 {
		rec := do(overlappingPatchAPI(t), http.MethodPatch, path, `{"body":"x"}`,
			map[string]string{"Content-Type": MergePatchMediaType})
		seen[rec.Body.String()]++
	}
	if len(seen) != 1 {
		t.Fatalf("one PATCH path reached several procedures: %v", seen)
	}
	for got := range seen {
		if !strings.Contains(got, `"owner":"nested"`) {
			t.Fatalf("PATCH chose %s but GET chose %s", got, want)
		}
	}
}

func TestJSONPatchCopyDoesNotAliasItsSource(t *testing.T) {
	for _, tc := range []struct{ name, doc, patch, want string }{
		{
			"object",
			`{"a":{"x":1}}`,
			`[{"op":"copy","from":"/a","path":"/b"},{"op":"replace","path":"/b/x","value":2}]`,
			`{"a":{"x":1},"b":{"x":2}}`,
		},
		{
			"array",
			`{"arr":[{"n":1}]}`,
			`[{"op":"copy","from":"/arr","path":"/dup"},{"op":"replace","path":"/dup/0/n","value":9}]`,
			`{"arr":[{"n":1}],"dup":[{"n":9}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyJSONPatch([]byte(tc.doc), []byte(tc.patch))
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestPatchPreservesIntegersBeyondFloat64(t *testing.T) {
	const doc = `{"id":9007199254740993,"name":"a"}`
	const want = `{"id":9007199254740993,"name":"b"}`
	merged, err := applyMergePatch([]byte(doc), []byte(`{"name":"b"}`))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if string(merged) != want {
		t.Fatalf("merge patch rounded an int64\n got %s\nwant %s", merged, want)
	}
	patched, err := applyJSONPatch([]byte(doc), []byte(`[{"op":"replace","path":"/name","value":"b"}]`))
	if err != nil {
		t.Fatalf("jsonpatch: %v", err)
	}
	if string(patched) != want {
		t.Fatalf("json patch rounded an int64\n got %s\nwant %s", patched, want)
	}
}

func TestJSONPatchTestComparesNumbersByValue(t *testing.T) {
	if _, err := applyJSONPatch([]byte(`{"n":1}`), []byte(`[{"op":"test","path":"/n","value":1.0}]`)); err != nil {
		t.Fatalf("1 and 1.0 should compare equal: %v", err)
	}
	if _, err := applyJSONPatch([]byte(`{"n":1}`), []byte(`[{"op":"test","path":"/n","value":2}]`)); err == nil {
		t.Fatal("1 and 2 compared equal")
	}
}

func TestPatchForwardsIfMatchToTheWrite(t *testing.T) {
	seen := make(chan string, 4)
	read := func(ctx context.Context, in getInput) (twoField, error) {
		return twoField{ID: 1, A: "start", B: "start"}, nil
	}
	write := func(ctx context.Context, in twoField) (twoField, error) {
		if tags := IfMatch(ctx); len(tags) > 0 {
			seen <- tags[0]
		} else {
			seen <- ""
		}
		return in, nil
	}
	h := NewRouter(
		Query("get", read, Path("things/{id}")),
		Mutation("save", write, Path("things/{id}"), Method("PUT")),
	).Handler(AutoPatch(), ETags(), Logger(discardLogger()))
	tag := do(h, http.MethodGet, "/api/things/1", "", nil).Header().Get("ETag")
	if tag == "" {
		t.Fatal("the read produced no entity tag")
	}
	rec := do(h, http.MethodPatch, "/api/things/1", `{"a":"A"}`,
		map[string]string{"Content-Type": MergePatchMediaType, "If-Match": tag})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch failed: %d %s", rec.Code, rec.Body)
	}
	if got := <-seen; got == "" {
		t.Fatal("the write never saw If-Match, so a store cannot enforce it")
	}
}

func TestPrecisionFastPathTriggersWhenNeeded(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want bool
	}{
		{"small int", `{"n":12345}`, false},
		{"fifteen digits", `{"n":123456789012345}`, false},
		{"sixteen digits", `{"n":1234567890123456}`, true},
		{"beyond 2^53", `{"n":9007199254740993}`, true},
		{"split across dot", `{"n":1234567.89012345678}`, true},
		{"short fraction", `{"n":123456789.123456}`, false},
		{"long fraction", `{"n":0.12345678901234567}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mayLosePrecision([]byte(tc.doc)); got != tc.want {
				t.Fatalf("mayLosePrecision(%s) = %v, want %v", tc.doc, got, tc.want)
			}
		})
	}
}

func TestPatchRoundTripsPrecisionSensitiveNumbers(t *testing.T) {
	for _, doc := range []string{
		`{"id":9007199254740993,"name":"a"}`,
		`{"id":1234567.8901234568,"name":"a"}`,
		`{"id":123456789012345678901234567890,"name":"a"}`,
	} {
		got, err := applyMergePatch([]byte(doc), []byte(`{"name":"b"}`))
		if err != nil {
			t.Fatalf("merge %s: %v", doc, err)
		}
		want := doc[:len(doc)-len(`"a"}`)] + `"b"}`
		if string(got) != want {
			t.Fatalf("\n got %s\nwant %s", got, want)
		}
	}
}
