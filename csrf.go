package bowline

import (
	"context"
	"net/http"
	"net/url"
	"slices"
)

type CSRFOptions struct {
	AllowedOrigins     []string
	TrustFetchMetadata bool
}

func CSRF(options CSRFOptions) HandlerOption {
	guard := &csrf{
		allowed:            slices.Clone(options.AllowedOrigins),
		trustFetchMetadata: options.TrustFetchMetadata,
	}
	return func(h *handler) { h.csrf = guard }
}

func SecurityHeaders() HandlerOption {
	return func(h *handler) { h.securityHeaders = true }
}

type csrf struct {
	allowed            []string
	trustFetchMetadata bool
}

func (c *csrf) allows(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return true
	}
	if c.trustFetchMetadata {
		switch req.Header.Get("Sec-Fetch-Site") {
		case "same-origin", "none":
			return true
		case "same-site", "cross-site":
			return c.listed(req.Header.Get("Origin"))
		}
	}
	if origin := req.Header.Get("Origin"); origin != "" {
		return c.sameHostOrListed(origin, req.Host)
	}
	if referer := req.Header.Get("Referer"); referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.Host == "" {
			return false
		}
		return c.sameHostOrListed(u.Scheme+"://"+u.Host, req.Host)
	}
	return false
}

func (c *csrf) sameHostOrListed(origin, host string) bool {
	if u, err := url.Parse(origin); err == nil && u.Host != "" && u.Host == host {
		return true
	}
	return c.listed(origin)
}

func (c *csrf) listed(origin string) bool {
	return origin != "" && slices.Contains(c.allowed, origin)
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
