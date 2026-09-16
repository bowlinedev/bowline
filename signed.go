package bowline

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/bowlinedev/bowline/signing"
)

func Signed(provider signing.SecretProvider) HandlerOption {
	return func(h *handler) {
		h.signatures = provider
		if h.replay == nil {
			h.replay = signing.NewReplayCache(signing.DefaultReplayCacheSize)
		}
	}
}

func signingLimit(h *handler) int64 {
	limit := h.maxBody
	for _, rt := range h.routes {
		if rt.proc.MaxBody > limit {
			limit = rt.proc.MaxBody
		}
		if rt.proc.Kind != KindUpload {
			continue
		}
		upload := h.maxUpload
		if upload <= 0 {
			upload = 32 << 20
		}
		if upload > limit {
			limit = upload
		}
	}
	return limit
}

func (h *handler) verifySignature(w http.ResponseWriter, req *http.Request) bool {
	var body []byte
	if req.Method != http.MethodGet && req.Body != nil {
		read, err := io.ReadAll(http.MaxBytesReader(w, req.Body, h.signedBody))
		if err != nil {
			h.writeError(w, nil, http.StatusRequestEntityTooLarge, Errorf(InvalidArgument, "request body exceeds %d bytes", h.signedBody))
			return false
		}
		body = read
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	err := signing.Verify(req.Context(), h.signatures, req.Header.Get(signing.Header), req.Method, signedPath(req), body, time.Now(), signing.WithReplayCache(h.replay))
	if err != nil {
		h.log.WarnContext(req.Context(), "bowline: rejected an unsigned or badly signed request", "path", req.URL.Path, "error", err)
		h.writeError(w, nil, 0, Errorf(Unauthenticated, "a valid %s header is required", signing.Header))
		return false
	}
	return true
}

func signedPath(req *http.Request) string {
	if req.RequestURI != "" {
		return req.RequestURI
	}
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if req.URL.RawQuery != "" {
		return path + "?" + req.URL.RawQuery
	}
	return path
}
