package bowline

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func problemHandler(t *testing.T, opts ...HandlerOption) http.Handler {
	t.Helper()
	fail := func(ctx context.Context, in getInput) (user, error) {
		return user{}, Errorf(NotFound, "invoice %d not found", in.ID).WithDetails(map[string]any{"id": 3})
	}
	options := append([]HandlerOption{Logger(discardLogger())}, opts...)
	return NewRouter(Query("boom", fail), Query("get", getUser)).Handler(options...)
}

func problemOf(t *testing.T, h http.Handler, accept string) (int, string, map[string]any) {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/boom", `{"id":3}`, map[string]string{"Accept": accept})
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %s", rec.Body.String())
	}
	return rec.Code, rec.Header().Get("Content-Type"), body
}

func TestProblemDetailsShapeFollowsRFC9457(t *testing.T) {
	h := problemHandler(t, ProblemDetails(ProblemTypeBase("https://api.example.com/errors/")))
	status, mediaType, body := problemOf(t, h, ProblemMediaType)
	if status != http.StatusNotFound {
		t.Fatalf("status %d, want 404", status)
	}
	if mediaType != ProblemMediaType+"; charset=utf-8" {
		t.Fatalf("content type %q", mediaType)
	}
	want := map[string]any{
		"type":   "https://api.example.com/errors/NOT_FOUND",
		"title":  "Not Found",
		"status": float64(404),
		"detail": "invoice 3 not found",
		"code":   "NOT_FOUND",
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %#v, want %#v", k, body[k], v)
		}
	}
	if body["instance"] != "/api/boom" {
		t.Errorf("instance = %#v", body["instance"])
	}
	if body["details"] == nil {
		t.Error("details must survive as an extension member")
	}
}

func TestProblemDetailsDefaultsToAboutBlank(t *testing.T) {
	_, _, body := problemOf(t, problemHandler(t, ProblemDetails()), ProblemMediaType)
	if body["type"] != "about:blank" {
		t.Fatalf("type = %#v; RFC 9457 says an absent type means about:blank", body["type"])
	}
}

func TestTheFrozenEnvelopeIsStillTheDefault(t *testing.T) {
	h := problemHandler(t, ProblemDetails())
	for _, accept := range []string{"", "application/json", "*/*"} {
		rec := do(h, http.MethodPost, "/api/boom", `{"id":3}`, map[string]string{"Accept": accept})
		if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("Accept %q gave content type %q; a client that did not ask for problem+json must keep the frozen envelope", accept, got)
		}
		if envelopeOf(t, rec).Code != NotFound {
			t.Fatalf("Accept %q did not produce the bowline envelope: %s", accept, rec.Body.String())
		}
	}
}

func TestProblemDetailsIsOffUntilEnabled(t *testing.T) {
	_, mediaType, _ := problemOf(t, problemHandler(t), ProblemMediaType)
	if mediaType != "application/json; charset=utf-8" {
		t.Fatalf("content type %q; problem+json must be opt in", mediaType)
	}
}

func TestProblemDetailsRedactsInProduction(t *testing.T) {
	boom := func(ctx context.Context, in getInput) (user, error) {
		return user{}, Errorf(Internal, "dial %s", secretMarker)
	}
	h := NewRouter(Query("boom", boom)).Handler(Production(true), ProblemDetails(), Logger(discardLogger()))
	rec := do(h, http.MethodPost, "/api/boom", `{"id":1}`, map[string]string{"Accept": ProblemMediaType})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, secretMarker) {
		t.Fatalf("problem+json leaked the underlying detail: %s", body)
	}
}

func TestProblemTitleReadsWell(t *testing.T) {
	cases := map[Code]string{
		NotFound:           "Not Found",
		InvalidArgument:    "Invalid Argument",
		FailedPrecondition: "Failed Precondition",
		Internal:           "Internal",
	}
	for code, want := range cases {
		if got := problemTitle(code); got != want {
			t.Errorf("problemTitle(%s) = %q, want %q", code, got, want)
		}
	}
}
