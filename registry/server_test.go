package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/registry"
)

const token = "s3cret"

func newServer(t *testing.T) (*registry.Client, registry.Store) {
	t.Helper()
	store := registry.NewMemStore()
	srv := httptest.NewServer(registry.NewServer(store, registry.Options{Tokens: []string{token}}).Handler())
	t.Cleanup(srv.Close)
	return &registry.Client{URL: srv.URL, Token: token}, store
}

func document(t *testing.T, procedures ...*contract.Procedure) *contract.Document {
	t.Helper()
	doc := &contract.Document{
		Bowline:    contract.Version,
		Types:      map[string]*contract.TypeDecl{},
		Errors:     map[string]*contract.ErrorDecl{},
		Procedures: procedures,
	}
	return doc
}

func query(path string, output *contract.Type) *contract.Procedure {
	return &contract.Procedure{
		Path:   path,
		Kind:   "query",
		Method: http.MethodGet,
		Input:  &contract.Type{Kind: contract.Struct},
		Output: output,
	}
}

func object(fields ...*contract.Field) *contract.Type {
	return &contract.Type{Kind: contract.Struct, Fields: fields}
}

func field(name string, t *contract.Type) *contract.Field {
	return &contract.Field{Name: name, Type: t}
}

func str() *contract.Type { return &contract.Type{Kind: contract.Primitive, Name: "string"} }

