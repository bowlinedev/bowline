package playground

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var copiedBack = []string{"Content-Type", "Deprecation", "Sunset", "Allow", "Idempotent-Replayed"}

var errUpstreamShape = errors.New("upstream must be an absolute URL or a path starting with /")

func (h *handler) proxy(w http.ResponseWriter, req *http.Request, procedure string) {
	if h.upstream == "" {
		writeUnavailable(w, "no upstream configured for the playground proxy")
		return
	}
	if procedure == "" {
		writeUnavailable(w, "proxy requests need a procedure path")
		return
	}
	target, err := h.resolve(req)
	if err != nil {
		writeUnavailable(w, err.Error())
		return
	}
	target.Path = strings.TrimSuffix(target.Path, "/") + "/" + procedure
	target.RawQuery = ""
	if input := req.URL.Query().Get("input"); input != "" {
		target.RawQuery = url.Values{"input": {input}}.Encode()
	}
	out, err := http.NewRequestWithContext(req.Context(), req.Method, target.String(), req.Body)
	if err != nil {
		writeUnavailable(w, err.Error())
		return
	}
	out.ContentLength = req.ContentLength
	for name, values := range req.Header {
		if h.allowlist[name] || strings.HasPrefix(name, "X-") {
			out.Header[name] = append([]string(nil), values...)
		}
	}
	resp, err := h.client.Do(out)
	if err != nil {
		writeUnavailable(w, "upstream unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()
	for _, name := range copiedBack {
		if v := resp.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *handler) resolve(req *http.Request) (*url.URL, error) {
	if strings.HasPrefix(h.upstream, "/") {
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		if forwarded := req.Header.Get("X-Forwarded-Proto"); forwarded != "" {
			scheme = forwarded
		}
		return url.Parse(scheme + "://" + req.Host + h.upstream)
	}
	u, err := url.Parse(h.upstream)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, errUpstreamShape
	}
	return u, nil
}

func writeUnavailable(w http.ResponseWriter, message string) {
	body, _ := json.Marshal(map[string]any{"error": map[string]string{"code": "UNAVAILABLE", "message": message}})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	w.Write(body)
}
