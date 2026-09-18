package bowline

import (
	"net/http"

	"github.com/bowlinedev/bowline/internal/reserved"
)

func WithContract(document []byte) HandlerOption {
	return func(h *handler) { h.contract = document }
}

func (h *handler) serveReserved(w http.ResponseWriter, req *http.Request, name string) {
	if h.reserved == nil {
		h.writeError(w, req, nil, 0, Errorf(Unimplemented, "unknown procedure %q", name))
		return
	}
	if req.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.writeError(w, req, nil, http.StatusMethodNotAllowed, Errorf(InvalidArgument, "method %s not allowed for %s; use GET", req.Method, name))
		return
	}
	body := h.reserved.Document
	if name == reserved.HealthPath {
		body = h.reserved.Health
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}
