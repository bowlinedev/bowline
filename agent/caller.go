package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/bowlinedev/bowline"
)

type Caller interface {
	Call(ctx context.Context, procedure, method string, input json.RawMessage, headers http.Header) (json.RawMessage, *bowline.Error, error)
}

type httpCaller struct {
	base    string
	headers http.Header
	client  *http.Client
}

func HTTPCaller(url string, headers http.Header, client *http.Client) Caller {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpCaller{base: strings.TrimSuffix(url, "/"), headers: headers, client: client}
}

func (c *httpCaller) Call(ctx context.Context, procedure, method string, input json.RawMessage, headers http.Header) (json.RawMessage, *bowline.Error, error) {
	req, err := buildRequest(ctx, c.base, procedure, method, input, c.headers, headers)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, bowline.Errorf(bowline.Unavailable, "%v", err), nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, bowline.Errorf(bowline.Unavailable, "reading response: %v", err), nil
	}
	return decodeResponse(resp.StatusCode, body)
}

type handlerCaller struct {
	handler http.Handler
	headers http.Header
}

func HandlerCaller(h http.Handler, headers http.Header) Caller {
	return &handlerCaller{handler: h, headers: headers}
}

func (c *handlerCaller) Call(ctx context.Context, procedure, method string, input json.RawMessage, headers http.Header) (json.RawMessage, *bowline.Error, error) {
	req, err := buildRequest(ctx, "", procedure, method, input, c.headers, headers)
	if err != nil {
		return nil, nil, err
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	return decodeResponse(rec.Code, rec.Body.Bytes())
}

func buildRequest(ctx context.Context, base, procedure, method string, input json.RawMessage, static, dynamic http.Header) (*http.Request, error) {
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	target := base + "/" + procedure
	var body io.Reader
	if method == http.MethodGet {
		target += "?input=" + url.QueryEscape(string(input))
	} else {
		body = bytes.NewReader(input)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	for name, values := range static {
		req.Header[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	for name, values := range dynamic {
		req.Header[http.CanonicalHeaderKey(name)] = append([]string(nil), values...)
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

type envelope struct {
	Error struct {
		Code    bowline.Code    `json:"code"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
		Issues  []bowline.Issue `json:"issues"`
	} `json:"error"`
}

func decodeResponse(status int, body []byte) (json.RawMessage, *bowline.Error, error) {
	if status >= 200 && status < 300 {
		return json.RawMessage(body), nil, nil
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Code == "" {
		return nil, &bowline.Error{Code: bowline.Unknown, Message: "upstream returned HTTP " + http.StatusText(status)}, nil
	}
	e := &bowline.Error{Code: env.Error.Code, Message: env.Error.Message, Issues: env.Error.Issues}
	if len(env.Error.Details) > 0 && string(env.Error.Details) != "null" {
		var details any
		if json.Unmarshal(env.Error.Details, &details) == nil {
			e.Details = details
		}
	}
	return nil, e, nil
}
