package fake

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"maps"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bowlinedev/bowline/contract"
)

const (
	maxDepth    = 3
	safeInteger = 1<<53 - 1
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

type Generator struct {
	doc  *contract.Document
	seed uint64
	mu   sync.Mutex
	seq  map[string]int64
}

func New(doc *contract.Document, seed uint64) *Generator {
	return &Generator{doc: doc, seed: seed, seq: map[string]int64{}}
}

type scope struct {
	procedure string
	path      []string
	env       map[string]*contract.Type
	depth     map[string]int
	owner     string
}

func (s scope) child(segment string) scope {
	path := make([]string, len(s.path)+1)
	copy(path, s.path)
	path[len(s.path)] = segment
	return scope{procedure: s.procedure, path: path, env: s.env, depth: s.depth, owner: s.owner}
}

func (g *Generator) Output(p *contract.Procedure) any {
	return g.Value(p.Output, p.Path, nil)
}

func (g *Generator) Value(t *contract.Type, procedure string, path []string) any {
	return g.value(t, nil, scope{procedure: procedure, path: path, depth: map[string]int{}, owner: procedure})
}

func (g *Generator) stream(s scope) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(strconv.FormatUint(g.seed, 10)))
	h.Write([]byte{0})
	h.Write([]byte(s.procedure))
	for _, segment := range s.path {
		h.Write([]byte{0})
		h.Write([]byte(segment))
	}
	sum := h.Sum64()
	return rand.New(rand.NewPCG(sum, sum^0x9e3779b97f4a7c15))
}

func (g *Generator) nextID(owner string) int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq[owner]++
	return g.seq[owner]
}

func (g *Generator) value(t *contract.Type, f *contract.Field, s scope) any {
	switch t.Kind {
	case contract.Primitive:
		return g.primitive(t.Name, t.Encoding, f, s)
	case contract.Ref:
		return g.ref(t, f, s)
	case contract.Array:
		return g.array(t, f, s)
	case contract.Map:
		return g.mapValue(t, s)
	case contract.Struct:
		return g.object(t.Fields, s)
	case contract.Param:
		if bound, ok := s.env[t.Name]; ok {
			return g.value(bound, f, scope{procedure: s.procedure, path: s.path, env: nil, depth: s.depth, owner: s.owner})
		}
		return nil
	}
	return nil
}

func (g *Generator) ref(t *contract.Type, f *contract.Field, s scope) any {
	decl, ok := g.doc.Types[t.ID]
	if !ok {
		return nil
	}
	switch decl.Kind {
	case contract.Primitive:
		return g.primitive(decl.Primitive, t.Encoding, f, s)
	case contract.Enum:
		values := decl.Values
		if f != nil {
			for _, rule := range f.Rules {
				if rule.Rule != "oneof" {
					continue
				}
				allowed := strings.Fields(rule.Param)
				var kept []contract.EnumValue
				for _, v := range decl.Values {
					for _, a := range allowed {
						if keyString(v.Value) == a {
							kept = append(kept, v)
						}
					}
				}
				if len(kept) > 0 {
					values = kept
				}
			}
		}
		if len(values) == 0 {
			return nil
		}
		return values[g.stream(s).IntN(len(values))].Value
	case contract.Struct, contract.Generic:
		if s.depth[t.ID] >= maxDepth {
			return nil
		}
		depth := map[string]int{}
		maps.Copy(depth, s.depth)
		depth[t.ID]++
		env := map[string]*contract.Type{}
		fields := decl.Fields
		if decl.Kind == contract.Generic {
			for i, p := range decl.Params {
				if i < len(t.Args) {
					env[p] = substitute(t.Args[i], s.env)
				}
			}
			if decl.Body != nil {
				fields = decl.Body.Fields
			}
		}
		return g.object(fields, scope{procedure: s.procedure, path: s.path, env: env, depth: depth, owner: t.ID})
	}
	return nil
}

func substitute(t *contract.Type, env map[string]*contract.Type) *contract.Type {
	if t == nil || len(env) == 0 {
		return t
	}
	if t.Kind == contract.Param {
		if bound, ok := env[t.Name]; ok {
			return bound
		}
		return t
	}
	copied := *t
	if len(t.Args) > 0 {
		copied.Args = make([]*contract.Type, len(t.Args))
		for i, a := range t.Args {
			copied.Args[i] = substitute(a, env)
		}
	}
	copied.Elem = substitute(t.Elem, env)
	copied.Value = substitute(t.Value, env)
	copied.Key = substitute(t.Key, env)
	return &copied
}

