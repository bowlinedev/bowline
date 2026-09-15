package playground

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
)

//go:embed all:ui/dist
var bundle embed.FS

const titleToken = "__BOWLINE_TITLE__"

type Option func(*handler)

type handler struct {
	contract  func() []byte
	upstream  string
	title     string
	allowlist map[string]bool
	client    *http.Client
	assets    fs.FS
	once      sync.Once
	index     []byte
	files     http.Handler
}

func WithUpstream(url string) Option {
	return func(h *handler) { h.upstream = url }
}

func WithTitle(title string) Option {
	return func(h *handler) { h.title = title }
}

func WithHeaderAllowlist(names ...string) Option {
	return func(h *handler) {
		for _, name := range names {
			h.allowlist[http.CanonicalHeaderKey(name)] = true
		}
	}
}

func New(contract []byte, opts ...Option) http.Handler {
	return NewDynamic(func() []byte { return contract }, opts...)
}

func NewDynamic(contract func() []byte, opts ...Option) http.Handler {
	h := &handler{contract: contract, title: "Bowline playground", allowlist: map[string]bool{}, client: http.DefaultClient}
	for _, name := range []string{"Authorization", "Content-Type", "Accept", "Idempotency-Key"} {
		h.allowlist[name] = true
	}
	for _, opt := range opts {
		opt(h)
	}
	assets, err := fs.Sub(bundle, "ui/dist")
	if err != nil {
		assets = bundle
	}
	h.assets = assets
	h.files = http.FileServerFS(assets)
	return h
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p := path.Clean("/" + req.URL.Path)
	switch {
	case p == "/" || p == "/index.html":
		h.serveIndex(w, req)
	case p == "/contract.json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(h.contract())
	case p == "/proxy" || strings.HasPrefix(p, "/proxy/"):
		h.proxy(w, req, strings.TrimPrefix(strings.TrimPrefix(p, "/proxy"), "/"))
	default:
		if _, err := fs.Stat(h.assets, strings.TrimPrefix(p, "/")); err != nil {
			h.serveIndex(w, req)
			return
		}
		req2 := req.Clone(req.Context())
		req2.URL.Path = p
		h.files.ServeHTTP(w, req2)
	}
}

func (h *handler) serveIndex(w http.ResponseWriter, req *http.Request) {
	h.once.Do(func() {
		data, err := fs.ReadFile(h.assets, "index.html")
		if err != nil {
			data, _ = fs.ReadFile(h.assets, "placeholder.html")
		}
		h.index = bytes.ReplaceAll(data, []byte(titleToken), []byte(h.title))
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if req.Method == http.MethodHead {
		return
	}
	w.Write(h.index)
}
