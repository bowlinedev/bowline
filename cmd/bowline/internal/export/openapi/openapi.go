package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type Info struct {
	Title     string
	Version   string
	ServerURL string
}

type document struct {
	OpenAPI    string            `json:"openapi"`
	Info       map[string]string `json:"info"`
	Servers    []map[string]any  `json:"servers,omitempty"`
	Tags       []schema          `json:"tags,omitempty"`
	Paths      map[string]schema `json:"paths"`
	Components components        `json:"components"`
}

type components struct {
	Schemas         map[string]schema `json:"schemas"`
	Responses       map[string]schema `json:"responses"`
	SecuritySchemes map[string]schema `json:"securitySchemes,omitempty"`
}

func securitySchemes(doc *contract.Document) map[string]schema {
	if len(doc.Security) == 0 {
		return nil
	}
	out := make(map[string]schema, len(doc.Security))
	for name, declared := range doc.Security {
		entry := schema{"type": declared.Kind}
		switch declared.Kind {
		case "http":
			entry["scheme"] = declared.Scheme
			if declared.BearerFormat != "" {
				entry["bearerFormat"] = declared.BearerFormat
			}
		case "apiKey":
			entry["in"] = declared.In
			entry["name"] = declared.Name
		}
		if declared.Doc != "" {
			entry["description"] = declared.Doc
		}
		out[name] = entry
	}
	return out
}

func requirement(names []string) []any {
	if len(names) == 0 {
		return nil
	}
	both := schema{}
	for _, name := range names {
		both[name] = []any{}
	}
	return []any{both}
}

func tagOf(path string) string {
	if i := strings.LastIndex(path, "."); i > 0 {
		return path[:i]
	}
	return ""
}

var codes = []struct {
	code   string
	status int
}{
	{"CANCELED", 408}, {"UNKNOWN", 500}, {"INVALID_ARGUMENT", 400}, {"DEADLINE_EXCEEDED", 408},
	{"NOT_FOUND", 404}, {"ALREADY_EXISTS", 409}, {"PERMISSION_DENIED", 403}, {"RESOURCE_EXHAUSTED", 429},
	{"FAILED_PRECONDITION", 412}, {"ABORTED", 409}, {"OUT_OF_RANGE", 400}, {"UNIMPLEMENTED", 404},
	{"INTERNAL", 500}, {"UNAVAILABLE", 503}, {"DATA_LOSS", 500}, {"UNAUTHENTICATED", 401},
}

func statusOf(code string) int {
	for _, c := range codes {
		if c.code == code {
			return c.status
		}
	}
	return 500
}

func Export(doc *contract.Document, info Info) ([]byte, error) {
	if info.Title == "" {
		info.Title = "API"
	}
	if info.Version == "" {
		info.Version = "0.0.0"
	}
	s := newSchemas(doc)
	out := document{
		OpenAPI:    "3.1.0",
		Info:       map[string]string{"title": info.Title, "version": info.Version},
		Paths:      map[string]schema{},
		Components: components{Schemas: map[string]schema{}, Responses: map[string]schema{}},
	}
	if info.ServerURL != "" {
		out.Servers = []map[string]any{{"url": info.ServerURL}}
	}
	out.Components.Schemas["Error"] = errorSchema()
	out.Components.Schemas["Issue"] = issueSchema()
	for _, c := range codes {
		out.Components.Responses[c.code] = errorResponse(c.code, "", "")
	}
	errorIDs := slices.Sorted(maps.Keys(doc.Errors))
	for _, id := range errorIDs {
		name, err := s.errorDetails(id)
		if err != nil {
			return nil, err
		}
		decl := doc.Errors[id]
		out.Components.Schemas[name+"Envelope"] = variantEnvelope(name)
		out.Components.Responses[name] = errorResponse(decl.Code, name, decl.Doc)
	}
	out.Components.SecuritySchemes = securitySchemes(doc)
	tags := map[string]bool{}
	for _, p := range doc.Procedures {
		op, err := s.operation(p)
		if err != nil {
			return nil, err
		}
		if names := requirement(p.Security); names != nil {
			op["security"] = names
		}
		if tag := tagOf(p.Path); tag != "" {
			op["tags"] = []any{tag}
			tags[tag] = true
		}
		method := strings.ToLower(p.Method)
		if method == "" {
			method = "get"
		}
		route := "/" + p.Path
		if p.HTTPPath != "" {
			route = "/" + p.HTTPPath
			if params := s.pathParameters(p); len(params) > 0 {
				existing, _ := op["parameters"].([]any)
				op["parameters"] = append(params, existing...)
			}
		}
		if existing, ok := out.Paths[route]; ok {
			existing[method] = op
			out.Paths[route] = existing
			continue
		}
		out.Paths[route] = schema{method: op}
	}
	for _, name := range slices.Sorted(maps.Keys(tags)) {
		out.Tags = append(out.Tags, schema{"name": name})
	}
	maps.Copy(out.Components.Schemas, s.defined)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *schemas) operation(p *contract.Procedure) (schema, error) {
	input, err := s.node(p.Input, nil)
	if err != nil {
		return nil, err
	}
	output, err := s.node(p.Output, nil)
	if err != nil {
		return nil, err
	}
	op := schema{"operationId": p.Path}
	if p.Doc != "" {
		op["description"] = p.Doc
	}
	if p.Deprecated != "" {
		op["deprecated"] = true
	}
	emptyInput := p.Input.Kind == contract.Struct && len(p.Input.Fields) == 0
	switch p.Kind {
	case "upload":
		op["requestBody"] = schema{
			"required": true,
			"content": schema{"multipart/form-data": schema{
				"schema": schema{
					"type":       "object",
					"properties": schema{"input": input, "file": schema{"type": "string", "format": "binary"}},
					"required":   []string{"input", "file"},
				},
				"encoding": schema{"input": schema{"contentType": "application/json"}},
			}},
		}
	default:
		if p.Method == http.MethodGet {
			op["parameters"] = []schema{{
				"name":     "input",
				"in":       "query",
				"required": !emptyInput,
				"content":  schema{"application/json": schema{"schema": input}},
			}}
		} else {
			op["requestBody"] = schema{
				"required": !emptyInput,
				"content":  schema{"application/json": schema{"schema": input}},
			}
		}
	}
	responses := schema{}
	if p.Kind == "subscription" {
		responses["200"] = schema{
			"description": "A stream of server-sent events. Each message event carries one value; an error event carries the error envelope; a done event ends the stream.",
			"content":     schema{"text/event-stream": schema{"schema": output}},
		}
	} else {
		responses["200"] = schema{
			"description": "Success",
			"content":     schema{"application/json": schema{"schema": output}},
		}
	}
	byStatus := map[int][]schema{}
	for _, id := range p.Errors {
		name := s.names[id]
		status := statusOf(s.doc.Errors[id].Code)
		byStatus[status] = append(byStatus[status], schema{"$ref": "#/components/responses/" + name})
	}
	for _, c := range []string{"INVALID_ARGUMENT", "INTERNAL"} {
		status := statusOf(c)
		if _, taken := byStatus[status]; !taken {
			byStatus[status] = append(byStatus[status], schema{"$ref": "#/components/responses/" + c})
		}
	}
	statuses := slices.Sorted(maps.Keys(byStatus))
	for _, status := range statuses {
		refs := byStatus[status]
		key := strconv.Itoa(status)
		if len(refs) == 1 {
			responses[key] = refs[0]
			continue
		}
		responses[key] = schema{
			"description": "One of several declared errors.",
			"content":     schema{"application/json": schema{"schema": schema{"oneOf": inlineEnvelopes(refs)}}},
		}
	}
	op["responses"] = responses
	return op, nil
}

