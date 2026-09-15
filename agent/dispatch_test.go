package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
)

type getInput struct {
	ID int64 `json:"id" validate:"required"`
}

type invoice struct {
	ID    int64  `json:"id"`
	Total string `json:"total"`
}

type voidInput struct {
	ID int64 `json:"id"`
}

func testRouter() *bowline.Router {
	return bowline.NewRouter(bowline.Mount("invoices", bowline.NewRouter(
		bowline.Query("get", func(ctx context.Context, in getInput) (invoice, error) {
			if in.ID == 999 {
				return invoice{}, bowline.Errorf(bowline.NotFound, "invoice %d not found", in.ID)
			}
			return invoice{ID: in.ID, Total: "USD 1500.00"}, nil
		}, bowline.Tool(bowline.Scope("billing"))),
		bowline.Mutation("void", func(ctx context.Context, in voidInput) (invoice, error) {
			call := bowline.CallFrom(ctx)
			if call.Request.Header.Get("Authorization") != "Bearer dev" {
				return invoice{}, bowline.Errorf(bowline.Unauthenticated, "token required")
			}
			return invoice{ID: in.ID, Total: "USD 0.00"}, nil
		}, bowline.Tool(bowline.Scope("billing"), bowline.Destructive())),
	)))
}

func testTools() []Tool {
	return []Tool{
		{Name: "invoices_get", Procedure: "invoices.get", ReadOnly: true, InputSchema: json.RawMessage(`{}`), OutputSchema: json.RawMessage(`{}`)},
		{Name: "invoices_void", Procedure: "invoices.void", Destructive: true, InputSchema: json.RawMessage(`{}`), OutputSchema: json.RawMessage(`{}`)},
	}
}

type spyTracer struct {
	started  []Call
	finished []Result
}

func (s *spyTracer) Start(ctx context.Context, call Call) (context.Context, func(Result)) {
	s.started = append(s.started, call)
	return ctx, func(r Result) { s.finished = append(s.finished, r) }
}

