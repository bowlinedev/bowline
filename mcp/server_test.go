package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/contract"
)

type getInput struct {
	ID int `json:"id" validate:"required"`
}

type invoice struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type createInput struct {
	Title string `json:"title" validate:"required,min=3"`
}

func requireAuth(next bowline.Next) bowline.Next {
	return func(ctx context.Context, in any) (any, error) {
		if bowline.CallFrom(ctx).Request.Header.Get("Authorization") == "" {
			return nil, bowline.Errorf(bowline.Unauthenticated, "missing token")
		}
		return next(ctx, in)
	}
}

func testRouter() *bowline.Router {
	get := func(ctx context.Context, in getInput) (invoice, error) {
		switch in.ID {
		case 404:
			return invoice{}, bowline.Errorf(bowline.NotFound, "invoice %d not found", in.ID)
		case 500:
			panic("boom")
		}
		return invoice{ID: in.ID, Title: "Invoice " + string(rune('0'+in.ID%10))}, nil
	}
	create := func(ctx context.Context, in createInput) (invoice, error) {
		return invoice{ID: 1, Title: in.Title}, nil
	}
	secret := func(ctx context.Context, in struct{}) (string, error) {
		return "hidden", nil
	}
	r := bowline.NewRouter(
		bowline.Mount("invoices", bowline.NewRouter(
			bowline.Query("get", get, bowline.Tool(bowline.Scope("invoices:read"))),
			bowline.Mutation("create", create, bowline.Tool(bowline.Scope("invoices:write"), bowline.Destructive())),
			bowline.Query("secret", secret),
		)),
	)
	r.Use(requireAuth)
	return r
}

const (
	getInputSchema     = `{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"]}`
	invoiceSchema      = `{"type":"object","properties":{"id":{"type":"integer"},"title":{"type":"string"}}}`
	createInputSchema  = `{"type":"object","properties":{"title":{"type":"string","minLength":3}},"required":["title"]}`
	secretOutputSchema = `{"type":"string"}`
)

func testDocument() *contract.Document {
	return &contract.Document{
		Bowline: contract.Version,
		Types:   map[string]*contract.TypeDecl{},
		Errors: map[string]*contract.ErrorDecl{
			"InvoiceNotFound": {Name: "InvoiceNotFound", Code: "NOT_FOUND"},
		},
		Procedures: []*contract.Procedure{
			{
				Path:    "invoices.get",
				Kind:    "query",
				Method:  "GET",
				Errors:  []string{"InvoiceNotFound"},
				Tool:    &contract.Tool{Scopes: []string{"invoices:read"}, ReadOnly: true},
				Schemas: &contract.Schemas{Input: json.RawMessage(getInputSchema), Output: json.RawMessage(invoiceSchema)},
				Doc:     "Get returns one invoice.",
			},
			{
				Path:    "invoices.create",
				Kind:    "mutation",
				Method:  "POST",
				Tool:    &contract.Tool{Scopes: []string{"invoices:write"}, Destructive: true},
				Schemas: &contract.Schemas{Input: json.RawMessage(createInputSchema), Output: json.RawMessage(invoiceSchema)},
				Doc:     "Create makes an invoice.",
			},
			{
				Path:    "invoices.secret",
				Kind:    "query",
				Method:  "GET",
				Schemas: &contract.Schemas{Input: json.RawMessage(`{"type":"object"}`), Output: json.RawMessage(secretOutputSchema)},
			},
		},
	}
}

func testTools(t *testing.T) []Tool {
	t.Helper()
	doc := testDocument()
	source, err := SchemasFromContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := ToolsFromContract(doc, source)
	if err != nil {
		t.Fatal(err)
	}
	return tools
}

func testServer(t *testing.T, opts ...Option) *Server {
	t.Helper()
	return NewServer(testTools(t), RouterDispatcher(testRouter()), opts...)
}

type reply struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

