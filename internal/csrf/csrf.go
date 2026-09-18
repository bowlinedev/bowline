package csrf

import (
	"net/http"
	"net/url"
	"slices"
)

type Guard struct {
	allowed            []string
	trustFetchMetadata bool
}

func New(allowedOrigins []string, trustFetchMetadata bool) *Guard {
	return &Guard{allowed: slices.Clone(allowedOrigins), trustFetchMetadata: trustFetchMetadata}
}

func safe(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

func (c *Guard) Allows(req *http.Request) bool {
	if safe(req.Method) {
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
	return req.Header.Get("Sec-Fetch-Site") == ""
}

func (c *Guard) sameHostOrListed(origin, host string) bool {
	if u, err := url.Parse(origin); err == nil && u.Host != "" && u.Host == host {
		return true
	}
	return c.listed(origin)
}

func (c *Guard) listed(origin string) bool {
	return origin != "" && slices.Contains(c.allowed, origin)
}