func TestDispatchThroughHandler(t *testing.T) {
	tracer := &spyTracer{}
	d := NewDispatcher(testTools(), HandlerCaller(testRouter().Handler(), http.Header{"Authorization": {"Bearer dev"}}), WithTracer(tracer))
	res, err := d.Dispatch(context.Background(), Call{ID: "1", Tool: "invoices_get", Input: json.RawMessage(`{"id":3}`)})
	if err != nil || res.Error != nil {
		t.Fatalf("get: %v %v", err, res.Error)
	}
	var out invoice
	if err := json.Unmarshal(res.Output, &out); err != nil || out.ID != 3 || out.Total != "USD 1500.00" {
		t.Fatalf("output %s", res.Output)
	}
	res, err = d.Dispatch(context.Background(), Call{ID: "2", Tool: "invoices_get", Input: json.RawMessage(`{"id":999}`)})
	if err != nil || res.Error == nil || res.Error.Code != bowline.NotFound || !strings.Contains(res.Error.Message, "999") {
		t.Fatalf("missing: %v %+v", err, res.Error)
	}
	res, err = d.Dispatch(context.Background(), Call{ID: "3", Tool: "invoices_void", Input: json.RawMessage(`{"id":3}`)})
	if err != nil || res.Error != nil || !strings.Contains(string(res.Output), `"USD 0.00"`) {
		t.Fatalf("void: %v %+v %s", err, res.Error, res.Output)
	}
	res, err = d.Dispatch(context.Background(), Call{ID: "4", Tool: "invoices_get", Input: json.RawMessage(`{"id":0}`)})
	if err != nil || res.Error == nil || res.Error.Code != bowline.InvalidArgument || len(res.Error.Issues) != 1 || res.Error.Issues[0].Rule != "required" {
		t.Fatalf("validation: %v %+v", err, res.Error)
	}
	if len(tracer.started) != 4 || len(tracer.finished) != 4 || tracer.started[1].ID != "2" || tracer.finished[1].Error == nil {
		t.Fatalf("tracer %+v %+v", tracer.started, tracer.finished)
	}
	if _, err := d.Dispatch(context.Background(), Call{Tool: "nope"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown tool: %v", err)
	}
}

func TestRecordingTracerWritesLines(t *testing.T) {
	var buf bytes.Buffer
	d := NewDispatcher(testTools(), HandlerCaller(testRouter().Handler(), nil), WithTracer(NewRecordingTracer(&buf)))
	d.Dispatch(context.Background(), Call{ID: "a", Tool: "invoices_get", Input: json.RawMessage(`{"id":3}`)})
	d.Dispatch(context.Background(), Call{ID: "b", Tool: "invoices_get", Input: json.RawMessage(`{"id":999}`)})
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines %q", buf.String())
	}
	var first struct {
		ID         string          `json:"id"`
		Tool       string          `json:"tool"`
		Input      json.RawMessage `json:"input"`
		Output     json.RawMessage `json:"output"`
		DurationMs int64           `json:"durationMs"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first.ID != "a" || first.Tool != "invoices_get" || string(first.Input) != `{"id":3}` || !strings.Contains(string(first.Output), `"id":3`) || first.DurationMs < 0 {
		t.Fatalf("first %s (%v)", lines[0], err)
	}
	if !strings.Contains(lines[1], `"error":{"code":"NOT_FOUND"`) || strings.Contains(lines[1], `"output"`) {
		t.Fatalf("second %s", lines[1])
	}
}

func TestHTTPCallerForwardsHeaders(t *testing.T) {
	var seen http.Header
	var seenURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		seenURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":3}`))
	}))
	defer srv.Close()
	caller := HTTPCaller(srv.URL+"/api/", http.Header{"Authorization": {"Bearer static"}, "X-Static": {"1"}}, srv.Client())
	out, callErr, err := caller.Call(context.Background(), "invoices.get", http.MethodGet, json.RawMessage(`{"id":3}`), http.Header{"Authorization": {"Bearer dynamic"}})
	if err != nil || callErr != nil || string(out) != `{"id":3}` {
		t.Fatalf("%s %v %v", out, callErr, err)
	}
	if seen.Get("Authorization") != "Bearer dynamic" || seen.Get("X-Static") != "1" || seenURL != "/api/invoices.get?input=%7B%22id%22%3A3%7D" {
		t.Fatalf("headers %v url %s", seen, seenURL)
	}
	out, callErr, err = caller.Call(context.Background(), "invoices.void", http.MethodPost, json.RawMessage(`{"id":3}`), nil)
	if err != nil || callErr != nil || seen.Get("Content-Type") != "application/json" || seenURL != "/api/invoices.void" {
		t.Fatalf("post: %s %v %v %v %s", out, callErr, err, seen, seenURL)
	}
}

func TestHTTPCallerMapsConnectionFailure(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	caller := HTTPCaller(url, nil, nil)
	out, callErr, err := caller.Call(context.Background(), "invoices.get", http.MethodGet, nil, nil)
	if err != nil || out != nil || callErr == nil || callErr.Code != bowline.Unavailable {
		t.Fatalf("%s %+v %v", out, callErr, err)
	}
	d := NewDispatcher(testTools(), caller)
	res, err := d.Dispatch(context.Background(), Call{Tool: "invoices_get"})
	if err != nil || res.Error == nil || res.Error.Code != bowline.Unavailable {
		t.Fatalf("%+v %v", res.Error, err)
	}
}

func TestDecodeResponseWithoutEnvelope(t *testing.T) {
	_, callErr, err := decodeResponse(http.StatusBadGateway, []byte("<html>"))
	if err != nil || callErr == nil || callErr.Code != bowline.Unknown || !strings.Contains(callErr.Message, "Bad Gateway") {
		t.Fatalf("%+v %v", callErr, err)
	}
}
