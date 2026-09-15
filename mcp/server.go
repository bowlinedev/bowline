package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bowlinedev/bowline"
)

type Tool struct {
	Name        string
	Procedure   string
	Method      string
	Description string
	Input       json.RawMessage
	Output      json.RawMessage
	ReadOnly    bool
	Destructive bool
	Scopes      []string
}

type Server struct {
	tools      []Tool
	byName     map[string]Tool
	dispatcher Dispatcher
	opts       *options
}

func NewServer(tools []Tool, d Dispatcher, opts ...Option) *Server {
	o := newOptions(opts)
	s := &Server{dispatcher: d, opts: o, byName: map[string]Tool{}}
	for _, t := range tools {
		if !o.visible(t) {
			continue
		}
		s.tools = append(s.tools, t)
		s.byName[t.Name] = t
	}
	return s
}

func (s *Server) Tools() []Tool {
	return append([]Tool(nil), s.tools...)
}

func (s *Server) Handle(ctx context.Context, msg json.RawMessage) (json.RawMessage, error) {
	return s.HandleWithHeaders(ctx, msg, nil)
}

func (s *Server) HandleWithHeaders(ctx context.Context, msg json.RawMessage, headers http.Header) (json.RawMessage, error) {
	resp := s.handle(ctx, msg, headers)
	if resp == nil {
		return nil, nil
	}
	return json.Marshal(resp)
}

func (s *Server) handle(ctx context.Context, msg json.RawMessage, headers http.Header) *response {
	var req request
	if err := json.Unmarshal(msg, &req); err != nil {
		return errorResponse(nil, codeParse, "parse error: "+err.Error())
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return errorResponse(req.ID, codeInvalidRequest, "invalid request: jsonrpc must be \"2.0\" and method is required")
	}
	notification := len(req.ID) == 0 || string(req.ID) == "null"
	var resp *response
	switch req.Method {
	case "initialize":
		resp = s.initialize(req)
	case "notifications/initialized":
		return nil
	case "ping":
		resp = resultResponse(req.ID, map[string]any{})
	case "tools/list":
		resp = resultResponse(req.ID, map[string]any{"tools": s.list()})
	case "tools/call":
		resp = s.call(ctx, req, headers)
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		resp = errorResponse(req.ID, codeMethodNotFound, "method not found: "+req.Method)
	}
	if notification {
		return nil
	}
	return resp
}

func (s *Server) initialize(req request) *response {
	var params initializeParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, codeInvalidParams, "invalid initialize params: "+err.Error())
		}
	}
	if params.ProtocolVersion != "" && params.ProtocolVersion != ProtocolVersion {
		return errorResponse(req.ID, codeInvalidParams, fmt.Sprintf("unsupported protocol version %s; this server speaks %s", params.ProtocolVersion, ProtocolVersion))
	}
	return resultResponse(req.ID, map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": "bowline", "version": bowline.Version},
	})
}

func (s *Server) list() []listedTool {
	out := make([]listedTool, 0, len(s.tools))
	for _, t := range s.tools {
		input := t.Input
		if len(input) == 0 {
			input = json.RawMessage(`{"type":"object"}`)
		}
		out = append(out, listedTool{
			Name:         t.Name,
			Description:  t.Description,
			InputSchema:  input,
			OutputSchema: t.Output,
			Annotations:  annotations{Title: t.Procedure, ReadOnlyHint: t.ReadOnly, DestructiveHint: t.Destructive},
		})
	}
	return out
}

func (s *Server) call(ctx context.Context, req request, headers http.Header) *response {
	var params callParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, codeInvalidParams, "invalid tools/call params: "+err.Error())
	}
	tool, ok := s.byName[params.Name]
	if !ok {
		return errorResponse(req.ID, codeInvalidParams, "unknown tool: "+params.Name)
	}
	if s.opts.rate != nil && !s.opts.rate.allow(clientKey(headers.Get("Authorization"))) {
		return resultResponse(req.ID, failure("RESOURCE_EXHAUSTED", "rate limit exceeded"))
	}
	arguments := params.Arguments
	if len(arguments) == 0 {
		arguments = json.RawMessage("{}")
	}
	status, body, err := s.dispatcher.Dispatch(ctx, tool.Procedure, tool.Method, arguments, headers)
	if err != nil {
		s.opts.logger.ErrorContext(ctx, "mcp: dispatch failed", "tool", tool.Name, "error", err)
		return resultResponse(req.ID, failure("UNAVAILABLE", err.Error()))
	}
	return resultResponse(req.ID, shape(status, body))
}

type envelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Issues  []struct {
			Path    []string `json:"path"`
			Message string   `json:"message"`
		} `json:"issues"`
	} `json:"error"`
}

func shape(status int, body []byte) callResult {
	text := strings.TrimSpace(string(body))
	if status >= 200 && status < 300 {
		var structured any
		if len(body) > 0 && json.Unmarshal(body, &structured) == nil {
			return callResult{Content: []content{{Type: "text", Text: text}}, StructuredContent: structured}
		}
		return callResult{Content: []content{{Type: "text", Text: text}}}
	}
	var env envelope
	if json.Unmarshal(body, &env) != nil || env.Error.Code == "" {
		return failure("UNKNOWN", fmt.Sprintf("upstream returned HTTP %d", status))
	}
	var structured any
	if err := json.Unmarshal(body, &structured); err != nil {
		return failure("UNKNOWN", err.Error())
	}
	lines := []string{env.Error.Code + ": " + env.Error.Message}
	for _, issue := range env.Error.Issues {
		lines = append(lines, strings.Join(issue.Path, ".")+": "+issue.Message)
	}
	return callResult{Content: []content{{Type: "text", Text: strings.Join(lines, "\n")}}, StructuredContent: structured, IsError: true}
}

func failure(code, message string) callResult {
	structured := map[string]any{"error": map[string]any{"code": code, "message": message}}
	return callResult{Content: []content{{Type: "text", Text: code + ": " + message}}, StructuredContent: structured, IsError: true}
}
