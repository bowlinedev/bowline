package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bowlinedev/bowline"
)

type Call struct {
	Tool  string
	ID    string
	Input json.RawMessage
}

type Result struct {
	Output json.RawMessage
	Error  *bowline.Error
}

type DispatchOption func(*Dispatcher)

func WithTracer(t Tracer) DispatchOption {
	return func(d *Dispatcher) { d.tracer = t }
}

type Dispatcher struct {
	tools   map[string]Tool
	methods map[string]string
	caller  Caller
	tracer  Tracer
}

func NewDispatcher(tools []Tool, caller Caller, opts ...DispatchOption) *Dispatcher {
	d := &Dispatcher{tools: map[string]Tool{}, methods: map[string]string{}, caller: caller}
	for _, t := range tools {
		d.tools[t.Name] = t
		d.methods[t.Name] = http.MethodPost
		if t.Method == http.MethodGet {
			d.methods[t.Name] = http.MethodGet
		}
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *Dispatcher) Dispatch(ctx context.Context, call Call) (Result, error) {
	tool, ok := d.tools[call.Tool]
	if !ok {
		return Result{}, fmt.Errorf("unknown tool %q", call.Tool)
	}
	finish := func(Result) {}
	if d.tracer != nil {
		ctx, finish = d.tracer.Start(ctx, call)
	}
	output, callErr, err := d.caller.Call(ctx, tool.Procedure, d.methods[call.Tool], call.Input, nil)
	if err != nil {
		return Result{}, err
	}
	result := Result{Output: output, Error: callErr}
	finish(result)
	return result, nil
}