func send(t *testing.T, s *Server, msg string) reply {
	t.Helper()
	raw, err := s.Handle(context.Background(), json.RawMessage(msg))
	if err != nil {
		t.Fatal(err)
	}
	var r reply
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if r.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc = %q", r.JSONRPC)
	}
	return r
}

func sendWithAuth(t *testing.T, s *Server, token, msg string) reply {
	t.Helper()
	headers := http.Header{}
	if token != "" {
		headers["Authorization"] = []string{token}
	}
	raw, err := s.HandleWithHeaders(context.Background(), json.RawMessage(msg), headers)
	if err != nil {
		t.Fatal(err)
	}
	var r reply
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return r
}

type toolReply struct {
	Content           []content       `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

func decodeCall(t *testing.T, r reply) toolReply {
	t.Helper()
	if r.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", r.Error)
	}
	var out toolReply
	if err := json.Unmarshal(r.Result, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Content) != 1 || out.Content[0].Type != "text" {
		t.Fatalf("content = %+v", out.Content)
	}
	return out
}

func TestInitializeHandshake(t *testing.T) {
	s := testServer(t)
	r := send(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`)
	if r.Error != nil {
		t.Fatalf("error: %+v", r.Error)
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools *struct{} `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(r.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocolVersion = %q", result.ProtocolVersion)
	}
	if result.Capabilities.Tools == nil {
		t.Error("capabilities.tools missing")
	}
	if result.ServerInfo.Name != "bowline" || result.ServerInfo.Version != bowline.Version {
		t.Errorf("serverInfo = %+v", result.ServerInfo)
	}
	raw, err := s.Handle(context.Background(), json.RawMessage(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil || raw != nil {
		t.Fatalf("notification produced %s, %v", raw, err)
	}
	if r := send(t, s, `{"jsonrpc":"2.0","id":2,"method":"ping"}`); r.Error != nil || string(r.Result) != "{}" {
		t.Errorf("ping = %s %+v", r.Result, r.Error)
	}
}

func TestInitializeVersionMismatch(t *testing.T) {
	r := send(t, testServer(t), `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	if r.Error == nil || r.Error.Code != codeInvalidParams {
		t.Fatalf("error = %+v", r.Error)
	}
	if !strings.Contains(r.Error.Message, ProtocolVersion) {
		t.Errorf("message %q does not name the supported version", r.Error.Message)
	}
}

func TestToolsListHidesUnexposed(t *testing.T) {
	r := send(t, testServer(t), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	var result struct {
		Tools []struct {
			Name         string          `json:"name"`
			Description  string          `json:"description"`
			InputSchema  json.RawMessage `json:"inputSchema"`
			OutputSchema json.RawMessage `json:"outputSchema"`
			Annotations  annotations     `json:"annotations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(r.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("tools = %+v", result.Tools)
	}
	get, create := result.Tools[0], result.Tools[1]
	if get.Name != "invoices_get" || create.Name != "invoices_create" {
		t.Errorf("names = %s, %s", get.Name, create.Name)
	}
	if get.Description != "Get returns one invoice.\nErrors: InvoiceNotFound" {
		t.Errorf("description = %q", get.Description)
	}
	if string(get.InputSchema) != getInputSchema || string(get.OutputSchema) != invoiceSchema {
		t.Errorf("schemas = %s / %s", get.InputSchema, get.OutputSchema)
	}
	if !get.Annotations.ReadOnlyHint || get.Annotations.DestructiveHint || get.Annotations.Title != "invoices.get" {
		t.Errorf("get annotations = %+v", get.Annotations)
	}
	if create.Annotations.ReadOnlyHint || !create.Annotations.DestructiveHint {
		t.Errorf("create annotations = %+v", create.Annotations)
	}
	for _, tool := range result.Tools {
		if tool.Name == "invoices_secret" {
			t.Error("unexposed procedure listed")
		}
	}
}

func TestToolsCallSuccess(t *testing.T) {
	s := testServer(t)
	out := decodeCall(t, sendWithAuth(t, s, "Bearer x", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":7}}}`))
	if out.IsError {
		t.Fatalf("isError set: %s", out.Content[0].Text)
	}
	if string(out.StructuredContent) != `{"id":7,"title":"Invoice 7"}` {
		t.Errorf("structuredContent = %s", out.StructuredContent)
	}
	if out.Content[0].Text != `{"id":7,"title":"Invoice 7"}` {
		t.Errorf("text = %q", out.Content[0].Text)
	}
	out = decodeCall(t, sendWithAuth(t, s, "Bearer x", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"invoices_create","arguments":{"title":"Rent"}}}`))
	if out.IsError || string(out.StructuredContent) != `{"id":1,"title":"Rent"}` {
		t.Errorf("create = %+v", out)
	}
}

func TestToolsCallUnauthenticated(t *testing.T) {
	out := decodeCall(t, send(t, testServer(t), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":7}}}`))
	if !out.IsError {
		t.Fatal("expected isError")
	}
	if out.Content[0].Text != "UNAUTHENTICATED: missing token" {
		t.Errorf("text = %q", out.Content[0].Text)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.StructuredContent, &env); err != nil || env.Error.Code != "UNAUTHENTICATED" {
		t.Errorf("structuredContent = %s", out.StructuredContent)
	}
}

