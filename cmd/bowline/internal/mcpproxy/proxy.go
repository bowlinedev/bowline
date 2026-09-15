package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bowlinedev/bowline/mcp"
)

type proxy struct {
	base   string
	static http.Header
	client *http.Client
}

func New(base string, static http.Header, client *http.Client) mcp.Dispatcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &proxy{base: strings.TrimRight(base, "/"), static: static, client: client}
}

func (p *proxy) Dispatch(ctx context.Context, procedure, method string, input json.RawMessage, headers http.Header) (int, []byte, error) {
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	var req *http.Request
	var err error
	if method == http.MethodGet {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, p.base+"/"+procedure+"?input="+url.QueryEscape(string(input)), nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/"+procedure, bytes.NewReader(input))
	}
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, values := range p.static {
		req.Header[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	for name, values := range headers {
		req.Header[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}
