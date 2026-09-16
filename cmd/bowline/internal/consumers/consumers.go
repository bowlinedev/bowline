package consumers

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/shape"
	"github.com/bowlinedev/bowline/contract"
)

type Consumer struct {
	Bowline      string        `json:"bowline"`
	Consumer     string        `json:"consumer"`
	Provider     string        `json:"provider,omitempty"`
	Interactions []Interaction `json:"interactions"`
}

type Interaction struct {
	Procedure string          `json:"procedure"`
	Method    string          `json:"method,omitempty"`
	Input     json.RawMessage `json:"input"`
	Response  Response        `json:"response"`
}

type Response struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

type Problem struct {
	Consumer  string
	Procedure string
	Path      []string
	Reason    string
}

func (p Problem) String() string {
	location := p.Procedure
	if len(p.Path) > 0 {
		location += " → " + strings.Join(p.Path, ".")
	}
	return fmt.Sprintf("consumer %s: %s: %s", p.Consumer, location, p.Reason)
}

func Load(dir string) ([]Consumer, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Consumer
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		c, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b Consumer) int { return cmp.Compare(a.Consumer, b.Consumer) })
	return out, nil
}

func Parse(data []byte) (Consumer, error) {
	var c Consumer
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Consumer == "" {
		return c, errors.New(`"consumer" is required`)
	}
	major, _, _ := strings.Cut(c.Bowline, ".")
	if major != "1" {
		return c, fmt.Errorf("consumer file format %q is not supported; this bowline reads 1.x", c.Bowline)
	}
	for i, in := range c.Interactions {
		if in.Procedure == "" {
			return c, fmt.Errorf("interaction %d has no procedure", i)
		}
	}
	return c, nil
}

func Verify(doc *contract.Document, list []Consumer) []Problem {
	procs := map[string]*contract.Procedure{}
	for _, p := range doc.Procedures {
		procs[p.Path] = p
	}
	var problems []Problem
	for _, c := range list {
		for _, in := range c.Interactions {
			problems = append(problems, verifyInteraction(doc, procs, c.Consumer, in)...)
		}
	}
	return problems
}

func verifyInteraction(doc *contract.Document, procs map[string]*contract.Procedure, consumer string, in Interaction) []Problem {
	p, ok := procs[in.Procedure]
	if !ok {
		return []Problem{{consumer, in.Procedure, nil, "procedure removed"}}
	}
	var problems []Problem
	if p.Kind == "subscription" {
		problems = append(problems, Problem{consumer, in.Procedure, nil, "procedure is now a subscription"})
		return problems
	}
	if in.Method != "" && in.Method != p.Method {
		if !(in.Method == "POST" && p.Method == "GET") {
			problems = append(problems, Problem{consumer, in.Procedure, nil, fmt.Sprintf("method changed from %s to %s", in.Method, p.Method)})
		}
	}
	input, err := decode(in.Input)
	if err != nil {
		problems = append(problems, Problem{consumer, in.Procedure, []string{"input"}, "recorded input is not JSON: " + err.Error()})
	} else {
		for _, m := range shape.Check(doc, p.Input, input) {
			problems = append(problems, Problem{consumer, in.Procedure, append([]string{"input"}, m.Path...), m.Reason})
		}
		if in.Response.Status < 400 {
			for _, issue := range shape.Validate(doc, p.Input, input) {
				problems = append(problems, Problem{consumer, in.Procedure, append([]string{"input"}, issue.Path...), "input no longer accepted: " + issue.Message})
			}
		}
	}
	body, err := decode(in.Response.Body)
	if err != nil {
		problems = append(problems, Problem{consumer, in.Procedure, []string{"response"}, "recorded response is not JSON: " + err.Error()})
		return problems
	}
	if in.Response.Status >= 400 {
		problems = append(problems, verifyError(doc, p, consumer, body)...)
		return problems
	}
	for _, m := range shape.Check(doc, p.Output, body) {
		problems = append(problems, Problem{consumer, in.Procedure, append([]string{"response"}, m.Path...), m.Reason})
	}
	return problems
}

func verifyError(doc *contract.Document, p *contract.Procedure, consumer string, body any) []Problem {
	obj, _ := body.(map[string]any)
	env, _ := obj["error"].(map[string]any)
	if env == nil {
		return []Problem{{consumer, p.Path, []string{"response"}, "recorded error response is not a Bowline error envelope"}}
	}
	typ, _ := env["type"].(string)
	if typ == "" {
		return nil
	}
	for _, id := range p.Errors {
		if decl, ok := doc.Errors[id]; ok && decl.Name == typ {
			details := env["details"]
			var problems []Problem
			if details != nil {
				for _, m := range shape.Check(doc, &contract.Type{Kind: contract.Struct, Fields: decl.Fields}, details) {
					problems = append(problems, Problem{consumer, p.Path, append([]string{"response", "error", "details"}, m.Path...), m.Reason})
				}
			}
			return problems
		}
	}
	return []Problem{{consumer, p.Path, []string{"response", "error", "type"}, fmt.Sprintf("error variant %s is no longer declared", typ)}}
}

func decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func Touches(c Consumer, procedure string, side string, path []string) int {
	count := 0
	for _, in := range c.Interactions {
		if in.Procedure != procedure {
			continue
		}
		raw := in.Response.Body
		if side == "input" {
			raw = in.Input
		} else if in.Response.Status >= 400 {
			continue
		}
		value, err := decode(raw)
		if err != nil {
			continue
		}
		if len(path) == 0 || present(value, path) {
			count++
		}
	}
	return count
}

func present(value any, path []string) bool {
	if len(path) == 0 {
		return value != nil
	}
	switch path[0] {
	case "*":
		switch x := value.(type) {
		case []any:
			for _, e := range x {
				if present(e, path[1:]) {
					return true
				}
			}
		case map[string]any:
			for _, e := range x {
				if present(e, path[1:]) {
					return true
				}
			}
		}
		return false
	default:
		obj, ok := value.(map[string]any)
		if !ok {
			return false
		}
		child, ok := obj[path[0]]
		if !ok {
			return false
		}
		return present(child, path[1:])
	}
}

func Report(problems []Problem, list []Consumer) string {
	var b strings.Builder
	if len(problems) == 0 {
		interactions := 0
		for _, c := range list {
			interactions += len(c.Interactions)
		}
		fmt.Fprintf(&b, "ok        %d interactions from %d consumer(s) still hold\n", interactions, len(list))
		return b.String()
	}
	for _, p := range problems {
		fmt.Fprintf(&b, "broken    %s\n", p)
	}
	fmt.Fprintf(&b, "bowline: %d problem(s) across consumer contracts\n", len(problems))
	return b.String()
}
