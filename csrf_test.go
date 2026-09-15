package bowline

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func csrfHandler(opts CSRFOptions, extra ...HandlerOption) http.Handler {
	options := append([]HandlerOption{CSRF(opts), Logger(discardLogger())}, extra...)
	return NewRouter(
		Query("get", getUser),
		Mutation("create", createUser),
		Upload("attach", attach),
	).Handler(options...)
}

func post(h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "app.example.com"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCSRFRejectsCrossOriginMutation(t *testing.T) {
	h := csrfHandler(CSRFOptions{})
	for _, origin := range []string{"https://evil.example", "http://app.example.com.evil.test", "null"} {
		t.Run(origin, func(t *testing.T) {
			rec := post(h, "/api/create", map[string]string{"Origin": origin})
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status %d, want 403, body %s", rec.Code, rec.Body.String())
			}
			got := envelopeOf(t, rec)
			if got.Code != PermissionDenied {
				t.Fatalf("code %q, want %q", got.Code, PermissionDenied)
			}
			if got.Message != "cross-origin request rejected" {
				t.Fatalf("message %q", got.Message)
			}
			if strings.Contains(rec.Body.String(), origin) {
				t.Fatalf("the rejection echoed the origin: %s", rec.Body.String())
			}
		})
	}
}

func TestCSRFAllowsSameOriginAndListedOrigins(t *testing.T) {
	h := csrfHandler(CSRFOptions{AllowedOrigins: []string{"https://admin.example.com"}})
	cases := map[string]map[string]string{
		"same host over https": {"Origin": "https://app.example.com"},
		"same host over http":  {"Origin": "http://app.example.com"},
		"listed origin":        {"Origin": "https://admin.example.com"},
		"same host referer":    {"Referer": "https://app.example.com/invoices"},
		"listed referer":       {"Referer": "https://admin.example.com/panel"},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			rec := post(h, "/api/create", headers)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, want 200, body %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCSRFIgnoresQueries(t *testing.T) {
	h := csrfHandler(CSRFOptions{})
	rec := do(h, http.MethodGet, "/api/get?input=%7B%22id%22%3A1%7D", "", map[string]string{"Origin": "https://evil.example"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200, body %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFFetchMetadataPrecedence(t *testing.T) {
	trusting := csrfHandler(CSRFOptions{TrustFetchMetadata: true, AllowedOrigins: []string{"https://admin.example.com"}})
	cases := []struct {
		name    string
		headers map[string]string
		status  int
	}{
		{"same-origin wins over a foreign origin", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "https://evil.example"}, 200},
		{"none passes", map[string]string{"Sec-Fetch-Site": "none"}, 200},
		{"cross-site loses to a same-host origin", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://app.example.com"}, 403},
		{"same-site loses to a same-host origin", map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://app.example.com"}, 403},
		{"cross-site with a listed origin passes", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://admin.example.com"}, 200},
		{"unknown value falls through to the origin check", map[string]string{"Sec-Fetch-Site": "sideways", "Origin": "https://app.example.com"}, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := post(trusting, "/api/create", tc.headers)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d, body %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestCSRFIgnoresFetchMetadataWhenNotTrusted(t *testing.T) {
	h := csrfHandler(CSRFOptions{})
	rec := post(h, "/api/create", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "https://evil.example"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403, body %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFRejectsMissingHeaders(t *testing.T) {
	h := csrfHandler(CSRFOptions{})
	rec := post(h, "/api/create", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403, body %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFRunsBeforeTheBodyIsRead(t *testing.T) {
	var read int64
	h := NewRouter(Mutation("create", createUser)).Handler(CSRF(CSRFOptions{}), Logger(discardLogger()))
	req := httptest.NewRequest(http.MethodPost, "/api/create", &countingReader{n: &read})
	req.Header.Set("Content-Type", "application/json")
	req.Host = "app.example.com"
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if read != 0 {
		t.Fatalf("the handler read %d bytes of a rejected request", read)
	}
}

type countingReader struct{ n *int64 }

func (r *countingReader) Read(p []byte) (int, error) {
	*r.n += int64(len(p))
	p[0] = '{'
	return 1, nil
}

func TestCSRFProtectsUploads(t *testing.T) {
	h := csrfHandler(CSRFOptions{})
	ct, body := multipartBody(t, [][3]string{{"input", "", `{"invoiceId":1}`}, {"file", "a.txt", "hi"}})
	req := httptest.NewRequest(http.MethodPost, "/api/attach", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Origin", "https://evil.example")
	req.Host = "app.example.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403, body %s", rec.Code, rec.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := NewRouter(Query("get", getUser), Mutation("create", createUser)).Handler(SecurityHeaders(), Logger(discardLogger()))
	t.Run("mutation", func(t *testing.T) {
		rec := do(h, http.MethodPost, "/api/create", `{"id":1}`, nil)
		assertSecurityHeaders(t, rec, true)
	})
	t.Run("error", func(t *testing.T) {
		rec := do(h, http.MethodGet, "/api/missing", "", nil)
		assertSecurityHeaders(t, rec, true)
	})
	t.Run("query", func(t *testing.T) {
		rec := do(h, http.MethodGet, "/api/get?input=%7B%22id%22%3A1%7D", "", nil)
		assertSecurityHeaders(t, rec, false)
	})
}

func assertSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder, wantNoStore bool) {
	t.Helper()
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options %q, want nosniff", got)
	}
	got := rec.Header().Get("Cache-Control")
	if wantNoStore && got != "no-store" {
		t.Fatalf("Cache-Control %q, want no-store", got)
	}
	if !wantNoStore && got != "" {
		t.Fatalf("Cache-Control %q, want none on a successful query", got)
	}
}

func TestSecurityHeadersAreAbsentWithoutTheOption(t *testing.T) {
	rec := do(testHandler(), http.MethodPost, "/api/users.create", `{"id":1}`, nil)
	if got := rec.Header().Get("X-Content-Type-Options"); got != "" {
		t.Fatalf("X-Content-Type-Options %q, want none", got)
	}
}
