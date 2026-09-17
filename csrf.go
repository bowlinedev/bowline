package bowline

import (
	"context"
	"net/http"

	"github.com/bowlinedev/bowline/internal/csrf"
)

type CSRFOptions struct {
	AllowedOrigins     []string
	TrustFetchMetadata bool
}

func CSRF(options CSRFOptions) HandlerOption {
	guard := csrf.New(options.AllowedOrigins, options.TrustFetchMetadata)
	return func(h *handler) { h.csrf = guard }
}

func SecurityHeaders() HandlerOption {
	return func(h *handler) { h.securityHeaders = true }
}

func (h *handler) secure(w http.ResponseWriter, method string) {
	if !h.securityHeaders {
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if method == http.MethodPost {
		w.Header().Set("Cache-Control", "no-store")
	}
}

func methodOf(ctx context.Context) string {
	if c := CallFrom(ctx); c != nil && c.Request != nil {
		return c.Request.Method
	}
	return http.MethodGet
}
