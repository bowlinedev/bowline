package conformance

import (
	"net/http"
	"strings"
	"testing"
)

func TestIdentityMount(t *testing.T) {
	Run(t, func(h http.Handler) http.Handler {
		mux := http.NewServeMux()
		mux.Handle("/api/", h)
		return mux
	})
}

func TestPrefixedMount(t *testing.T) {
	handler := http.NewServeMux()
	handler.Handle("/v1/api/", Router().Handler(Options()...))
	srv := startServer(t, handler)
	RunURL(t, srv+"/v1/api/")
}

func TestReportsExactlyTheBrokenRule(t *testing.T) {
	stripping := func(h http.Handler) http.Handler {
		mux := http.NewServeMux()
		mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(&stripDeprecation{ResponseWriter: w}, r)
		}))
		return mux
	}
	handler := stripping(Router().Handler(Options()...))
	base := startServer(t, handler) + "/api"
	client := &http.Client{}
	var failed []string
	for _, tc := range cases {
		if err := execute(client, base, tc); err != nil {
			failed = append(failed, tc.Name)
		}
	}
	if strings.Join(failed, ",") != "deprecation-header" {
		t.Fatalf("failed cases %v, want only deprecation-header", failed)
	}
}

type stripDeprecation struct {
	http.ResponseWriter
}

func (s *stripDeprecation) WriteHeader(status int) {
	s.Header().Del("Deprecation")
	s.ResponseWriter.WriteHeader(status)
}

func (s *stripDeprecation) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func TestEveryAssertionHasACase(t *testing.T) {
	names := map[string]bool{}
	for _, tc := range cases {
		names[tc.Name] = true
	}
	for _, want := range []string{"success-body", "code-not_found", "get-without-input", "post-on-query", "get-on-mutation", "get-on-sensitive", "unsupported-media-type", "malformed-json", "unknown-procedure", "trailing-slash", "validation-issues", "body-limit", "panic-is-internal", "survives-panic", "deprecation-header", "nested-path", "wire-encodings", "subscription-stream", "upload", "upload-missing-file"} {
		if !names[want] {
			t.Errorf("missing case %s", want)
		}
	}
}