func TestPublishAndRead(t *testing.T) {
	ctx := context.Background()
	client, _ := newServer(t)
	doc := document(t, query("invoices.list", object(field("total", str()))))
	result, err := client.Publish(ctx, "ledger", doc, "git:1", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Hash == "" {
		t.Fatalf("first publish: %+v", result)
	}
	again, err := client.Publish(ctx, "ledger", doc, "git:1", "main")
	if err != nil {
		t.Fatal(err)
	}
	if again.Created || again.Hash != result.Hash {
		t.Fatalf("republish must not create: %+v", again)
	}
	latest, err := client.Latest(ctx, "ledger", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(latest.Procedures) != 1 || latest.Procedures[0].Path != "invoices.list" {
		t.Fatalf("latest: %+v", latest.Procedures)
	}
	byHash, err := client.Version(ctx, "ledger", result.Hash)
	if err != nil || len(byHash.Procedures) != 1 {
		t.Fatalf("version by hash: %+v %v", byHash, err)
	}
	services, err := client.Services(ctx)
	if err != nil || len(services) != 1 || services[0].Name != "ledger" {
		t.Fatalf("publishing must register the service: %+v %v", services, err)
	}
}

func TestTagsAndServiceDetail(t *testing.T) {
	ctx := context.Background()
	client, _ := newServer(t)
	first := document(t, query("invoices.list", object(field("total", str()))))
	second := document(t, query("invoices.list", object(field("total", str()), field("note", str()))))
	a, err := client.Publish(ctx, "ledger", first, "git:1", "main")
	if err != nil {
		t.Fatal(err)
	}
	b, err := client.Publish(ctx, "ledger", second, "git:2", "")
	if err != nil {
		t.Fatal(err)
	}
	latest, err := client.Latest(ctx, "ledger", "")
	if err != nil || len(latest.Procedures[0].Output.Fields) != 1 {
		t.Fatalf("an untagged publish must not move main: %+v %v", latest, err)
	}
	summary, err := client.Tag(ctx, "ledger", "main", b.Hash)
	if err != nil || summary.Hash != b.Hash {
		t.Fatalf("tag: %+v %v", summary, err)
	}
	latest, err = client.Latest(ctx, "ledger", "main")
	if err != nil || len(latest.Procedures[0].Output.Fields) != 2 {
		t.Fatalf("main did not move: %+v %v", latest, err)
	}
	if a.Hash == b.Hash {
		t.Fatal("two different documents hashed the same")
	}
}

func TestConsumersAndGraph(t *testing.T) {
	ctx := context.Background()
	client, _ := newServer(t)
	doc := document(t, query("invoices.list", object(field("total", str()))))
	if _, err := client.Publish(ctx, "ledger", doc, "", "main"); err != nil {
		t.Fatal(err)
	}
	usage := json.RawMessage(`{"bowline":"1.2","consumer":"web","provider":"ledger","interactions":[{"procedure":"invoices.list","method":"GET","input":{},"response":{"status":200,"body":{"total":"USD 1.00"}}}]}`)
	if err := client.PublishConsumer(ctx, "ledger", registry.Consumer{Consumer: "web", Usage: usage}); err != nil {
		t.Fatal(err)
	}
	list, err := client.Consumers(ctx, "ledger")
	if err != nil || len(list) != 1 || list[0].Consumer != "web" || list[0].Provider != "ledger" {
		t.Fatalf("consumers: %+v %v", list, err)
	}
	if err := client.PublishComposition(ctx, "edge", registry.Composition{Hash: "sha256:ccc", Services: map[string]string{"ledger": "sha256:aaa"}}); err != nil {
		t.Fatal(err)
	}
	if err := client.PublishConsumer(ctx, "edge", registry.Consumer{Consumer: "mobile", Usage: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	graph, err := client.Graph(ctx)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, n := range graph.Nodes {
		kinds[n.Name] = n.Kind
	}
	if kinds["ledger"] != "service" || kinds["edge"] != "gateway" || kinds["web"] != "consumer" || kinds["mobile"] != "consumer" {
		t.Fatalf("node kinds: %+v", kinds)
	}
	want := map[registry.Edge]bool{
		{From: "edge", To: "ledger", Kind: "composes"}: true,
		{From: "web", To: "ledger", Kind: "consumes"}:  true,
		{From: "mobile", To: "edge", Kind: "consumes"}: true,
	}
	for _, e := range graph.Edges {
		delete(want, e)
	}
	if len(want) != 0 {
		t.Fatalf("missing edges %+v in %+v", want, graph.Edges)
	}
}

func TestWritesNeedAToken(t *testing.T) {
	ctx := context.Background()
	client, _ := newServer(t)
	anonymous := &registry.Client{URL: client.URL}
	doc := document(t, query("invoices.list", object(field("total", str()))))
	_, err := anonymous.Publish(ctx, "ledger", doc, "", "main")
	var be *bowline.Error
	if !errors.As(err, &be) || be.Code != bowline.Unauthenticated {
		t.Fatalf("publish without a token: %v", err)
	}
	wrong := &registry.Client{URL: client.URL, Token: "nope"}
	if err := wrong.PublishConsumer(ctx, "ledger", registry.Consumer{Consumer: "web", Usage: json.RawMessage(`{}`)}); !errors.As(err, &be) || be.Code != bowline.Unauthenticated {
		t.Fatalf("consumer with a wrong token: %v", err)
	}
	if _, err := anonymous.Services(ctx); err != nil {
		t.Fatalf("reads must stay open: %v", err)
	}
}

func TestErrorsUseTheBowlineEnvelope(t *testing.T) {
	ctx := context.Background()
	client, _ := newServer(t)
	var be *bowline.Error
	if _, err := client.Latest(ctx, "ghost", "main"); !errors.As(err, &be) || be.Code != bowline.NotFound {
		t.Fatalf("unknown service: %v", err)
	}
	resp, err := http.Post(client.URL+"/v1/services/ledger/versions", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth runs before parsing: %d", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodPost, client.URL+"/v1/services/ledger/versions", strings.NewReader("{not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("a malformed contract must be INVALID_ARGUMENT: %d", resp.StatusCode)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil || envelope.Error.Code != "INVALID_ARGUMENT" {
		t.Fatalf("envelope: %+v %v", envelope, err)
	}
}

func TestHealthAndIndex(t *testing.T) {
	client, _ := newServer(t)
	resp, err := http.Get(client.URL + "/v1/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var ok map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&ok); err != nil || !ok["ok"] {
		t.Fatalf("healthz: %+v %v", ok, err)
	}
	index, err := http.Get(client.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer index.Body.Close()
	body, _ := io.ReadAll(index.Body)
	if index.StatusCode != http.StatusOK || !strings.Contains(string(body), "/v1/services") {
		t.Fatalf("index: %d %s", index.StatusCode, body)
	}
}
