package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type Tool struct {
	Name         string
	Procedure    string
	Method       string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	ReadOnly     bool
	Destructive  bool
	Scopes       []string
}

type Option func(*filter)

type filter struct {
	scopes   []string
	readOnly bool
}

func Scopes(names ...string) Option {
	return func(f *filter) { f.scopes = append(f.scopes, names...) }
}

func ReadOnly() Option {
	return func(f *filter) { f.readOnly = true }
}

func Load(path string) (*contract.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return contract.Parse(data)
}

func Tools(doc *contract.Document, opts ...Option) ([]Tool, error) {
	var f filter
	for _, opt := range opts {
		opt(&f)
	}
	var list []Tool
	seen := map[string]string{}
	for _, p := range doc.Procedures {
		if p.Tool == nil {
			continue
		}
		if f.readOnly && !p.Tool.ReadOnly {
			continue
		}
		if len(f.scopes) > 0 && !intersects(f.scopes, p.Tool.Scopes) {
			continue
		}
		name := strings.ReplaceAll(p.Path, ".", "_")
		if other, dup := seen[name]; dup {
			return nil, fmt.Errorf("tool name %q is used by both %s and %s", name, other, p.Path)
		}
		seen[name] = p.Path
		input, output := p.Schemas.InputOrNil(), p.Schemas.OutputOrNil()
		if input == nil || output == nil {
			return nil, fmt.Errorf("procedure %s has no embedded schemas; run bowline gen with \"schemas\": true", p.Path)
		}
		list = append(list, Tool{
			Name:         name,
			Procedure:    p.Path,
			Method:       p.Method,
			Description:  describe(doc, p),
			InputSchema:  input,
			OutputSchema: output,
			ReadOnly:     p.Tool.ReadOnly,
			Destructive:  p.Tool.Destructive,
			Scopes:       append([]string(nil), p.Tool.Scopes...),
		})
	}
	return list, nil
}

func describe(doc *contract.Document, p *contract.Procedure) string {
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
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}
