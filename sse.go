package bowline

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func Heartbeat(d time.Duration) HandlerOption {
	return func(h *handler) { h.heartbeat = d }
}

func acceptsEventStream(req *http.Request) bool {
	for part := range strings.SplitSeq(req.Header.Get("Accept"), ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mediaType == "text/event-stream" || mediaType == "*/*" {
			return true
		}
	}
	return false
}

func (h *handler) serveSubscription(w http.ResponseWriter, req *http.Request, rt *route, ctx context.Context, in any) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, nil, 0, Errorf(Internal, "response writer does not support streaming"))
		return
	}
	h.secure(w, http.MethodGet)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	if rt.proc.Deprecated != "" {
		w.Header().Set("Deprecation", "true")
	}
	w.WriteHeader(http.StatusOK)
	sink := &eventSink{w: w, flusher: flusher, ctx: ctx, plan: rt.proc.plan}
	defer sink.close()
	if err := sink.comment("open"); err != nil {
		return
	}
	if h.heartbeat > 0 {
		go func() {
			ticker := time.NewTicker(h.heartbeat)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if sink.comment("ping") != nil {
						return
					}
				}
			}
		}()
	}
	_, err := h.invoke(ctx, rt, rt.proc.attach(in, sink.send))
	if err != nil {
		status, env, undeclared := classify(err, h.production, rt.proc.variants)
		if status >= 500 {
			h.log.ErrorContext(ctx, "bowline: subscription failed", "procedure", rt.path, "error", err)
		}
		if undeclared && !h.production {
			h.log.WarnContext(ctx, "bowline: undeclared error variant", "procedure", rt.path, "error", err)
		}
		body, marshalErr := json.Marshal(env)
		if marshalErr != nil {
			body = []byte(`{"error":{"code":"INTERNAL","message":"error encoding failed"}}`)
		}
		sink.write("error", body)
		return
	}
	sink.write("done", []byte("{}"))
}
