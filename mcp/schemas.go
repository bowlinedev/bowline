package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

var ErrNoSchemas = errors.New("mcp: the contract carries no tool schemas; run bowline gen with \"schemas\": true")

type SchemaSource interface {
	Schemas(procedure string) (input, output json.RawMessage, ok bool)
}

type contractSchemas map[string]*contract.Schemas

func (c contractSchemas) Schemas(procedure string) (json.RawMessage, json.RawMessage, bool) {
	s, ok := c[procedure]
	if !ok {
		return nil, nil, false
	}
	return s.Input, s.Output, true
}

func SchemasFromContract(doc *contract.Document) (SchemaSource, error) {
	source := contractSchemas{}
	exposed := 0
	for _, p := range doc.Procedures {
		if p.Tool == nil {
			continue
		}
		exposed++
		if p.Schemas != nil {
			source[p.Path] = p.Schemas
		}
	}
	if exposed > 0 && len(source) == 0 {
		return nil, ErrNoSchemas
	}
	return source, nil
}

func ToolName(path string) string {
	return strings.ReplaceAll(path, ".", "_")
}

func ToolsFromContract(doc *contract.Document, source SchemaSource) ([]Tool, error) {
	var tools []Tool
	seen := map[string]string{}
	for _, p := range doc.Procedures {
		if p.Tool == nil {
			continue
		}
		name := ToolName(p.Path)
		if other, dup := seen[name]; dup {
			return nil, fmt.Errorf("mcp: procedures %s and %s both map to tool name %s", other, p.Path, name)
		}
		seen[name] = p.Path
		input, output, ok := source.Schemas(p.Path)
		if !ok {
			return nil, fmt.Errorf("mcp: procedure %s has no schemas; run bowline gen with \"schemas\": true", p.Path)
		}
		tools = append(tools, Tool{
			Name:        name,
			Procedure:   p.Path,
			Method:      p.Method,
			Description: describe(doc, p),
			Input:       input,
			Output:      output,
			ReadOnly:    p.Tool.ReadOnly,
			Destructive: p.Tool.Destructive,
			Idempotent:  p.Tool.ReadOnly || p.Idempotent,
			OpenWorld:   false,
			Scopes:      append([]string(nil), p.Tool.Scopes...),
		})
	}
	return tools, nil
}

func describe(doc *contract.Document, p *contract.Procedure) string {
	description := strings.TrimSpace(p.Doc)
	if len(p.Errors) == 0 {
		return description
	}
	names := make([]string, 0, len(p.Errors))
	for _, id := range p.Errors {
		if decl, ok := doc.Errors[id]; ok {
			names = append(names, decl.Name)
		}
	}
	if description != "" {
		description += "\n"
	}
	return description + "Errors: " + strings.Join(names, ", ")
}
