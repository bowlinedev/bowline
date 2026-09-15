package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/bowlinedev/bowline"
)

type Dispatcher interface {
	Dispatch(ctx context.Context, procedure, method string, input json.RawMessage, headers http.Header) (status int, body []byte, err error)
}

type routerDispatcher struct {
	handler http.Handler
}

func RouterDispatcher(r *bowline.Router, opts ...bowline.HandlerOption) Dispatcher {
	return &routerDispatcher{handler: r.Handler(opts...)}
}

func (d *routerDispatcher) Dispatch(ctx context.Context, procedure, method string, input json.RawMessage, headers http.Header) (int, []byte, error) {
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	var req *http.Request
	var err error
	if method == http.MethodGet {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, "/"+procedure+"?input="+url.QueryEscape(string(input)), nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, "/"+procedure, bytes.NewReader(input))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	if err != nil {
		return 0, nil, err
	}
	for name, values := range headers {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	w := newMemoryWriter()
	d.handler.ServeHTTP(w, req)
	return w.status, w.body.Bytes(), nil
}
