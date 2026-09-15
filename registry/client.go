package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/contract"
)

type Client struct {
	URL   string
	Token string
	HTTP  *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, headers map[string]string) ([]byte, error) {
	target := strings.TrimRight(c.URL, "/") + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, &bowline.Error{Code: bowline.InvalidArgument, Message: err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &bowline.Error{Code: bowline.Canceled, Message: "request canceled"}
		}
		return nil, &bowline.Error{Code: bowline.Unavailable, Message: fmt.Sprintf("calling the registry at %s: %v", target, err)}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, &bowline.Error{Code: bowline.Unavailable, Message: err.Error()}
	}
	if resp.StatusCode >= 400 {
		return nil, decodeError(resp.StatusCode, data)
	}
	return data, nil
}

func decodeError(status int, data []byte) error {
	var envelope struct {
		Error struct {
			Code    bowline.Code    `json:"code"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
			Issues  []bowline.Issue `json:"issues"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Error.Code == "" {
		return &bowline.Error{Code: bowline.Unknown, Message: fmt.Sprintf("the registry returned HTTP %d", status)}
	}
	e := &bowline.Error{Code: envelope.Error.Code, Message: envelope.Error.Message, Issues: envelope.Error.Issues}
	if len(envelope.Error.Details) > 0 {
		e.Details = envelope.Error.Details
	}
	return e
}

func (c *Client) Publish(ctx context.Context, service string, doc *contract.Document, ref, tag string) (PublishResult, error) {
	body, err := doc.Marshal()
	if err != nil {
		return PublishResult{}, &bowline.Error{Code: bowline.InvalidArgument, Message: err.Error()}
	}
	data, err := c.do(ctx, http.MethodPost, "/v1/services/"+url.PathEscape(service)+"/versions", body, map[string]string{
		"Bowline-Ref": ref,
		"Bowline-Tag": tag,
	})
	if err != nil {
		return PublishResult{}, err
	}
	var result PublishResult
	if err := json.Unmarshal(data, &result); err != nil {
		return PublishResult{}, &bowline.Error{Code: bowline.Internal, Message: "decoding the publish result: " + err.Error()}
	}
	return result, nil
}

func (c *Client) PublishConsumer(ctx context.Context, provider string, consumer Consumer) error {
	body, err := json.Marshal(consumer)
	if err != nil {
		return &bowline.Error{Code: bowline.InvalidArgument, Message: err.Error()}
	}
	_, err = c.do(ctx, http.MethodPost, "/v1/services/"+url.PathEscape(provider)+"/consumers", body, nil)
	return err
}

func (c *Client) Latest(ctx context.Context, service, tag string) (*contract.Document, error) {
	path := "/v1/services/" + url.PathEscape(service) + "/latest"
	if tag != "" {
		path += "?tag=" + url.QueryEscape(tag)
	}
	data, err := c.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	return parseDocument(data)
}

func (c *Client) Version(ctx context.Context, service, hash string) (*contract.Document, error) {
	data, err := c.do(ctx, http.MethodGet, "/v1/services/"+url.PathEscape(service)+"/versions/"+url.PathEscape(hash), nil, nil)
	if err != nil {
		return nil, err
	}
	return parseDocument(data)
}

func parseDocument(data []byte) (*contract.Document, error) {
	doc, err := contract.Parse(data)
	if err != nil {
		return nil, &bowline.Error{Code: bowline.Internal, Message: "the registry returned a contract that does not parse: " + err.Error()}
	}
	return doc, nil
}

func (c *Client) Impact(ctx context.Context, service string, candidate *contract.Document, strict bool) (*ImpactReport, error) {
	body, err := candidate.Marshal()
	if err != nil {
		return nil, &bowline.Error{Code: bowline.InvalidArgument, Message: err.Error()}
	}
	path := "/v1/services/" + url.PathEscape(service) + "/impact"
	if strict {
		path += "?strict=true"
	}
	data, err := c.do(ctx, http.MethodPost, path, body, nil)
	if err != nil {
		return nil, err
	}
	var report ImpactReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, &bowline.Error{Code: bowline.Internal, Message: "decoding the impact report: " + err.Error()}
	}
	return &report, nil
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	data, err := c.do(ctx, http.MethodGet, "/v1/services", nil, nil)
	if err != nil {
		return nil, err
	}
	var out []Service
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &bowline.Error{Code: bowline.Internal, Message: err.Error()}
	}
	return out, nil
}

func (c *Client) PutService(ctx context.Context, s Service) error {
	body, err := json.Marshal(s)
	if err != nil {
		return &bowline.Error{Code: bowline.InvalidArgument, Message: err.Error()}
	}
	_, err = c.do(ctx, http.MethodPut, "/v1/services/"+url.PathEscape(s.Name), body, nil)
	return err
}

func (c *Client) Tag(ctx context.Context, service, tag, hash string) (VersionSummary, error) {
	body, err := json.Marshal(map[string]string{"hash": hash})
	if err != nil {
		return VersionSummary{}, &bowline.Error{Code: bowline.InvalidArgument, Message: err.Error()}
	}
	data, err := c.do(ctx, http.MethodPut, "/v1/services/"+url.PathEscape(service)+"/tags/"+url.PathEscape(tag), body, nil)
	if err != nil {
		return VersionSummary{}, err
	}
	var summary VersionSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return VersionSummary{}, &bowline.Error{Code: bowline.Internal, Message: err.Error()}
	}
	return summary, nil
}

func (c *Client) Consumers(ctx context.Context, provider string) ([]Consumer, error) {
	data, err := c.do(ctx, http.MethodGet, "/v1/services/"+url.PathEscape(provider)+"/consumers", nil, nil)
	if err != nil {
		return nil, err
	}
	var out []Consumer
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &bowline.Error{Code: bowline.Internal, Message: err.Error()}
	}
	return out, nil
}

func (c *Client) PublishComposition(ctx context.Context, gateway string, composition Composition) error {
	body, err := json.Marshal(composition)
	if err != nil {
		return &bowline.Error{Code: bowline.InvalidArgument, Message: err.Error()}
	}
	_, err = c.do(ctx, http.MethodPost, "/v1/gateways/"+url.PathEscape(gateway)+"/compositions", body, nil)
	return err
}

func (c *Client) Graph(ctx context.Context) (Graph, error) {
	data, err := c.do(ctx, http.MethodGet, "/v1/graph", nil, nil)
	if err != nil {
		return Graph{}, err
	}
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return Graph{}, &bowline.Error{Code: bowline.Internal, Message: err.Error()}
	}
	return g, nil
}
