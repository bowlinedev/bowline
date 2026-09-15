package mock

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bowlinedev/bowline/contract"
)

type recorder struct {
	doc    *contract.Document
	routes map[string]*contract.Procedure
	dir    string
	proxy  *httputil.ReverseProxy
	log    *slog.Logger
	now    func() time.Time
	mu     sync.Mutex
}

func Recorder(doc *contract.Document, upstream *url.URL, dir string, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	r := &recorder{doc: doc, routes: map[string]*contract.Procedure{}, dir: dir, log: logger, now: time.Now}
	for _, p := range doc.Procedures {
		r.routes[p.Path] = p
	}
	base := strings.TrimRight(upstream.Path, "/")
	r.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			name := procedureName(pr.In.URL.Path)
			pr.Out.URL.Path = base + "/" + name
			pr.Out.URL.RawPath = ""
			pr.Out.Host = upstream.Host
		},
		ModifyResponse: r.capture,
	}
	return r
}

func procedureName(path string) string {
	path = strings.TrimSuffix(path, "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	return path
}

type inputKey struct{}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var input []byte
	if req.Method == http.MethodGet {
		input = []byte(req.URL.Query().Get("input"))
	} else if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		input = body
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}
	req = req.WithContext(withInput(req.Context(), input))
	r.proxy.ServeHTTP(w, req)
}

func (r *recorder) capture(resp *http.Response) error {
	name := procedureName(resp.Request.URL.Path)
	p, ok := r.routes[name]
	if !ok || p.Kind == "subscription" || p.Kind == "upload" {
		return nil
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	input, _ := inputFrom(resp.Request.Context())
	fx, hash, err := NewFixture(p, input, resp.StatusCode, resp.Header, body, r.now().UTC().Format(time.RFC3339))
	if err != nil {
		r.log.Warn("mock: not recording", "procedure", p.Path, "error", err)
		return nil
	}
	target := filepath.Join(r.dir, filepath.FromSlash(FixturePath(p.Path, hash)))
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	data, err := Encode(fx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return err
	}
	r.log.Info("mock: recorded", "procedure", p.Path, "file", FixturePath(p.Path, hash))
	return nil
}
