package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bowlinedev/bowline/contract"
)

type route struct {
	service  string
	upstream *url.URL
	path     string
	method   string
	kind     string
	retries  int
}

type Gateway struct {
	cfg      *Config
	composed *contract.Document
	document []byte
	hash     string
	routes   map[string]route
	clients  map[string]*http.Client
	forward  map[string]bool

	mu      sync.Mutex
	probed  time.Time
	probes  map[string]error
	nowFunc func() time.Time
}

func New(cfg *Config, docs map[string]*contract.Document) (*Gateway, error) {
	if cfg == nil {
		return nil, errors.New("gateway: no configuration")
	}
	composed, diags := Compose(docs)
	if len(diags) > 0 {
		messages := make([]string, len(diags))
		for i, d := range diags {
			messages[i] = d.String()
		}
		return nil, errors.New("gateway: " + strings.Join(messages, "; "))
	}
	document, err := composed.Marshal()
	if err != nil {
		return nil, err
	}
	g := &Gateway{
		cfg:      cfg,
		composed: composed,
		document: document,
		hash:     composed.Hash,
		routes:   map[string]route{},
		clients:  map[string]*http.Client{},
		forward:  map[string]bool{},
		nowFunc:  time.Now,
	}
	for _, name := range append(append([]string(nil), cfg.ForwardHeaders...), "Content-Type", "Accept", "Idempotency-Key", "Bowline-Signature") {
		g.forward[http.CanonicalHeaderKey(name)] = true
	}
	for name, up := range cfg.Services {
		parsed, err := url.Parse(up.URL)
		if err != nil {
			return nil, err
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, errors.New("gateway: service " + name + " has an invalid url " + up.URL)
		}
		g.clients[name] = &http.Client{}
		for _, p := range docs[name].Procedures {
			g.routes[name+"."+p.Path] = route{
				service:  name,
				upstream: parsed,
				path:     p.Path,
				method:   p.Method,
				kind:     p.Kind,
				retries:  up.Retries,
			}
		}
	}
	return g, nil
}

func (g *Gateway) Contract() *contract.Document {
	return g.composed
}

func (g *Gateway) Document() []byte {
	return g.document
}

func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	prefix := "/" + strings.Trim(g.cfg.Prefix, "/")
	if prefix == "/" {
		prefix = ""
	}
	mux.Handle(prefix+"/", http.StripPrefix(prefix, http.HandlerFunc(g.serve)))
	if prefix != "" {
		mux.Handle(prefix, http.RedirectHandler(prefix+"/", http.StatusPermanentRedirect))
	}
	return mux
}

func (g *Gateway) serve(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	switch path {
	case ".bowline/contract":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(g.document)
		return
	case ".bowline/health":
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hash": g.hash})
		return
	}
	rt, ok := g.routes[path]
	if !ok {
		writeEnvelope(w, "UNIMPLEMENTED", "unknown procedure \""+path+"\"")
		return
	}
	g.proxy(w, req, rt)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	w.Write(data)
}

var envelopeStatus = map[string]int{
	"UNIMPLEMENTED":     http.StatusNotFound,
	"UNAVAILABLE":       http.StatusServiceUnavailable,
	"DEADLINE_EXCEEDED": http.StatusRequestTimeout,
	"INTERNAL":          http.StatusInternalServerError,
}

func writeEnvelope(w http.ResponseWriter, code, message string) {
	status, ok := envelopeStatus[code]
	if !ok {
		status = http.StatusInternalServerError
	}
	body, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "message": message}})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}
