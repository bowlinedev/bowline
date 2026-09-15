package gateway

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var hopByHop = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

var passThroughResponse = []string{"Content-Type", "Deprecation", "Sunset", "Cache-Control", "ETag"}

var retryBackoff = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond}

func shouldRetry(method, kind string, attempt, retries, status int, err error) bool {
	if attempt >= retries {
		return false
	}
	if method != http.MethodGet || kind != "query" {
		return false
	}
	if err != nil {
		return true
	}
	return status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func (g *Gateway) proxy(w http.ResponseWriter, req *http.Request, rt route) {
	if rt.kind == "subscription" {
		g.stream(w, req, rt)
		return
	}
	streaming := rt.kind == "upload"
	ctx := req.Context()
	var cancel context.CancelFunc
	if !streaming && g.cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, g.cfg.Timeout)
		defer cancel()
	}

	var body []byte
	if req.Body != nil && !streaming {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			writeEnvelope(w, "INTERNAL", "reading the request body: "+err.Error())
			return
		}
		body = data
	}

	retries := rt.retries
	if streaming {
		retries = 0
	}
	for attempt := 0; ; attempt++ {
		var reader io.Reader
		switch {
		case streaming:
			reader = req.Body
		case body != nil:
			reader = strings.NewReader(string(body))
		}
		outbound, err := g.request(ctx, req, rt, reader)
		if err != nil {
			writeEnvelope(w, "INTERNAL", err.Error())
			return
		}
		resp, err := g.clients[rt.service].Do(outbound)
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		if shouldRetry(rt.method, rt.kind, attempt, retries, status, err) {
			if resp != nil {
				resp.Body.Close()
			}
			select {
			case <-time.After(retryBackoff[min(attempt, len(retryBackoff)-1)]):
				continue
			case <-ctx.Done():
				g.writeContextError(w, ctx, rt)
				return
			}
		}
		if err != nil {
			g.writeContextError(w, ctx, rt)
			return
		}
		defer resp.Body.Close()
		copyResponseHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}
}

func (g *Gateway) writeContextError(w http.ResponseWriter, ctx context.Context, rt route) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		writeEnvelope(w, "DEADLINE_EXCEEDED", "service \""+rt.service+"\" did not answer in time")
		return
	}
	writeEnvelope(w, "UNAVAILABLE", "service \""+rt.service+"\" is unavailable")
}

func (g *Gateway) request(ctx context.Context, req *http.Request, rt route, body io.Reader) (*http.Request, error) {
	target := *rt.upstream
	target.Path = strings.TrimRight(rt.upstream.Path, "/") + "/" + rt.path
	target.RawQuery = req.URL.RawQuery
	outbound, err := http.NewRequestWithContext(ctx, req.Method, target.String(), body)
	if err != nil {
		return nil, err
	}
	for name, values := range req.Header {
		canonical := http.CanonicalHeaderKey(name)
		if hopByHop[canonical] || !g.forward[canonical] {
			continue
		}
		for _, v := range values {
			outbound.Header.Add(canonical, v)
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	if prior := req.Header.Get("X-Forwarded-For"); prior != "" && host != "" {
		outbound.Header.Set("X-Forwarded-For", prior+", "+host)
	} else if host != "" {
		outbound.Header.Set("X-Forwarded-For", host)
	}
	outbound.Header.Set("X-Forwarded-Host", req.Host)
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if forwarded := req.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	outbound.Header.Set("X-Forwarded-Proto", scheme)
	return outbound, nil
}

func copyResponseHeaders(dst, src http.Header) {
	for _, name := range passThroughResponse {
		if v := src.Get(name); v != "" {
			dst.Set(name, v)
		}
	}
}
