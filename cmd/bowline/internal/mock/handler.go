package mock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/fake"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/shape"
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
	doc      *contract.Document
	routes   map[string]*contract.Procedure
	rest     []restRoute
	gen      *fake.Generator
	state    *state
	fixtures *fixtureSet
	strict   bool
	log      *slog.Logger
}

func New(doc *contract.Document, opts Options) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	gen := fake.New(doc, opts.Seed)
	h := &handler{doc: doc, routes: map[string]*contract.Procedure{}, gen: gen, state: newState(doc, gen), strict: opts.Strict, log: opts.Logger}
	for _, p := range doc.Procedures {
		h.routes[p.Path] = p
	}
	h.rest = restRoutes(doc)
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
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Issues  []shape.Issue `json:"issues,omitempty"`
}

type envelope struct {
	Error errorBody `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string, issues []shape.Issue) {
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
	var params map[string]string
	if !ok {
		matched, bound, allowed := h.matchREST(req.URL.EscapedPath(), req.Method)
		switch {
		case matched != nil:
			p, params = matched, bound
		case len(allowed) > 0:
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			writeError(w, http.StatusMethodNotAllowed, "INVALID_ARGUMENT", fmt.Sprintf("method %s not allowed for %s; use %s", req.Method, req.URL.Path, strings.Join(allowed, " or ")), nil)
			return
		default:
			writeError(w, http.StatusNotFound, "UNIMPLEMENTED", fmt.Sprintf("unknown procedure %q", path), nil)
			return
		}
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
	if p.HTTPPath != "" {
		merged, err := h.mergeREST(p, raw, params, req.URL.Query())
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid input: "+err.Error(), nil)
			return
		}
		raw = merged
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
	issues := shape.Validate(h.doc, p.Input, input)
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
	if p.HTTPPath != "" {
		return method == p.Method
	}
	switch method {
	case http.MethodPost:
		return true
	case http.MethodGet:
		return p.Method == http.MethodGet
	}
	return false
}

func readInput(w http.ResponseWriter, req *http.Request) ([]byte, int, string, string) {
	if !sendsBody(req.Method) {
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
		return nil, errors.New("expected a JSON object")
	}
	return obj, nil
}
