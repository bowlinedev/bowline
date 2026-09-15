package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/jsonschema"
	"github.com/bowlinedev/bowline/contract"
)

type Tool struct {
	Name        string
	Procedure   string
	Description string
	Input       json.RawMessage
	Output      json.RawMessage
	ReadOnly    bool
	Destructive bool
	Scopes      []string
}

type Filter struct {
	Scopes   []string
	ReadOnly bool
}

func FromContract(doc *contract.Document, filter Filter) ([]Tool, error) {
	var list []Tool
	seen := map[string]string{}
	for _, p := range doc.Procedures {
		if p.Tool == nil {
			continue
		}
		if filter.ReadOnly && !p.Tool.ReadOnly {
			continue
		}
		if len(filter.Scopes) > 0 && !intersects(filter.Scopes, p.Tool.Scopes) {
			continue
		}
		name := strings.ReplaceAll(p.Path, ".", "_")
		if other, dup := seen[name]; dup {
			return nil, fmt.Errorf("tool name %q is used by both %s and %s", name, other, p.Path)
		}
		seen[name] = p.Path
		input, output := p.Schemas.InputOrNil(), p.Schemas.OutputOrNil()
		if input == nil || output == nil {
			derivedIn, derivedOut, err := jsonschema.Procedure(doc, p)
			if err != nil {
				return nil, err
			}
			input, output = derivedIn, derivedOut
		}
		list = append(list, Tool{
			Name:        name,
			Procedure:   p.Path,
			Description: Describe(doc, p),
			Input:       input,
			Output:      output,
			ReadOnly:    p.Tool.ReadOnly,
			Destructive: p.Tool.Destructive,
			Scopes:      append([]string(nil), p.Tool.Scopes...),
		})
	}
	return list, nil
}

func Describe(doc *contract.Document, p *contract.Procedure) string {
	description := strings.TrimSpace(p.Doc)
	if len(p.Errors) > 0 {
		names := make([]string, 0, len(p.Errors))
		for _, id := range p.Errors {
			if decl, ok := doc.Errors[id]; ok {
				names = append(names, decl.Name)
			}
		}
		if description != "" {
			description += "\n"
		}
		description += "Errors: " + strings.Join(names, ", ")
	}
	return description
}

func intersects(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

const (
	FormatAnthropic  = "anthropic"
	FormatOpenAI     = "openai"
	FormatJSONSchema = "json-schema"
)

func Encode(list []Tool, format string) ([]byte, error) {
	switch format {
	case FormatAnthropic, FormatOpenAI, FormatJSONSchema:
	default:
		return nil, fmt.Errorf("unknown tool format %q; use anthropic, openai, or json-schema", format)
	}
	items := make([]map[string]any, 0, len(list))
	for _, t := range list {
		switch format {
		case FormatAnthropic:
			items = append(items, map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Input,
			})
		case FormatOpenAI:
			items = append(items, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Input,
				},
			})
		case FormatJSONSchema:
			items = append(items, map[string]any{
				"name":         t.Name,
				"procedure":    t.Procedure,
				"description":  t.Description,
				"inputSchema":  t.Input,
				"outputSchema": t.Output,
				"readOnly":     t.ReadOnly,
				"destructive":  t.Destructive,
				"scopes":       sortedCopy(t.Scopes),
			})
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sortedCopy(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
}
