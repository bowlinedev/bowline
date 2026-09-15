package bowline

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bowlinedev/bowline/internal/idempotency"
)

type IdempotencyState int

const (
	IdempotencyNew IdempotencyState = iota
	IdempotencyInFlight
	IdempotencyStored
)

type IdempotencyStore interface {
	Begin(ctx context.Context, key string) (state IdempotencyState, status int, body []byte, err error)
	Complete(ctx context.Context, key string, status int, body []byte, ttl time.Duration) error
	Abort(ctx context.Context, key string) error
}

func Idempotent() ProcOption {
	return func(p *Procedure) {
		if p.Kind != KindMutation {
			panic(fmt.Sprintf("bowline: %s %q: Idempotent applies to mutations only", p.Kind, p.Name))
		}
		p.Idempotent = true
	}
}

func Idempotency(store IdempotencyStore, ttl time.Duration) HandlerOption {
	return func(h *handler) {
		h.idempotency = store
		h.idempotencyTTL = ttl
	}
}

func RequireIdempotencyKey() HandlerOption {
	return func(h *handler) { h.requireKey = true }
}

type scopeKey struct{}

func WithIdempotencyScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func MemoryIdempotencyStore() IdempotencyStore {
	return memoryStore{inner: idempotency.NewMemory(nil)}
}

type memoryStore struct {
	inner *idempotency.Memory
}

func (m memoryStore) Begin(ctx context.Context, key string) (IdempotencyState, int, []byte, error) {
	state, status, body, err := m.inner.Begin(ctx, key)
	return IdempotencyState(state), status, body, err
}

func (m memoryStore) Complete(ctx context.Context, key string, status int, body []byte, ttl time.Duration) error {
	return m.inner.Complete(ctx, key, status, body, ttl)
}

func (m memoryStore) Abort(ctx context.Context, key string) error {
	return m.inner.Abort(ctx, key)
}

type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(p []byte) (int, error) {
	r.body.Write(p)
	return r.ResponseWriter.Write(p)
}

func idempotencyKey(ctx context.Context, path, header string) string {
	scope, _ := ctx.Value(scopeKey{}).(string)
	return scope + "\x00" + path + "\x00" + header
}

func (h *handler) serveIdempotent(w http.ResponseWriter, req *http.Request, rt *route) {
	header := req.Header.Get("Idempotency-Key")
	if header == "" {
		if h.requireKey {
			h.writeError(w, nil, 0, Errorf(InvalidArgument, "the Idempotency-Key header is required for %s", rt.path))
			return
		}
		h.execute(w, req, rt)
		return
	}
	ctx := req.Context()
	key := idempotencyKey(ctx, rt.path, header)
	state, status, body, err := h.idempotency.Begin(ctx, key)
	if err != nil {
		h.log.ErrorContext(ctx, "bowline: idempotency store failed", "procedure", rt.path, "error", err)
		h.writeError(w, nil, 0, Errorf(Unavailable, "idempotency store unavailable"))
		return
	}
	switch state {
	case IdempotencyStored:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(status)
		w.Write(body)
		return
	case IdempotencyInFlight:
		h.writeError(w, nil, 0, Errorf(Aborted, "a request with this idempotency key is still in flight"))
		return
	}
	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	completed := false
	defer func() {
		if completed {
			return
		}
		h.idempotency.Abort(ctx, key)
	}()
	h.execute(rec, req, rt)
	if rec.status >= 500 {
		return
	}
	ttl := h.idempotencyTTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if err := h.idempotency.Complete(ctx, key, rec.status, rec.body.Bytes(), ttl); err != nil {
		h.log.ErrorContext(ctx, "bowline: idempotency store failed", "procedure", rt.path, "error", err)
		return
	}
	completed = true
}
