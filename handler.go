package bowline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/bowlinedev/bowline/internal/codec"
)

type HandlerOption func(*handler)

func MaxBodySize(n int64) HandlerOption {
	return func(h *handler) { h.maxBody = n }
}

func Logger(l *slog.Logger) HandlerOption {
	return func(h *handler) { h.log = l }
}

func Production(on bool) HandlerOption {
	return func(h *handler) { h.production = on }
}

func StrictInput() HandlerOption {
	return func(h *handler) { h.strict = true }
}

type handler struct {
	routes     map[string]route
	maxBody    int64
	log        *slog.Logger
	production bool
	strict     bool
	validate   func(p *Procedure, in any) []Issue
	normalize  func(p *Procedure, out any) (any, error)
}

func (r *Router) Handler(opts ...HandlerOption) http.Handler {
	h := &handler{routes: map[string]route{}, maxBody: 1 << 20, log: slog.Default()}
	for _, opt := range opts {
		opt(h)
	}
	for _, rt := range r.routes() {
		h.routes[rt.path] = rt
	}
	return h
}

func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimSuffix(req.URL.Path, "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	rt, ok := h.routes[path]
	if !ok {
		h.writeError(w, 0, Errorf(Unimplemented, "unknown procedure %q", path))
		return
	}
	proc := rt.proc
	if !methodAllowed(proc, req.Method) {
		w.Header().Set("Allow", proc.Method())
		h.writeError(w, http.StatusMethodNotAllowed, Errorf(InvalidArgument, "method %s not allowed for %s; use %s", req.Method, rt.path, proc.Method()))
		return
	}
	raw, status, err := h.readInput(w, req)
	if err != nil {
		h.writeError(w, status, err)
		return
	}
	ptr := proc.newIn()
	if err := codec.Decode(raw, ptr, h.strict); err != nil {
		h.writeError(w, 0, Errorf(InvalidArgument, "invalid input: %v", err))
		return
	}
	in := proc.deref(ptr)
	if h.validate != nil {
		if issues := h.validate(proc, in); len(issues) > 0 {
			e := Errorf(InvalidArgument, "invalid input")
			e.Issues = issues
			h.writeError(w, 0, e)
			return
		}
	}
	call := &Call{Procedure: *proc, Request: req}
	call.Procedure.Path = rt.path
	ctx := withCall(req.Context(), call)
	out, err := h.invoke(ctx, rt, in)
	if err != nil {
		if status, _ := classify(err, h.production); status >= 500 {
			h.log.ErrorContext(ctx, "bowline: procedure failed", "procedure", rt.path, "error", err)
		}
		h.writeError(w, 0, err)
		return
	}
	if h.normalize != nil {
		out, err = h.normalize(proc, out)
		if err != nil {
			h.log.ErrorContext(ctx, "bowline: output normalization failed", "procedure", rt.path, "error", err)
			h.writeError(w, 0, Errorf(Internal, "output normalization failed: %w", err))
			return
		}
	}
	body, err := json.Marshal(out)
	if err != nil {
		h.log.ErrorContext(ctx, "bowline: output encoding failed", "procedure", rt.path, "error", err)
		h.writeError(w, 0, Errorf(Internal, "output encoding failed: %w", err))
		return
	}
	if proc.Deprecated != "" {
		w.Header().Set("Deprecation", "true")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func methodAllowed(p *Procedure, method string) bool {
	switch method {
	case http.MethodPost:
		return true
	case http.MethodGet:
		return p.Kind == KindQuery && !p.Sensitive
	}
	return false
}

func (h *handler) readInput(w http.ResponseWriter, req *http.Request) ([]byte, int, error) {
	if req.Method == http.MethodGet {
		return []byte(req.URL.Query().Get("input")), 0, nil
	}
	if ct := req.Header.Get("Content-Type"); ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			return nil, http.StatusUnsupportedMediaType, Errorf(InvalidArgument, "content type must be application/json")
		}
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, h.maxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, http.StatusRequestEntityTooLarge, Errorf(InvalidArgument, "request body exceeds %d bytes", h.maxBody)
		}
		return nil, 0, Errorf(InvalidArgument, "reading request body: %v", err)
	}
	return body, 0, nil
}

func (h *handler) invoke(ctx context.Context, rt route, in any) (out any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			h.log.ErrorContext(ctx, "bowline: procedure panicked", "procedure", rt.path, "panic", rec, "stack", string(debug.Stack()))
			err = Errorf(Internal, "panic: %v", rec)
		}
	}()
	return rt.next(ctx, in)
}

func (h *handler) writeError(w http.ResponseWriter, statusOverride int, err error) {
	status, env := classify(err, h.production)
	if statusOverride != 0 {
		status = statusOverride
	}
	body, marshalErr := json.Marshal(env)
	if marshalErr != nil {
		body = []byte(fmt.Sprintf(`{"error":{"code":"INTERNAL","message":%q}}`, "error encoding failed"))
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}
