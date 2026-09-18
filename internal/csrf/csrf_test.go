package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var safeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
}

var everyMethod = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodOptions,
	http.MethodTrace,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodConnect,
	"PURGE",
	"LOCK",
	"get",
	"post",
}

func crossOrigin(method string) *http.Request {
	req := httptest.NewRequest(method, "/api/thing", nil)
	req.Host = "app.example.com"
	req.Header.Set("Origin", "https://evil.example")
	return req
}

func TestOnlySafeMethodsSkipTheCheck(t *testing.T) {
	guard := New(nil, false)
	for _, method := range everyMethod {
		t.Run(method, func(t *testing.T) {
			allowed := guard.Allows(crossOrigin(method))
			if want := safeMethods[method]; allowed != want {
				t.Fatalf("a cross-origin %s is allowed=%v, want %v", method, allowed, want)
			}
		})
	}
}

func TestOnlySafeMethodsSkipTheCheckUnderFetchMetadata(t *testing.T) {
	guard := New(nil, true)
	for _, method := range everyMethod {
		t.Run(method, func(t *testing.T) {
			req := crossOrigin(method)
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			allowed := guard.Allows(req)
			if want := safeMethods[method]; allowed != want {
				t.Fatalf("a cross-site %s is allowed=%v, want %v", method, allowed, want)
			}
		})
	}
}

func TestSameOriginPassesOnEveryMethod(t *testing.T) {
	guard := New(nil, false)
	for _, method := range everyMethod {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/thing", nil)
			req.Host = "app.example.com"
			req.Header.Set("Origin", "https://app.example.com")
			if !guard.Allows(req) {
				t.Fatalf("a same-origin %s was rejected", method)
			}
		})
	}
}

func TestLowercaseSafeMethodIsNotTreatedAsSafe(t *testing.T) {
	if New(nil, false).Allows(crossOrigin("get")) {
		t.Fatal("methods are case-sensitive; \"get\" must not be treated as GET")
	}
}