func inlineEnvelopes(refs []schema) []schema {
	out := make([]schema, 0, len(refs))
	for _, r := range refs {
		name := r["$ref"].(string)[len("#/components/responses/"):]
		out = append(out, schema{"$ref": "#/components/schemas/" + name + "Envelope"})
	}
	return out
}

func errorSchema() schema {
	values := make([]any, len(codes))
	for i, c := range codes {
		values[i] = c.code
	}
	return schema{
		"type": "object",
		"properties": schema{
			"error": schema{
				"type": "object",
				"properties": schema{
					"code":    schema{"type": "string", "enum": values},
					"message": schema{"type": "string"},
					"type":    schema{"type": "string"},
					"details": schema{},
					"issues":  schema{"type": "array", "items": ref("Issue")},
				},
				"required":             []string{"code", "message"},
				"additionalProperties": false,
			},
		},
		"required":             []string{"error"},
		"additionalProperties": false,
	}
}

func issueSchema() schema {
	return schema{
		"type": "object",
		"properties": schema{
			"path":    schema{"type": "array", "items": schema{"type": "string"}},
			"rule":    schema{"type": "string"},
			"message": schema{"type": "string"},
		},
		"required":             []string{"path", "rule", "message"},
		"additionalProperties": false,
	}
}

func variantEnvelope(variant string) schema {
	return schema{"allOf": []schema{
		ref("Error"),
		{
			"type": "object",
			"properties": schema{"error": schema{
				"type":       "object",
				"properties": schema{"type": schema{"const": variant}, "details": ref(variant)},
				"required":   []string{"type", "details"},
			}},
		},
	}}
}

func errorResponse(code, variant, doc string) schema {
	description := fmt.Sprintf("Error with code %s.", code)
	body := ref("Error")
	if variant != "" {
		description = fmt.Sprintf("Error variant %s with code %s.", variant, code)
		if doc != "" {
			description = doc
		}
		body = ref(variant + "Envelope")
	}
	return schema{
		"description": description,
		"content":     schema{"application/json": schema{"schema": body}},
	}
}

func (s *schemas) pathParameters(p *contract.Procedure) []any {
	names, err := contract.PathParams(p.HTTPPath)
	if err != nil || len(names) == 0 {
		return nil
	}
	fields := map[string]*contract.Field{}
	for _, f := range s.procedureInputFields(p.Input) {
		fields[f.Name] = f
	}
	out := make([]any, 0, len(names))
	for _, name := range names {
		param := map[string]any{"name": name, "in": "path", "required": true}
		if f, ok := fields[name]; ok {
			if f.Doc != "" {
				param["description"] = f.Doc
			}
			if node, err := s.node(f.Type, nil); err == nil {
				param["schema"] = node
			}
		}
		out = append(out, param)
	}
	return out
}

func (s *schemas) procedureInputFields(t *contract.Type) []*contract.Field {
	for range 8 {
		if t == nil {
			return nil
		}
		switch t.Kind {
		case contract.Struct:
			return t.Fields
		case contract.Ref:
			decl, ok := s.doc.Types[t.ID]
			if !ok {
				return nil
			}
			if decl.Kind == "struct" {
				return decl.Fields
			}
			if decl.Kind == "generic" {
				t = decl.Body
				continue
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}
