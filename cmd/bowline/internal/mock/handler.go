package mock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/fake"
	"github.com/bowlinedev/bowline/contract"
)

const maxBody = 4 << 20

type Options struct {
	Seed     uint64
	Fixtures fs.FS
	Strict   bool
	Logger   *slog.Logger
}

type handler struct {
	doc       *contract.Document
	routes    map[string]*contract.Procedure
	gen       *fake.Generator
	state     *state
	validator *validator
	fixtures  *fixtureSet
	strict    bool
	log       *slog.Logger
}

func New(doc *contract.Document, opts Options) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	gen := fake.New(doc, opts.Seed)
	h := &handler{doc: doc, routes: map[string]*contract.Procedure{}, gen: gen, state: newState(doc, gen), validator: &validator{doc: doc}, strict: opts.Strict, log: opts.Logger}
	for _, p := range doc.Procedures {
		h.routes[p.Path] = p
	}
	if opts.Fixtures != nil {
		set, err := loadFixtures(opts.Fixtures)
		if err != nil {
			opts.Logger.Error("mock: loading fixtures", "error", err)
		}
		h.fixtures = set
	}
	return h
}

type errorBody struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Issues  []issue `json:"issues,omitempty"`
}

type envelope struct {
	Error errorBody `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string, issues []issue) {
	body, _ := json.Marshal(envelope{errorBody{Code: code, Message: message, Issues: issues}})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimSuffix(req.URL.Path, "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	p, ok := h.routes[path]
	if !ok {
		writeError(w, http.StatusNotFound, "UNIMPLEMENTED", fmt.Sprintf("unknown procedure %q", path), nil)
		return
	}
	if p.Kind == "subscription" || p.Kind == "upload" {
		writeError(w, http.StatusNotFound, "UNIMPLEMENTED", fmt.Sprintf("the mock server does not serve %s procedures", p.Kind), nil)
		return
	}
	if !methodAllowed(p, req.Method) {
		w.Header().Set("Allow", p.Method)
		writeError(w, http.StatusMethodNotAllowed, "INVALID_ARGUMENT", fmt.Sprintf("method %s not allowed for %s; use %s", req.Method, path, p.Method), nil)
		return
	}
	raw, status, code, message := readInput(w, req)
	if message != "" {
		writeError(w, status, code, message, nil)
		return
	}
	if h.fixtures != nil {
		if fx, ok := h.fixtures.lookup(p.Path, raw); ok {
			fx.serve(w)
			return
		}
		if h.strict {
			h.log.Warn("mock: no fixture", "procedure", p.Path)
			writeError(w, http.StatusNotFound, "UNIMPLEMENTED", fmt.Sprintf("no recorded fixture for %s with this input", p.Path), nil)
			return
		}
		h.log.Warn("mock: no fixture, generating", "procedure", p.Path)
	}
	input, err := decodeInput(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid input: "+err.Error(), nil)
		return
	}
	var issues []issue
	h.validator.check(p.Input, input, nil, nil, &issues)
	if len(issues) > 0 {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid input", issues)
		return
	}
	out := h.respond(p, input)
	body, err := json.Marshal(out)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error(), nil)
		return
	}
	if p.Deprecated != "" {
		w.Header().Set("Deprecation", "true")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (h *handler) respond(p *contract.Procedure, input map[string]any) any {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	switch p.Kind {
	case "mutation":
		if out, ok := h.state.create(p, input); ok {
			return out
		}
	case "query":
		if out, ok := h.state.get(p, input); ok {
			return out
		}
		if out, ok := h.state.list(p); ok {
			return out
		}
	}
	return h.gen.Output(p)
}

func methodAllowed(p *contract.Procedure, method string) bool {
	switch method {
	case http.MethodPost:
		return true
	case http.MethodGet:
		return p.Method == http.MethodGet
	}
	return false
}

func readInput(w http.ResponseWriter, req *http.Request) ([]byte, int, string, string) {
	if req.Method == http.MethodGet {
		return []byte(req.URL.Query().Get("input")), 0, "", ""
	}
	if ct := req.Header.Get("Content-Type"); ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			return nil, http.StatusUnsupportedMediaType, "INVALID_ARGUMENT", "content type must be application/json"
		}
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, maxBody))
	if err != nil {
		return nil, http.StatusRequestEntityTooLarge, "INVALID_ARGUMENT", fmt.Sprintf("request body exceeds %d bytes", maxBody)
	}
	return body, 0, "", ""
}

func decodeInput(raw []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return obj, nil
}
