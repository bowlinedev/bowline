package bowline

import (
	"context"
	"net/http"
	"strings"
)

const ProblemMediaType = "application/problem+json"

type problemOptions struct {
	enabled  bool
	typeBase string
}

type ProblemOption func(*problemOptions)

func ProblemTypeBase(base string) ProblemOption {
	return func(p *problemOptions) { p.typeBase = base }
}

func ProblemDetails(opts ...ProblemOption) HandlerOption {
	return func(h *handler) {
		h.problem.enabled = true
		for _, opt := range opts {
			opt(&h.problem)
		}
	}
}

func requestFrom(ctx context.Context) *http.Request {
	if call := CallFrom(ctx); call != nil {
		return call.Request
	}
	return nil
}

func acceptsProblem(req *http.Request) bool {
	if req == nil {
		return false
	}
	for value := range strings.SplitSeq(req.Header.Get("Accept"), ",") {
		media, _, _ := strings.Cut(strings.TrimSpace(value), ";")
		if strings.EqualFold(strings.TrimSpace(media), ProblemMediaType) {
			return true
		}
	}
	return false
}

func problemTitle(code Code) string {
	var b strings.Builder
	b.Grow(len(code))
	upper := true
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c == '_' {
			b.WriteByte(' ')
			upper = true
			continue
		}
		if upper {
			b.WriteByte(c)
			upper = false
			continue
		}
		b.WriteByte(c | 0x20)
	}
	return b.String()
}

func (h *handler) problemBody(status int, env wireEnvelope, req *http.Request) map[string]any {
	kind := "about:blank"
	if h.problem.typeBase != "" {
		kind = h.problem.typeBase + string(env.Error.Code)
	}
	out := map[string]any{
		"type":   kind,
		"title":  problemTitle(env.Error.Code),
		"status": status,
		"code":   string(env.Error.Code),
	}
	if env.Error.Message != "" {
		out["detail"] = env.Error.Message
	}
	if req != nil && req.URL != nil {
		out["instance"] = req.URL.EscapedPath()
	}
	if env.Error.Type != "" {
		out["errorType"] = env.Error.Type
	}
	if env.Error.Details != nil {
		out["details"] = env.Error.Details
	}
	if len(env.Error.Issues) > 0 {
		out["issues"] = env.Error.Issues
	}
	return out
}
