package bowline

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

const reservedDocument = `{"bowline":"1.2","types":{},"errors":{},"procedures":[{"path":"ping","kind":"query","method":"GET","input":{"kind":"struct"},"output":{"kind":"struct"}}]}`

type pingInput struct{}

type pingOutput struct {
	OK bool `json:"ok"`
}

func ping(ctx context.Context, in pingInput) (pingOutput, error) {
	return pingOutput{OK: true}, nil
}

func reservedRouter() *Router {
	return NewRouter(Query("ping", ping))
}

func TestReservedContractEndpoint(t *testing.T) {
	h := reservedRouter().Handler(WithContract([]byte(reservedDocument)))
	rec := do(h, http.MethodGet, "/.bowline/contract", "", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != reservedDocument {
		t.Fatalf("body %s", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content type %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache control %q", cc)
	}
}

func TestReservedHealthEndpointReportsTheContractHash(t *testing.T) {
	doc, err := contract.Parse([]byte(reservedDocument))
	if err != nil {
		t.Fatal(err)
	}
	want, err := doc.ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	h := reservedRouter().Handler(WithContract([]byte(reservedDocument)))
	rec := do(h, http.MethodGet, "/.bowline/health", "", nil)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var health struct {
		OK   bool   `json:"ok"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("body %s: %v", rec.Body.String(), err)
	}
	if !health.OK || health.Hash != want {
		t.Fatalf("health %+v, want hash %s", health, want)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache control %q", cc)
	}
}

func TestReservedPathsUseTheDocumentHashWhenPresent(t *testing.T) {
	doc, err := contract.Parse([]byte(reservedDocument))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetHash(); err != nil {
		t.Fatal(err)
	}
	hashed, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	h := reservedRouter().Handler(WithContract(hashed))
	rec := do(h, http.MethodGet, "/.bowline/health", "", nil)
	var health struct {
		Hash string `json:"hash"`
	}
	json.Unmarshal(rec.Body.Bytes(), &health)
	if health.Hash != doc.Hash {
		t.Fatalf("hash %q, want %q", health.Hash, doc.Hash)
	}
}

func TestReservedPathsAreUnimplementedWithoutTheOption(t *testing.T) {
	h := reservedRouter().Handler()
	for _, path := range []string{"/.bowline/contract", "/.bowline/health"} {
		rec := do(h, http.MethodGet, path, "", nil)
		if rec.Code != 404 || errorCode(t, rec) != Unimplemented {
			t.Fatalf("%s: status %d body %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestReservedPathsWorkUnderAPrefix(t *testing.T) {
	inner := reservedRouter().Handler(WithContract([]byte(reservedDocument)))
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", inner))
	for _, path := range []string{"/api/.bowline/contract", "/api/.bowline/health"} {
		rec := do(mux, http.MethodGet, path, "", nil)
		if rec.Code != 200 {
			t.Fatalf("%s: status %d body %s", path, rec.Code, rec.Body.String())
		}
	}
	mounted := http.NewServeMux()
	mounted.Handle("/v1/api/", inner)
	rec := do(mounted, http.MethodGet, "/v1/api/.bowline/contract", "", nil)
	if rec.Code != 200 {
		t.Fatalf("mounted without stripping: status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestReservedPathsRejectOtherMethods(t *testing.T) {
	h := reservedRouter().Handler(WithContract([]byte(reservedDocument)))
	rec := do(h, http.MethodPost, "/.bowline/contract", "{}", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("allow %q", allow)
	}
	if errorCode(t, rec) != InvalidArgument {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestMalformedContractPanicsAtHandlerTime(t *testing.T) {
	option := WithContract([]byte(`{"bowline":`))
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected a panic")
		}
		message, ok := rec.(string)
		if !ok || !strings.Contains(message, "bowline: WithContract:") || !strings.Contains(message, "parse") {
			t.Fatalf("panic %v", rec)
		}
	}()
	reservedRouter().Handler(option)
}

func TestProceduresStillServeAlongsideReservedPaths(t *testing.T) {
	h := reservedRouter().Handler(WithContract([]byte(reservedDocument)))
	rec := do(h, http.MethodGet, "/ping", "", nil)
	if rec.Code != 200 || rec.Body.String() != `{"ok":true}` {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
