package bowline

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/signing"
)

var signedSecrets = signing.StaticSecrets{"edge": []byte("shared secret")}

type echoInput struct {
	Name string `json:"name"`
}

type echoOutput struct {
	Name string `json:"name"`
}

func signedRouter(calls *atomic.Int64) *Router {
	echo := func(ctx context.Context, in echoInput) (echoOutput, error) {
		calls.Add(1)
		return echoOutput(in), nil
	}
	return NewRouter(
		Query("read", echo),
		Mutation("write", echo),
	)
}

func signedRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	path := req.URL.EscapedPath()
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}
	var raw []byte
	if method != http.MethodGet {
		raw = []byte(body)
	}
	req.Header.Set(signing.Header, signing.Sign(method, path, raw, "edge", signedSecrets["edge"], time.Now()))
	return req
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSignedAcceptsAProperlySignedCall(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets))
	rec := serve(h, signedRequest(t, http.MethodPost, "/write", `{"name":"ada"}`))
	if rec.Code != 200 || rec.Body.String() != `{"name":"ada"}` {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	rec = serve(h, signedRequest(t, http.MethodGet, "/read?input=%7B%22name%22%3A%22grace%22%7D", ""))
	if rec.Code != 200 || rec.Body.String() != `{"name":"grace"}` {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if calls.Load() != 2 {
		t.Fatalf("calls %d", calls.Load())
	}
}

func TestSignedRejectsUnsignedRequestsBeforeTheProcedureRuns(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets))
	rec := do(h, http.MethodPost, "/write", `{"name":"ada"}`, nil)
	if rec.Code != 401 || errorCode(t, rec) != Unauthenticated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatalf("the procedure ran %d times", calls.Load())
	}
}

func TestSignedRejectsATamperedBodyBeforeDecoding(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets))
	req := signedRequest(t, http.MethodPost, "/write", `{"name":"ada"}`)
	req.Body = io.NopCloser(strings.NewReader(`{"name":"mallory"}`))
	rec := serve(h, req)
	if rec.Code != 401 || errorCode(t, rec) != Unauthenticated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatalf("the procedure ran %d times", calls.Load())
	}
}

func TestSignedRejectsAMovedSignature(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets))
	signed := signedRequest(t, http.MethodPost, "/write", `{"name":"ada"}`)
	replayed := httptest.NewRequest(http.MethodPost, "/read", strings.NewReader(`{"name":"ada"}`))
	replayed.Header.Set("Content-Type", "application/json")
	replayed.Header.Set(signing.Header, signed.Header.Get(signing.Header))
	rec := serve(h, replayed)
	if rec.Code != 401 || calls.Load() != 0 {
		t.Fatalf("status %d calls %d body %s", rec.Code, calls.Load(), rec.Body.String())
	}
}

func TestSignedRejectsStaleTimestamps(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets))
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	stale := time.Now().Add(-10 * time.Minute)
	req.Header.Set(signing.Header, signing.Sign(http.MethodGet, "/read", nil, "edge", signedSecrets["edge"], stale))
	rec := serve(h, req)
	if rec.Code != 401 || calls.Load() != 0 {
		t.Fatalf("status %d calls %d", rec.Code, calls.Load())
	}
}

func TestSignedRejectsAnUnknownKey(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets))
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.Header.Set(signing.Header, signing.Sign(http.MethodGet, "/read", nil, "ghost", []byte("nope"), time.Now()))
	rec := serve(h, req)
	if rec.Code != 401 || errorCode(t, rec) != Unauthenticated || calls.Load() != 0 {
		t.Fatalf("status %d calls %d body %s", rec.Code, calls.Load(), rec.Body.String())
	}
}

func TestSignedDoesNotLeakWhichCheckFailed(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets))
	bodies := map[string]string{}
	for name, req := range map[string]*http.Request{
		"unsigned": httptest.NewRequest(http.MethodGet, "/read", nil),
		"unknown":  httptest.NewRequest(http.MethodGet, "/read", nil),
	} {
		if name == "unknown" {
			req.Header.Set(signing.Header, signing.Sign(http.MethodGet, "/read", nil, "ghost", []byte("nope"), time.Now()))
		}
		bodies[name] = serve(h, req).Body.String()
	}
	if bodies["unsigned"] != bodies["unknown"] {
		t.Fatalf("responses differ: %q vs %q", bodies["unsigned"], bodies["unknown"])
	}
}

func TestSignedProtectsTheReservedPaths(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler(Signed(signedSecrets), WithContract([]byte(reservedDocument)))
	rec := do(h, http.MethodGet, "/.bowline/contract", "", nil)
	if rec.Code != 401 {
		t.Fatalf("unsigned status %d", rec.Code)
	}
	rec = serve(h, signedRequest(t, http.MethodGet, "/.bowline/contract", ""))
	if rec.Code != 200 {
		t.Fatalf("signed status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestSignedRoundTripsThroughTheTransport(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.StripPrefix("/api", signedRouter(&calls).Handler(Signed(signedSecrets))))
	defer srv.Close()
	client := &http.Client{Transport: &signing.Transport{KeyID: "edge", Secret: signedSecrets["edge"]}}

	resp, err := client.Post(srv.URL+"/api/write", "application/json", strings.NewReader(`{"name":"ada"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != `{"name":"ada"}` {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}

	resp, err = client.Get(srv.URL + "/api/read?input=%7B%22name%22%3A%22grace%22%7D")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != `{"name":"grace"}` {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}

	plain := &http.Client{}
	resp, err = plain.Post(srv.URL+"/api/write", "application/json", strings.NewReader(`{"name":"ada"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("unsigned status %d", resp.StatusCode)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls %d", calls.Load())
	}
}

func TestWithoutSignedEveryRequestIsAccepted(t *testing.T) {
	var calls atomic.Int64
	h := signedRouter(&calls).Handler()
	rec := do(h, http.MethodPost, "/write", `{"name":"ada"}`, nil)
	if rec.Code != 200 || calls.Load() != 1 {
		t.Fatalf("status %d calls %d", rec.Code, calls.Load())
	}
}
