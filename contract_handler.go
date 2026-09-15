package bowline

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

const (
	contractPath = ".bowline/contract"
	healthPath   = ".bowline/health"
)

func WithContract(document []byte) HandlerOption {
	return func(h *handler) { h.contract = document }
}

type reserved struct {
	document []byte
	health   []byte
}

func newReserved(document []byte) (*reserved, error) {
	doc, err := contract.Parse(document)
	if err != nil {
		return nil, err
	}
	hash := doc.Hash
	if hash == "" {
		computed, err := doc.ComputeHash()
		if err != nil {
			return nil, err
		}
		hash = computed
	}
	health, err := json.Marshal(struct {
		OK   bool   `json:"ok"`
		Hash string `json:"hash"`
	}{OK: true, Hash: hash})
	if err != nil {
		return nil, err
	}
	return &reserved{document: document, health: health}, nil
}

func reservedPath(urlPath string) string {
	trimmed := strings.TrimSuffix(urlPath, "/")
	for _, name := range []string{contractPath, healthPath} {
		if trimmed == name || trimmed == "/"+name || strings.HasSuffix(trimmed, "/"+name) {
			return name
		}
	}
	return ""
}

func (h *handler) serveReserved(w http.ResponseWriter, req *http.Request, name string) {
	if h.reserved == nil {
		h.writeError(w, nil, 0, Errorf(Unimplemented, "unknown procedure %q", name))
		return
	}
	if req.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		h.writeError(w, nil, http.StatusMethodNotAllowed, Errorf(InvalidArgument, "method %s not allowed for %s; use GET", req.Method, name))
		return
	}
	body := h.reserved.document
	if name == healthPath {
		body = h.reserved.health
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func mustReserved(document []byte) *reserved {
	r, err := newReserved(document)
	if err != nil {
		panic(fmt.Sprintf("bowline: WithContract: %v", err))
	}
	return r
}
