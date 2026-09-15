package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type Format string

const (
	Anthropic Format = "anthropic"
	OpenAI    Format = "openai"
	Schema    Format = "json-schema"
)

func Encode(tools []Tool, f Format) (json.RawMessage, error) {
	switch f {
	case Anthropic, OpenAI, Schema:
	default:
		return nil, fmt.Errorf("unknown tool format %q; use anthropic, openai, or json-schema", string(f))
	}
	items := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		switch f {
		case Anthropic:
			items = append(items, map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.InputSchema,
			})
		case OpenAI:
			items = append(items, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.InputSchema,
				},
			})
		case Schema:
			items = append(items, map[string]any{
				"name":         t.Name,
				"procedure":    t.Procedure,
				"description":  t.Description,
				"inputSchema":  t.InputSchema,
				"outputSchema": t.OutputSchema,
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