func TestToolsCallValidationIssues(t *testing.T) {
	out := decodeCall(t, sendWithAuth(t, testServer(t), "Bearer x", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_create","arguments":{"title":"no"}}}`))
	if !out.IsError {
		t.Fatal("expected isError")
	}
	lines := strings.Split(out.Content[0].Text, "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "INVALID_ARGUMENT: ") || !strings.HasPrefix(lines[1], "title: ") {
		t.Errorf("text = %q", out.Content[0].Text)
	}
}

func TestToolsCallNotFoundAndPanic(t *testing.T) {
	s := testServer(t)
	out := decodeCall(t, sendWithAuth(t, s, "Bearer x", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":404}}}`))
	if !out.IsError || out.Content[0].Text != "NOT_FOUND: invoice 404 not found" {
		t.Errorf("not found = %+v", out)
	}
	out = decodeCall(t, sendWithAuth(t, s, "Bearer x", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":500}}}`))
	if !out.IsError || !strings.HasPrefix(out.Content[0].Text, "INTERNAL: ") {
		t.Errorf("panic = %+v", out)
	}
}

func TestToolsCallUnknownTool(t *testing.T) {
	r := send(t, testServer(t), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_secret","arguments":{}}}`)
	if r.Error == nil || r.Error.Code != codeInvalidParams {
		t.Fatalf("error = %+v", r.Error)
	}
}

func TestMalformedAndUnknownMethod(t *testing.T) {
	s := testServer(t)
	r := send(t, s, `{"jsonrpc":"2.0","id":1,`)
	if r.Error == nil || r.Error.Code != codeParse || string(r.ID) != "null" {
		t.Fatalf("parse error = %+v id=%s", r.Error, r.ID)
	}
	r = send(t, s, `{"jsonrpc":"2.0","id":2,"method":"resources/list"}`)
	if r.Error == nil || r.Error.Code != codeMethodNotFound {
		t.Fatalf("method not found = %+v", r.Error)
	}
	r = send(t, s, `{"id":3,"method":"ping"}`)
	if r.Error == nil || r.Error.Code != codeInvalidRequest {
		t.Fatalf("invalid request = %+v", r.Error)
	}
}

type failingDispatcher struct{}

func (failingDispatcher) Dispatch(context.Context, string, string, json.RawMessage, http.Header) (int, []byte, error) {
	return 0, nil, errors.New("upstream down")
}

func TestDispatchErrorBecomesToolError(t *testing.T) {
	s := NewServer(testTools(t), failingDispatcher{})
	out := decodeCall(t, send(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":1}}}`))
	if !out.IsError || out.Content[0].Text != "UNAVAILABLE: upstream down" {
		t.Errorf("result = %+v", out)
	}
}
