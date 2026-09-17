package bowline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bowlinedev/bowline/internal/codec"
	"github.com/bowlinedev/bowline/internal/sse"
)

type StreamFailure struct {
	Status int
	Body   json.RawMessage
	Err    error
}

func (f *StreamFailure) Error() string {
	return f.Err.Error()
}

func (f *StreamFailure) Unwrap() error {
	return f.Err
}

func (r *Router) Subscribe(ctx context.Context, path string, input []byte, send func(data []byte) error, opts ...HandlerOption) error {
	h := &handler{maxBody: 1 << 20, log: slog.Default()}
	for _, opt := range opts {
		opt(h)
	}
	var rt *route
	for _, candidate := range r.routes() {
		if candidate.path == path {
			rt = &candidate
			break
		}
	}
	if rt == nil {
		return failure(Errorf(Unimplemented, "unknown procedure %q", path), h, nil)
	}
	proc := rt.proc
	if proc.Kind != KindSubscription {
		return failure(Errorf(InvalidArgument, "%s is a %s, not a subscription", path, proc.Kind), h, nil)
	}
	frameCtx, ptr := proc.newFrame(ctx, Call{Procedure: &rt.procedure})
	if err := codec.Decode(input, ptr, h.strict); err != nil {
		return failure(h.invalidInput(err), h, nil)
	}
	if issues := proc.checker.Check(ptr); len(issues) > 0 {
		e := Errorf(InvalidArgument, "invalid input")
		e.Issues = make([]Issue, len(issues))
		for i, issue := range issues {
			e.Issues[i] = Issue{Path: issue.Path, Rule: issue.Rule, Message: issue.Message}
		}
		return failure(e, h, nil)
	}
	emit := func(v any) error {
		data, err := sse.EncodeMessage(frameCtx, proc.plan, v)
		if err != nil {
			return err
		}
		return send(data)
	}
	if _, err := h.invoke(frameCtx, rt, proc.attach(ptr, emit)); err != nil {
		status, _, undeclared := classify(err, h.production, proc.variants)
		if status >= 500 {
			h.log.ErrorContext(ctx, "bowline: subscription failed", "procedure", path, "error", err)
		}
		if undeclared && !h.production {
			h.log.WarnContext(ctx, "bowline: undeclared error variant", "procedure", path, "error", err)
		}
		return failure(err, h, proc)
	}
	return nil
}

func failure(err error, h *handler, proc *Procedure) error {
	var variants []variant
	if proc != nil {
		variants = proc.variants
	}
	status, env, _ := classify(err, h.production, variants)
	body, marshalErr := json.Marshal(env)
	if marshalErr != nil {
		body = fmt.Appendf(nil, `{"error":{"code":"INTERNAL","message":%q}}`, "error encoding failed")
	}
	return &StreamFailure{Status: status, Body: body, Err: err}
}