func (g *Generator) object(fields []*contract.Field, s scope) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		fs := s.child(f.Name)
		r := g.stream(fs)
		if f.Example != nil {
			out[f.Name] = f.Example
			continue
		}
		required := hasRule(f, "required")
		if f.Optional && !required && r.Float64() >= 0.7 {
			continue
		}
		if f.Nullable && !required && r.Float64() < 0.2 {
			out[f.Name] = nil
			continue
		}
		if isID(f.Name) && idKind(g.doc, f.Type) != "" {
			owner := s.owner
			if f.Name != "id" && f.Name != "ID" {
				owner = "field:" + f.Name
			}
			id := g.nextID(owner)
			if idKind(g.doc, f.Type) == "string" {
				out[f.Name] = strconv.FormatInt(id, 10)
			} else if f.Type.Encoding == "string" {
				out[f.Name] = strconv.FormatInt(id, 10)
			} else {
				out[f.Name] = id
			}
			continue
		}
		v := g.value(f.Type, f, fs)
		if v == nil && !f.Nullable {
			if f.Optional {
				continue
			}
		}
		out[f.Name] = v
	}
	return out
}

func isID(name string) bool {
	return name == "id" || strings.HasSuffix(name, "Id") || strings.HasSuffix(name, "ID")
}

func idKind(doc *contract.Document, t *contract.Type) string {
	name := ""
	switch t.Kind {
	case contract.Primitive:
		name = t.Name
	case contract.Ref:
		if decl, ok := doc.Types[t.ID]; ok && decl.Kind == contract.Primitive {
			name = decl.Primitive
		}
	}
	switch name {
	case "string":
		return "string"
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	}
	return ""
}

func (g *Generator) array(t *contract.Type, f *contract.Field, s scope) any {
	if t.Elem != nil && t.Elem.Kind == contract.Ref && s.depth[t.Elem.ID] >= maxDepth {
		return []any{}
	}
	r := g.stream(s)
	count := 0
	switch {
	case t.Length > 0:
		count = t.Length
	default:
		lo, hi := 2, 4
		if f != nil {
			if v, ok := ruleInt(f, "len"); ok {
				lo, hi = v, v
			} else {
				if v, ok := ruleInt(f, "min"); ok {
					lo = v
					if hi < lo {
						hi = lo
					}
				}
				if v, ok := ruleInt(f, "max"); ok {
					hi = v
					if lo > hi {
						lo = hi
					}
				}
			}
			if hasRule(f, "required") && lo < 1 {
				lo = 1
				if hi < 1 {
					hi = 1
				}
			}
		}
		count = lo
		if hi > lo {
			count = lo + r.IntN(hi-lo+1)
		}
	}
	out := make([]any, 0, count)
	for i := 0; i < count; i++ {
		es := s.child(strconv.Itoa(i))
		if t.Elem.Nullable && r.Float64() < 0.2 {
			out = append(out, nil)
			continue
		}
		out = append(out, g.value(t.Elem, nil, es))
	}
	return out
}

func (g *Generator) mapValue(t *contract.Type, s scope) any {
	r := g.stream(s)
	count := 1 + r.IntN(3)
	out := map[string]any{}
	for i := 0; i < count; i++ {
		key := g.mapKey(t.Key, r)
		if _, dup := out[key]; dup {
			continue
		}
		out[key] = g.value(t.Value, nil, s.child(key))
	}
	return out
}

func (g *Generator) mapKey(t *contract.Type, r *rand.Rand) string {
	if t != nil {
		if t.Kind == contract.Ref {
			if decl, ok := g.doc.Types[t.ID]; ok && decl.Kind == contract.Enum && len(decl.Values) > 0 {
				return keyString(decl.Values[r.IntN(len(decl.Values))].Value)
			}
		}
		if idKind(g.doc, t) == "integer" {
			return strconv.Itoa(1 + r.IntN(1000))
		}
	}
	return words[r.IntN(len(words))]
}

func keyString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	}
	return fmt.Sprint(v)
}

func (g *Generator) primitive(name, encoding string, f *contract.Field, s scope) any {
	r := g.stream(s)
	switch name {
	case "string":
		return g.text(f, r)
	case "bool":
		return r.IntN(2) == 1
	case "int8", "int16", "int32", "int64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(name, "int"))
		v := g.integer(f, r, -(int64(1) << (bits - 1)), int64(1)<<(bits-1)-1)
		if encoding == "string" {
			return strconv.FormatInt(v, 10)
		}
		return v
	case "uint8", "uint16", "uint32", "uint64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(name, "uint"))
		upper := int64(safeInteger)
		if bits < 63 {
			upper = int64(1)<<bits - 1
		}
		v := g.integer(f, r, 0, upper)
		if encoding == "string" {
			return strconv.FormatInt(v, 10)
		}
		return v
	case "float32", "float64":
		return g.float(f, r)
	case "timestamp":
		offset := time.Duration(r.Int64N(int64(30 * 24 * time.Hour)))
		return epoch.Add(-offset).Truncate(time.Second).Format(time.RFC3339)
	case "duration":
		return int64(time.Millisecond) + r.Int64N(int64(time.Hour-time.Millisecond))
	case "bytes":
		buf := make([]byte, 8+r.IntN(25))
		for i := range buf {
			buf[i] = byte(r.IntN(256))
		}
		return base64.StdEncoding.EncodeToString(buf)
	case "raw":
		return map[string]any{}
	}
	return nil
}

