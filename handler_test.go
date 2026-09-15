package bowline

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func failing(ctx context.Context, in getInput) (user, error) {
	switch in.ID {
	case 1:
		return user{}, Errorf(NotFound, "no user %d", in.ID)
	case 2:
		return user{}, errors.New("boom")
	case 3:
		panic("kaboom")
	}
	return user{ID: in.ID}, nil
}

func testHandler(opts ...HandlerOption) http.Handler {
	users := NewRouter(
		Query("get", failing, Description("Get")),
		Query("old", getUser, Deprecated("use get")),
		Query("search", getUser, Sensitive()),
		Mutation("create", createUser),
	)
	return NewRouter(Mount("users", users), Query("health", health)).Handler(opts...)
}

func do(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost && headers["Content-Type"] == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) Code {
	t.Helper()
	var env wireEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not an error envelope: %s", rec.Body.String())
	}
	return env.Error.Code
}

func TestQueryViaGet(t *testing.T) {
	rec := do(testHandler(), http.MethodGet, "/api/users.get?input="+url.QueryEscape(`{"id":7}`), "", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content type %q", ct)
	}
	var u user
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil || u.ID != 7 {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestQueryViaPostAndMissingInput(t *testing.T) {
	rec := do(testHandler(), http.MethodPost, "/users.get", `{"id":8}`, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":8`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	rec = do(testHandler(), http.MethodGet, "/health", "", nil)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "{}" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestMethodRules(t *testing.T) {
	h := testHandler()
	cases := []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/users.create", 405},
		{http.MethodGet, "/users.search", 405},
		{http.MethodPost, "/users.search", 200},
		{http.MethodPut, "/users.get", 405},
		{http.MethodPost, "/users.create", 200},
	}
	for _, tc := range cases {
		rec := do(h, tc.method, tc.path, `{"id":1,"name":"x"}`, nil)
		if rec.Code != tc.status {
			t.Errorf("%s %s: status %d, want %d: %s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}
		if tc.status == 405 && rec.Header().Get("Allow") == "" {
			t.Errorf("%s %s: missing Allow header", tc.method, tc.path)
		}
	}
}

func TestErrorMapping(t *testing.T) {
	h := testHandler(Production(true), Logger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	cases := []struct {
		id     int
		status int
		code   Code
	}{
		{1, 404, NotFound},
		{2, 500, Internal},
		{3, 500, Internal},
	}
	for _, tc := range cases {
		rec := do(h, http.MethodPost, "/users.get", `{"id":`+strconv.Itoa(tc.id)+`}`, nil)
		if rec.Code != tc.status || errorCode(t, rec) != tc.code {
			t.Errorf("id %d: status %d code %s body %s", tc.id, rec.Code, errorCode(t, rec), rec.Body.String())
		}
	}
	rec := do(h, http.MethodPost, "/users.get", `{"id":2}`, nil)
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatal("production mode leaked the error message")
	}
	dev := testHandler(Logger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	rec = do(dev, http.MethodPost, "/users.get", `{"id":2}`, nil)
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatal("development mode should include the error message")
	}
}

func TestUnknownProcedureAndBadInput(t *testing.T) {
	h := testHandler()
	rec := do(h, http.MethodGet, "/nope", "", nil)
	if rec.Code != 404 || errorCode(t, rec) != Unimplemented {
		t.Fatalf("unknown: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPost, "/users.get", `{"id":`, nil)
	if rec.Code != 400 || errorCode(t, rec) != InvalidArgument {
		t.Fatalf("malformed: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodPost, "/users.get", `{"id":1}`, map[string]string{"Content-Type": "text/plain"})
	if rec.Code != 415 || errorCode(t, rec) != InvalidArgument {
		t.Fatalf("content type: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(h, http.MethodGet, "/users.get?input="+url.QueryEscape(`{"id":4,"zzz":1}`), "", nil)
	if rec.Code != 200 {
		t.Fatalf("lenient unknown field: %d %s", rec.Code, rec.Body.String())
	}
	strict := testHandler(StrictInput())
	rec = do(strict, http.MethodGet, "/users.get?input="+url.QueryEscape(`{"id":4,"zzz":1}`), "", nil)
	if rec.Code != 400 {
		t.Fatalf("strict unknown field: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBodyLimit(t *testing.T) {
	h := testHandler(MaxBodySize(16))
	rec := do(h, http.MethodPost, "/users.create", `{"id":1,"name":"`+strings.Repeat("x", 100)+`"}`, nil)
	if rec.Code != 413 || errorCode(t, rec) != InvalidArgument {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestDeprecationHeader(t *testing.T) {
	rec := do(testHandler(), http.MethodGet, "/users.old?input="+url.QueryEscape(`{"id":1}`), "", nil)
	if rec.Header().Get("Deprecation") != "true" {
		t.Fatalf("missing Deprecation header, got %v", rec.Header())
	}
}

func TestCallIsInContext(t *testing.T) {
	var seen *Call
	mw := func(next Next) Next {
		return func(ctx context.Context, in any) (any, error) {
			seen = CallFrom(ctx)
			return next(ctx, in)
		}
	}
	h := NewRouter(Query("get", getUser, Use(mw))).Handler()
	do(h, http.MethodPost, "/get", `{"id":1}`, nil)
	if seen == nil || seen.Procedure.Path != "get" || seen.Request == nil {
		t.Fatalf("call not populated: %+v", seen)
	}
}

func TestTrailingSlashAndPrefix(t *testing.T) {
	rec := do(testHandler(), http.MethodGet, "/v1/api/health/", "", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
