package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bowlinedev/bowline"
)

type echoInput struct {
	Name string `json:"name"`
}

type echoOutput struct {
	Method string `json:"method"`
	Name   string `json:"name"`
	Token  string `json:"token"`
}

func echoRouter() *bowline.Router {
	echo := func(ctx context.Context, in echoInput) (echoOutput, error) {
		call := bowline.CallFrom(ctx)
		return echoOutput{Method: call.Request.Method, Name: in.Name, Token: call.Request.Header.Get("Authorization")}, nil
	}
	return bowline.NewRouter(
		bowline.Query("lookup", echo),
		bowline.Mutation("change", echo),
	)
}

func TestRouterDispatcherGetAndPost(t *testing.T) {
	d := RouterDispatcher(echoRouter())
	headers := http.Header{"Authorization": {"Bearer t"}}
	for _, tc := range []struct{ procedure, method string }{{"lookup", "GET"}, {"change", "POST"}} {
		status, body, err := d.Dispatch(context.Background(), tc.procedure, tc.method, json.RawMessage(`{"name":"ada"}`), headers)
		if err != nil {
			t.Fatal(err)
		}
		if status != http.StatusOK {
			t.Fatalf("%s: status %d body %s", tc.procedure, status, body)
		}
		var out echoOutput
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		if out.Method != tc.method || out.Name != "ada" || out.Token != "Bearer t" {
			t.Errorf("%s: %+v", tc.procedure, out)
		}
	}
}

func TestRouterDispatcherUnknownProcedure(t *testing.T) {
	d := RouterDispatcher(echoRouter())
	status, body, err := d.Dispatch(context.Background(), "missing", "GET", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusNotFound {
		t.Fatalf("status %d body %s", status, body)
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Code != "UNIMPLEMENTED" {
		t.Errorf("body = %s", body)
	}
}

func TestRouterDispatcherPassesHandlerOptions(t *testing.T) {
	d := RouterDispatcher(echoRouter(), bowline.StrictInput())
	status, body, err := d.Dispatch(context.Background(), "change", "POST", json.RawMessage(`{"name":"ada","extra":1}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if status < 400 {
		t.Fatalf("expected rejection, got %d %s", status, body)
	}
}
