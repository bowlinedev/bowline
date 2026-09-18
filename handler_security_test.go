package bowline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const secretMarker = "s3cr3t-dsn-user:pw@10.0.0.5"

func envelopeOf(t *testing.T, rec *httptest.ResponseRecorder) wireError {
	t.Helper()
	var env wireEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not an error envelope: %s", rec.Body.String())
	}
	return env.Error
}

func routerReturning(err error) http.Handler {
	fail := func(ctx context.Context, in getInput) (user, error) { return user{}, err }
	return NewRouter(Query("boom", fail)).Handler(Production(true), Logger(discardLogger()))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestProductionRedactsEveryNonBowlineError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   Code
		body   string
	}{
		{"plain error", errors.New(secretMarker), 500, Internal, "internal error"},
		{"wrapped error", fmt.Errorf("connecting: %w", errors.New(secretMarker)), 500, Internal, "internal error"},
		{"bowline internal", Errorf(Internal, "dial %s", secretMarker), 500, Internal, "internal error"},
		{"bowline unavailable", Errorf(Unavailable, "upstream %s is down", secretMarker), 503, Unavailable, "internal error"},
		{"bowline data loss", Errorf(DataLoss, "corrupt page at %s", secretMarker), 500, DataLoss, "internal error"},
		{"bowline unknown", Errorf(Unknown, "unclassified %s", secretMarker), 500, Unknown, "internal error"},
		{"bowline not found", Errorf(NotFound, "no user 7"), 404, NotFound, "no user 7"},
		{"bowline invalid argument", Errorf(InvalidArgument, "id must be positive"), 400, InvalidArgument, "id must be positive"},
		{"bowline permission denied", Errorf(PermissionDenied, "not your invoice"), 403, PermissionDenied, "not your invoice"},
		{"canceled", context.Canceled, 408, Canceled, "request canceled"},
		{"deadline exceeded", context.DeadlineExceeded, 408, DeadlineExceeded, "deadline exceeded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(routerReturning(tc.err), http.MethodGet, "/api/boom?input=%7B%22id%22%3A1%7D", "", nil)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d, body %s", rec.Code, tc.status, rec.Body.String())
			}
			got := envelopeOf(t, rec)
			if got.Code != tc.code {
				t.Fatalf("code %q, want %q", got.Code, tc.code)
			}
			if got.Message != tc.body {
				t.Fatalf("message %q, want %q", got.Message, tc.body)
			}
			if strings.Contains(rec.Body.String(), secretMarker) {
				t.Fatalf("body leaked the error detail: %s", rec.Body.String())
			}
		})
	}
}

func TestProductionRedactsDetailsAndIssuesOnServerErrors(t *testing.T) {
	err := Errorf(Internal, "broken").WithDetails(map[string]string{"dsn": secretMarker})
	err.Issues = []Issue{{Path: []string{"dsn"}, Rule: "url", Message: secretMarker}}
	rec := do(routerReturning(err), http.MethodGet, "/api/boom?input=%7B%22id%22%3A1%7D", "", nil)
	if strings.Contains(rec.Body.String(), secretMarker) {
		t.Fatalf("body leaked details or issues: %s", rec.Body.String())
	}
	got := envelopeOf(t, rec)
	if got.Details != nil || got.Issues != nil {
		t.Fatalf("details %v issues %v, want both empty", got.Details, got.Issues)
	}
}

func TestDevelopmentKeepsServerErrorDetail(t *testing.T) {
	fail := func(ctx context.Context, in getInput) (user, error) { return user{}, errors.New(secretMarker) }
	h := NewRouter(Query("boom", fail)).Handler(Logger(discardLogger()))
	rec := do(h, http.MethodGet, "/api/boom?input=%7B%22id%22%3A1%7D", "", nil)
	if !strings.Contains(envelopeOf(t, rec).Message, secretMarker) {
		t.Fatalf("development hid the error: %s", rec.Body.String())
	}
}

func TestPanicNeverLeaksStackInBody(t *testing.T) {
	panicking := func(ctx context.Context, in getInput) (user, error) { panic(secretMarker) }
	for _, tc := range []struct {
		name       string
		production bool
		message    string
	}{
		{"production", true, "internal error"},
		{"development", false, "panic: " + secretMarker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewRouter(Query("boom", panicking)).Handler(Production(tc.production), Logger(discardLogger()))
			rec := do(h, http.MethodGet, "/api/boom?input=%7B%22id%22%3A1%7D", "", nil)
			if rec.Code != 500 {
				t.Fatalf("status %d, want 500", rec.Code)
			}
			if got := envelopeOf(t, rec).Message; got != tc.message {
				t.Fatalf("message %q, want %q", got, tc.message)
			}
			for _, needle := range []string{"goroutine ", "runtime/debug", ".go:", "bowline.(*handler)"} {
				if strings.Contains(rec.Body.String(), needle) {
					t.Fatalf("body contains stack fragment %q: %s", needle, rec.Body.String())
				}
			}
		})
	}
}

func TestPanicStackReachesTheLogger(t *testing.T) {
	panicking := func(ctx context.Context, in getInput) (user, error) { panic("kaboom") }
	var logged strings.Builder
	h := NewRouter(Query("boom", panicking)).Handler(
		Production(true),
		Logger(slog.New(slog.NewTextHandler(&logged, nil))),
	)
	do(h, http.MethodGet, "/api/boom?input=%7B%22id%22%3A1%7D", "", nil)
	if !strings.Contains(logged.String(), "goroutine ") {
		t.Fatalf("the stack never reached the logger: %s", logged.String())
	}
}

