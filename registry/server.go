package registry

import (
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/contract"
)

const maxBody = 8 << 20

type Options struct {
	Tokens   []string
	Logger   *slog.Logger
	UI       fs.FS
	Now      func() time.Time
	Composer Composer
}

type Server struct {
	store    Store
	tokens   []string
	log      *slog.Logger
	ui       fs.FS
	now      func() time.Time
	composer Composer
}

func NewServer(store Store, opts Options) *Server {
	s := &Server{store: store, tokens: opts.Tokens, log: opts.Logger, ui: opts.UI, now: opts.Now, composer: opts.Composer}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

type VersionSummary struct {
	Hash        string    `json:"hash"`
	PublishedAt time.Time `json:"publishedAt"`
	Ref         string    `json:"ref,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

type ServiceDetail struct {
	Service
	Latest *VersionSummary `json:"latest,omitempty"`
}

type PublishResult struct {
	Hash    string        `json:"hash"`
	Created bool          `json:"created"`
	Impact  *ImpactReport `json:"impact,omitempty"`
}

type Node struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func summarize(v Version) VersionSummary {
	return VersionSummary{Hash: v.Hash, PublishedAt: v.PublishedAt, Ref: v.Ref, Tags: v.Tags}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /v1/services", s.listServices)
	mux.HandleFunc("PUT /v1/services/{name}", s.write(s.putService))
	mux.HandleFunc("GET /v1/services/{name}", s.getService)
	mux.HandleFunc("POST /v1/services/{name}/versions", s.write(s.publishVersion))
	mux.HandleFunc("GET /v1/services/{name}/versions", s.listVersions)
	mux.HandleFunc("GET /v1/services/{name}/versions/{hash}", s.getVersion)
	mux.HandleFunc("GET /v1/services/{name}/latest", s.getLatest)
	mux.HandleFunc("PUT /v1/services/{name}/tags/{tag}", s.write(s.putTag))
	mux.HandleFunc("POST /v1/services/{name}/consumers", s.write(s.putConsumer))
	mux.HandleFunc("GET /v1/services/{name}/consumers", s.listConsumers)
	mux.HandleFunc("POST /v1/services/{name}/impact", s.impact)
	mux.HandleFunc("POST /v1/gateways/{name}/compositions", s.write(s.putComposition))
	mux.HandleFunc("GET /v1/graph", s.graph)
	mux.Handle("GET /", s.index())
	return s.logging(mux)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.DebugContext(r.Context(), "registry request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (s *Server) index() http.Handler {
	if s.ui != nil {
		return s.assets()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			fail(w, bowline.Unimplemented, "unknown path "+r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("bowline registry\n\nThe API is under /v1; start at /v1/services.\n"))
	})
}

func (s *Server) assets() http.Handler {
	files := http.FileServerFS(s.ui)
	if _, err := fs.Stat(s.ui, "index.html"); err == nil {
		return files
	}
	placeholder, err := fs.ReadFile(s.ui, "placeholder.html")
	if err != nil {
		return files
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(placeholder)
	})
}

func (s *Server) write(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			fail(w, bowline.Unauthenticated, "a bearer token from the registry's configured token list is required")
			return
		}
		next(w, r)
	}
}

func writeJSONResponse(w http.ResponseWriter, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail(w, bowline.Internal, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(append(data, '\n'))
}

func writeRaw(w http.ResponseWriter, raw []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}

func fail(w http.ResponseWriter, code bowline.Code, message string) {
	body := map[string]any{"error": map[string]any{"code": string(code), "message": message}}
	data, _ := json.Marshal(body)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code.HTTPStatus())
	w.Write(append(data, '\n'))
}

func failStore(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		fail(w, bowline.NotFound, err.Error())
		return
	}
	fail(w, bowline.Internal, err.Error())
}

func readBody(w http.ResponseWriter, r *http.Request, v any) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		fail(w, bowline.InvalidArgument, "reading the request body: "+err.Error())
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		fail(w, bowline.InvalidArgument, "the request body is not valid JSON: "+err.Error())
		return false
	}
	return true
}

func (s *Server) listServices(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Services(r.Context())
	if err != nil {
		failStore(w, err)
		return
	}
	if list == nil {
		list = []Service{}
	}
	writeJSONResponse(w, list)
}

func (s *Server) putService(w http.ResponseWriter, r *http.Request) {
	var record Service
	if !readBody(w, r, &record) {
		return
	}
	record.Name = r.PathValue("name")
	if record.CreatedAt.IsZero() {
		record.CreatedAt = s.now().UTC()
	}
	if err := s.store.PutService(r.Context(), record); err != nil {
		fail(w, bowline.InvalidArgument, err.Error())
		return
	}
	writeJSONResponse(w, record)
}

func (s *Server) getService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	record, err := s.store.Service(r.Context(), name)
	if err != nil {
		failStore(w, err)
		return
	}
	detail := ServiceDetail{Service: record}
	versions, err := s.store.Versions(r.Context(), name)
	if err != nil {
		failStore(w, err)
		return
	}
	if len(versions) > 0 {
		latest := summarize(versions[0])
		detail.Latest = &latest
	}
	writeJSONResponse(w, detail)
}

func (s *Server) ensureService(r *http.Request, name string) error {
	if _, err := s.store.Service(r.Context(), name); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	return s.store.PutService(r.Context(), Service{Name: name, CreatedAt: s.now().UTC()})
}

func (s *Server) publishVersion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		fail(w, bowline.InvalidArgument, "reading the request body: "+err.Error())
		return
	}
	doc, err := contract.Parse(data)
	if err != nil {
		fail(w, bowline.InvalidArgument, "the published contract is not valid: "+err.Error())
		return
	}
	hash, err := doc.ComputeHash()
	if err != nil {
		fail(w, bowline.Internal, err.Error())
		return
	}
	report, err := ImpactWith(r.Context(), s.store, name, doc, ImpactOptions{Composer: s.composer})
	if err != nil {
		failStore(w, err)
		return
	}
	if err := s.ensureService(r, name); err != nil {
		fail(w, bowline.InvalidArgument, err.Error())
		return
	}
	version := Version{
		Service:     name,
		Hash:        hash,
		PublishedAt: s.now().UTC(),
		Ref:         r.Header.Get("Bowline-Ref"),
		Contract:    data,
	}
	if tag := r.Header.Get("Bowline-Tag"); tag != "" {
		version.Tags = []string{tag}
	}
	created, err := s.store.PutVersion(r.Context(), version)
	if err != nil {
		failStore(w, err)
		return
	}
	writeJSONResponse(w, PublishResult{Hash: hash, Created: created, Impact: report})
}

func (s *Server) impact(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		fail(w, bowline.InvalidArgument, "reading the request body: "+err.Error())
		return
	}
	doc, err := contract.Parse(data)
	if err != nil {
		fail(w, bowline.InvalidArgument, "the candidate contract is not valid: "+err.Error())
		return
	}
	strict := r.URL.Query().Get("strict") == "true"
	report, err := ImpactWith(r.Context(), s.store, r.PathValue("name"), doc, ImpactOptions{Strict: strict, Composer: s.composer})
	if err != nil {
		failStore(w, err)
		return
	}
	writeJSONResponse(w, report)
}

func (s *Server) listVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.store.Versions(r.Context(), r.PathValue("name"))
	if err != nil {
		failStore(w, err)
		return
	}
	out := make([]VersionSummary, 0, len(versions))
	for _, v := range versions {
		out = append(out, summarize(v))
	}
	writeJSONResponse(w, out)
}

func (s *Server) getVersion(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.Version(r.Context(), r.PathValue("name"), r.PathValue("hash"))
	if err != nil {
		failStore(w, err)
		return
	}
	writeRaw(w, v.Contract)
}

func (s *Server) getLatest(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		tag = "main"
	}
	v, err := s.store.Tagged(r.Context(), r.PathValue("name"), tag)
	if err != nil {
		failStore(w, err)
		return
	}
	writeRaw(w, v.Contract)
}

func (s *Server) putTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hash string `json:"hash"`
	}
	if !readBody(w, r, &body) {
		return
	}
	name, tag := r.PathValue("name"), r.PathValue("tag")
	if err := s.store.Tag(r.Context(), name, tag, body.Hash); err != nil {
		failStore(w, err)
		return
	}
	v, err := s.store.Version(r.Context(), name, body.Hash)
	if err != nil {
		failStore(w, err)
		return
	}
	writeJSONResponse(w, summarize(v))
}

func (s *Server) putConsumer(w http.ResponseWriter, r *http.Request) {
	var record Consumer
	if !readBody(w, r, &record) {
		return
	}
	record.Provider = r.PathValue("name")
	if record.RecordedAt.IsZero() {
		record.RecordedAt = s.now().UTC()
	}
	if err := s.store.PutConsumer(r.Context(), record); err != nil {
		fail(w, bowline.InvalidArgument, err.Error())
		return
	}
	writeJSONResponse(w, record)
}

func (s *Server) listConsumers(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Consumers(r.Context(), r.PathValue("name"))
	if err != nil {
		failStore(w, err)
		return
	}
	if list == nil {
		list = []Consumer{}
	}
	writeJSONResponse(w, list)
}

func (s *Server) putComposition(w http.ResponseWriter, r *http.Request) {
	var record Composition
	if !readBody(w, r, &record) {
		return
	}
	record.Gateway = r.PathValue("name")
	if record.PublishedAt.IsZero() {
		record.PublishedAt = s.now().UTC()
	}
	if err := s.store.PutComposition(r.Context(), record); err != nil {
		fail(w, bowline.InvalidArgument, err.Error())
		return
	}
	writeJSONResponse(w, record)
}

func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	services, err := s.store.Services(ctx)
	if err != nil {
		failStore(w, err)
		return
	}
	compositions, err := s.store.Compositions(ctx, "")
	if err != nil {
		failStore(w, err)
		return
	}
	kinds := map[string]string{}
	for _, s := range services {
		kinds[s.Name] = "service"
	}
	providers := make([]string, 0, len(services))
	for _, s := range services {
		providers = append(providers, s.Name)
	}
	var edges []Edge
	for _, c := range compositions {
		kinds[c.Gateway] = "gateway"
		providers = append(providers, c.Gateway)
		names := slices.Sorted(maps.Keys(c.Services))
		for _, name := range names {
			if _, ok := kinds[name]; !ok {
				kinds[name] = "service"
			}
			edges = append(edges, Edge{From: c.Gateway, To: name, Kind: "composes"})
		}
	}
	slices.Sort(providers)
	seen := map[Edge]bool{}
	var consumerEdges []Edge
	for _, provider := range providers {
		consumers, err := s.store.Consumers(ctx, provider)
		if err != nil {
			failStore(w, err)
			return
		}
		for _, c := range consumers {
			if _, ok := kinds[c.Consumer]; !ok {
				kinds[c.Consumer] = "consumer"
			}
			edge := Edge{From: c.Consumer, To: provider, Kind: "consumes"}
			if !seen[edge] {
				seen[edge] = true
				consumerEdges = append(consumerEdges, edge)
			}
		}
	}
	edges = append(edges, consumerEdges...)
	names := slices.Sorted(maps.Keys(kinds))
	nodes := make([]Node, 0, len(names))
	for _, name := range names {
		nodes = append(nodes, Node{Name: name, Kind: kinds[name]})
	}
	slices.SortFunc(edges, func(a, b Edge) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
	if edges == nil {
		edges = []Edge{}
	}
	writeJSONResponse(w, Graph{Nodes: nodes, Edges: edges})
}
