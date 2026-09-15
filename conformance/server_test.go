package conformance

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func startServer(t *testing.T, h http.Handler) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}