func (g *Generator) text(f *contract.Field, r *rand.Rand) string {
	if f != nil {
		for _, rule := range f.Rules {
			switch rule.Rule {
			case "oneof":
				options := strings.Fields(rule.Param)
				if len(options) > 0 {
					return options[r.IntN(len(options))]
				}
			case "email":
				return words[r.IntN(len(words))] + "@example.com"
			case "url":
				return "https://example.com/" + words[r.IntN(len(words))]
			case "uuid":
				return uuid(r)
			}
		}
	}
	lo, hi := 0, math.MaxInt32
	if f != nil {
		if v, ok := ruleInt(f, "len"); ok {
			lo, hi = v, v
		} else {
			if v, ok := ruleInt(f, "min"); ok {
				lo = v
			}
			if v, ok := ruleInt(f, "max"); ok {
				hi = v
			}
		}
		if hasRule(f, "required") && lo < 1 {
			lo = 1
		}
	}
	count := 1 + r.IntN(3)
	parts := make([]string, count)
	for i := range parts {
		parts[i] = words[r.IntN(len(words))]
	}
	text := strings.Join(parts, " ")
	for len(text) < lo {
		text += " " + words[r.IntN(len(words))]
	}
	if len(text) > hi {
		text = strings.TrimSpace(text[:hi])
		if len(text) < lo {
			text = strings.Repeat("x", lo)
		}
	}
	return text
}

func uuid(r *rand.Rand) string {
	var b [16]byte
	for i := range b {
		b[i] = byte(r.IntN(256))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hexdigits[v>>4], hexdigits[v&0x0f])
	}
	return string(out)
}

func (g *Generator) integer(f *contract.Field, r *rand.Rand, floor, ceil int64) int64 {
	lo, hi := int64(1), int64(1000)
	if f != nil {
		if v, ok := ruleInt(f, "len"); ok {
			lo, hi = int64(v), int64(v)
		} else {
			if v, ok := ruleInt(f, "min"); ok {
				lo = int64(v)
				if hi < lo {
					hi = lo + 999
				}
			}
			if v, ok := ruleInt(f, "max"); ok {
				hi = int64(v)
				if lo > hi {
					lo = hi
				}
			}
		}
		if hasRule(f, "oneof") {
			for _, rule := range f.Rules {
				if rule.Rule == "oneof" {
					options := strings.Fields(rule.Param)
					if len(options) > 0 {
						if v, err := strconv.ParseInt(options[r.IntN(len(options))], 10, 64); err == nil {
							return v
						}
					}
				}
			}
		}
	}
	lo = max(lo, floor, -safeInteger)
	hi = min(hi, ceil, safeInteger)
	if hi < lo {
		return lo
	}
	if hi == lo {
		return lo
	}
	return lo + r.Int64N(hi-lo+1)
}

func (g *Generator) float(f *contract.Field, r *rand.Rand) float64 {
	lo, hi := 1.0, 1000.0
	if f != nil {
		if v, ok := ruleFloat(f, "len"); ok {
			return v
		}
		if v, ok := ruleFloat(f, "min"); ok {
			lo = v
			if hi < lo {
				hi = lo + 999
			}
		}
		if v, ok := ruleFloat(f, "max"); ok {
			hi = v
			if lo > hi {
				lo = hi
			}
		}
	}
	if hi <= lo {
		return lo
	}
	v := lo + r.Float64()*(hi-lo)
	return math.Round(v*100) / 100
}

func hasRule(f *contract.Field, name string) bool {
	if f == nil {
		return false
	}
	for _, r := range f.Rules {
		if r.Rule == name {
			return true
		}
	}
	return false
}

func ruleInt(f *contract.Field, name string) (int, bool) {
	if f == nil {
		return 0, false
	}
	for _, r := range f.Rules {
		if r.Rule == name {
			v, err := strconv.Atoi(r.Param)
			if err != nil {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}

func ruleFloat(f *contract.Field, name string) (float64, bool) {
	if f == nil {
		return 0, false
	}
	for _, r := range f.Rules {
		if r.Rule == name {
			v, err := strconv.ParseFloat(r.Param, 64)
			if err != nil {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}