func TestErrorBodiesNeverEchoRequestBody(t *testing.T) {
	const marker = "9182736450"
	bodies := map[string]string{
		"type mismatch":      `{"id":"` + secretMarker + `"}`,
		"number into string": `{"name":` + marker + `}`,
		"syntax error":       `{"id":` + marker + `,,}`,
		"trailing data":      `{"id":1} ` + marker,
	}
	for _, strict := range []bool{false, true} {
		for name, body := range bodies {
			t.Run(fmt.Sprintf("strict=%v/%s", strict, name), func(t *testing.T) {
				opts := []HandlerOption{Production(true), Logger(discardLogger())}
				if strict {
					opts = append(opts, StrictInput())
				}
				h := NewRouter(Mutation("create", createUser)).Handler(opts...)
				rec := do(h, http.MethodPost, "/api/create", body, nil)
				if rec.Code == 200 {
					return
				}
				for _, needle := range []string{marker, secretMarker} {
					if strings.Contains(rec.Body.String(), needle) {
						t.Fatalf("error body echoed request content %q: %s", needle, rec.Body.String())
					}
				}
				if got := envelopeOf(t, rec).Message; got != "invalid input" {
					t.Fatalf("message %q, want %q", got, "invalid input")
				}
			})
		}
	}
}

func TestBodyLimitAppliesBeforeDecode(t *testing.T) {
	var called atomic.Int64
	counting := func(ctx context.Context, in user) (user, error) {
		called.Add(1)
		return in, nil
	}
	h := NewRouter(Mutation("create", counting)).Handler(MaxBodySize(64), Logger(discardLogger()))
	body := `{"name":"` + strings.Repeat("a", 512) + `"}`
	rec := do(h, http.MethodPost, "/api/create", body, nil)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413, body %s", rec.Code, rec.Body.String())
	}
	if called.Load() != 0 {
		t.Fatalf("the procedure ran %d times for an oversized body", called.Load())
	}
	if got := envelopeOf(t, rec).Code; got != InvalidArgument {
		t.Fatalf("code %q, want %q", got, InvalidArgument)
	}
	if strings.Contains(rec.Body.String(), strings.Repeat("a", 64)) {
		t.Fatalf("the 413 body echoed the request: %s", rec.Body.String())
	}
}

func TestUploadInputPartLimitAppliesBeforeDecode(t *testing.T) {
	h := uploadHandler(MaxBodySize(32), Logger(discardLogger()))
	ct, body := multipartBody(t, [][3]string{
		{"input", "", `{"invoiceId":1,"pad":"` + strings.Repeat("a", 512) + `"}`},
		{"file", "note.txt", "hello"},
	})
	rec := do(h, http.MethodPost, "/api/invoices.attach", body.String(), map[string]string{"Content-Type": ct})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(envelopeOf(t, rec).Message, "input part exceeds 32 bytes") {
		t.Fatalf("message %q", envelopeOf(t, rec).Message)
	}
}

func TestBodyLimitClosesTheConnection(t *testing.T) {
	h := NewRouter(Mutation("create", createUser)).Handler(MaxBodySize(64), Logger(discardLogger()))
	srv := httptest.NewServer(h)
	defer srv.Close()
	body := `{"name":"` + strings.Repeat("a", 1<<16) + `"}`
	resp, err := http.Post(srv.URL+"/api/create", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413", resp.StatusCode)
	}
	if !resp.Close && resp.Header.Get("Connection") != "close" {
		t.Fatal("the 413 response never asked net/http to close the connection")
	}
}

func TestProductionRedactionFollowsTheStatusOfEveryCode(t *testing.T) {
	for code, status := range httpStatus {
		t.Run(string(code), func(t *testing.T) {
			failure := Errorf(code, "dialing %s", secretMarker).WithDetails(map[string]any{"host": secretMarker})
			rec := do(routerReturning(failure), http.MethodPost, "/api/boom", `{"id":1}`, nil)
			if rec.Code != status {
				t.Fatalf("status %d, want %d", rec.Code, status)
			}
			env := envelopeOf(t, rec)
			if env.Code != code {
				t.Fatalf("code %q, want %q", env.Code, code)
			}
			if status >= 500 {
				if strings.Contains(rec.Body.String(), secretMarker) {
					t.Fatalf("a %d body leaked the underlying detail: %s", status, rec.Body.String())
				}
				if env.Message != "internal error" {
					t.Fatalf("message %q, want %q", env.Message, "internal error")
				}
				if env.Details != nil || len(env.Issues) > 0 {
					t.Fatalf("a %d response kept details or issues: %s", status, rec.Body.String())
				}
				return
			}
			if !strings.Contains(env.Message, secretMarker) {
				t.Fatalf("a %d message is meant for the caller and must survive redaction: %q", status, env.Message)
			}
		})
	}
}

func TestSensitiveInputNeverTravelsInAURL(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("declaring Sensitive with %s was accepted; the input would travel in the URL", method)
				}
			}()
			NewRouter(Query("secret", getUser, Sensitive(), Path("secret/{id}"), Method(method)))
		})
	}
}

func TestSensitiveQueryIsNotReachableOverGET(t *testing.T) {
	h := NewRouter(Query("secret", getUser, Sensitive())).Handler(Logger(discardLogger()))
	rec := do(h, http.MethodGet, "/api/secret?input=%7B%22id%22%3A1%7D", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, want 405; a sensitive input must not be reachable through the URL", rec.Code)
	}
}
