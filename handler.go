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
	"time"

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
	routes     map[string]*route
	maxBody    int64
	log        *slog.Logger
	production bool
	strict     bool
	heartbeat  time.Duration
	maxUpload  int64

	idempotency    IdempotencyStore
	idempotencyTTL time.Duration
	requireKey     bool
}

func (r *Router) Handler(opts ...HandlerOption) http.Handler {
	h := &handler{routes: map[string]*route{}, maxBody: 1 << 20, log: slog.Default()}
	for _, opt := range opts {
		opt(h)
	}
	for _, rt := range r.routes() {
		h.routes[rt.path] = &rt
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
		h.writeError(w, nil, 0, Errorf(Unimplemented, "unknown procedure %q", path))
		return
	}
	proc := rt.proc
	if proc.Kind == KindSubscription && !acceptsEventStream(req) {
		h.writeError(w, nil, 0, Errorf(InvalidArgument, "subscriptions are served as text/event-stream; send Accept: text/event-stream"))
		return
	}
	if !methodAllowed(proc, req.Method) {
		w.Header().Set("Allow", proc.Method())
		h.writeError(w, nil, http.StatusMethodNotAllowed, Errorf(InvalidArgument, "method %s not allowed for %s; use %s", req.Method, rt.path, proc.Method()))
		return
	}
	if proc.Kind == KindMutation && proc.Idempotent && h.idempotency != nil {
		h.serveIdempotent(w, req, rt)
		return
	}
	h.execute(w, req, rt)
}

func (h *handler) execute(w http.ResponseWriter, req *http.Request, rt *route) {
	proc := rt.proc
	if proc.Kind == KindUpload {
		h.serveUpload(w, req, rt)
		return
	}
	raw, status, err := h.readInput(w, req)
	if err != nil {
		h.writeError(w, nil, status, err)
		return
	}
	ctx, ptr := proc.newFrame(req.Context(), Call{Procedure: &rt.procedure, Request: req})
	if err := codec.Decode(raw, ptr, h.strict); err != nil {
		h.writeError(w, nil, 0, Errorf(InvalidArgument, "invalid input: %v", err))
		return
	}
	in := ptr
	if issues := proc.checker.Check(in); len(issues) > 0 {
		e := Errorf(InvalidArgument, "invalid input")
		e.Issues = make([]Issue, len(issues))
		for i, issue := range issues {
			e.Issues[i] = Issue{Path: issue.Path, Rule: issue.Rule, Message: issue.Message}
		}
		h.writeError(w, nil, 0, e)
		return
	}
	if proc.Kind == KindSubscription {
		h.serveSubscription(w, req, rt, ctx, in)
		return
	}
	out, err := h.invoke(ctx, rt, in)
	if err != nil {
		status, _, undeclared := classify(err, h.production, proc.variants)
		if status >= 500 {
			h.log.ErrorContext(ctx, "bowline: procedure failed", "procedure", rt.path, "error", err)
		}
		if undeclared && !h.production {
			h.log.WarnContext(ctx, "bowline: undeclared error variant", "procedure", rt.path, "error", err)
		}
		h.writeError(w, proc, 0, err)
		return
	}
	h.writeOutput(w, ctx, rt, out)
}

func (h *handler) writeOutput(w http.ResponseWriter, ctx context.Context, rt *route, out any) {
	out, err := rt.proc.plan.Normalize(out)
	if err != nil {
		h.log.ErrorContext(ctx, "bowline: output normalization failed", "procedure", rt.path, "error", err)
		h.writeError(w, nil, 0, Errorf(Internal, "output normalization failed: %w", err))
		return
	}
	body, err := json.Marshal(out)
	if err != nil {
		h.log.ErrorContext(ctx, "bowline: output encoding failed", "procedure", rt.path, "error", err)
		h.writeError(w, nil, 0, Errorf(Internal, "output encoding failed: %w", err))
		return
	}
	if rt.proc.Deprecated != "" {
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
		return (p.Kind == KindQuery || p.Kind == KindSubscription) && !p.Sensitive
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

func (h *handler) invoke(ctx context.Context, rt *route, in any) (out any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			h.log.ErrorContext(ctx, "bowline: procedure panicked", "procedure", rt.path, "panic", rec, "stack", string(debug.Stack()))
			err = Errorf(Internal, "panic: %v", rec)
		}
	}()
	return rt.next(ctx, in)
}

func (h *handler) writeError(w http.ResponseWriter, proc *Procedure, statusOverride int, err error) {
	var variants []variant
	if proc != nil {
		variants = proc.variants
	}
	status, env, _ := classify(err, h.production, variants)
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
