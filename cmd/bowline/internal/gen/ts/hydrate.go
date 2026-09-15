package ts

import (
	"sort"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type hydrateEntry struct {
	path []string
	kind string
	ref  string
}

func (g *generator) hydrator(t *contract.Type) string {
	if t.Kind != contract.Ref {
		return ""
	}
	key := g.tsType(t, false)
	if _, seen := g.hydrators[key]; seen {
		return key
	}
	g.hydrators[key] = nil
	decl := g.doc.Types[t.ID]
	var entries []hydrateEntry
	switch decl.Kind {
	case contract.Struct:
		for _, f := range decl.Fields {
			g.collect(f.Type, nil, []string{f.Name}, &entries)
		}
	case contract.Generic:
		env := map[string]*contract.Type{}
		for i, p := range decl.Params {
			if i < len(t.Args) {
				env[p] = t.Args[i]
			}
		}
		for _, f := range decl.Body.Fields {
			g.collect(f.Type, env, []string{f.Name}, &entries)
		}
	}
	g.hydrators[key] = entries
	return key
}

func (g *generator) collect(t *contract.Type, env map[string]*contract.Type, path []string, out *[]hydrateEntry) {
	switch t.Kind {
	case contract.Primitive:
		if t.Name == "timestamp" {
			*out = append(*out, hydrateEntry{path: clone(path), kind: "timestamp"})
		} else if t.Encoding == "string" {
			*out = append(*out, hydrateEntry{path: clone(path), kind: "bigint"})
		}
	case contract.Ref:
		resolved := substitute(t, env)
		key := g.hydrator(resolved)
		*out = append(*out, hydrateEntry{path: clone(path), kind: "ref", ref: key})
	case contract.Array:
		g.collect(t.Elem, env, append(path, "*"), out)
	case contract.Map:
		g.collect(t.Value, env, append(path, "*"), out)
	case contract.Struct:
		for _, f := range t.Fields {
			g.collect(f.Type, env, append(path, f.Name), out)
		}
	case contract.Param:
		if bound, ok := env[t.Name]; ok {
			g.collect(bound, nil, path, out)
		}
	}
}

func substitute(t *contract.Type, env map[string]*contract.Type) *contract.Type {
	if len(env) == 0 || t == nil {
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

func clone(path []string) []string {
	out := make([]string, len(path))
	copy(out, path)
	return out
}

func (g *generator) pruneHydrators() map[string][]hydrateEntry {
	live := map[string]bool{}
	for key, entries := range g.hydrators {
		for _, e := range entries {
			if e.kind != "ref" {
				live[key] = true
				break
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for key, entries := range g.hydrators {
			if live[key] {
				continue
			}
			for _, e := range entries {
				if e.kind == "ref" && live[e.ref] {
					live[key] = true
					changed = true
					break
				}
			}
		}
	}
	out := map[string][]hydrateEntry{}
	for key, entries := range g.hydrators {
		if !live[key] {
			continue
		}
		var kept []hydrateEntry
		for _, e := range entries {
			if e.kind != "ref" || live[e.ref] {
				kept = append(kept, e)
			}
		}
		out[key] = kept
	}
	return out
}

func (g *generator) runtimeTable() string {
	for _, p := range g.doc.Procedures {
		g.hydrator(p.Output)
	}
	hydrators := g.pruneHydrators()
	keys := make([]string, 0, len(hydrators))
	for k := range hydrators {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("export const contract = {\n")
	b.WriteString("  version: " + strconv.Quote(g.doc.Bowline) + ",\n")
	if len(keys) == 0 {
		b.WriteString("  hydrators: {},\n")
	} else {
		b.WriteString("  hydrators: {\n")
	}
	for _, k := range keys {
		b.WriteString("    " + strconv.Quote(k) + ": [\n")
		for _, e := range hydrators[k] {
			parts := make([]string, len(e.path))
			for i, p := range e.path {
				parts[i] = strconv.Quote(p)
			}
			kind := strconv.Quote(e.kind)
			if e.kind == "ref" {
				kind = "{ ref: " + strconv.Quote(e.ref) + " }"
			}
			b.WriteString("      { path: [" + strings.Join(parts, ", ") + "], kind: " + kind + " },\n")
		}
		b.WriteString("    ],\n")
	}
	if len(keys) > 0 {
		b.WriteString("  },\n")
	}
	b.WriteString("  procedures: {\n")
	for _, p := range g.doc.Procedures {
		line := "    " + strconv.Quote(p.Path) + ": { kind: " + strconv.Quote(p.Kind) + ", method: " + strconv.Quote(p.Method)
		if key := g.tsType(p.Output, false); p.Output.Kind == contract.Ref && len(hydrators[key]) > 0 {
			line += ", output: " + strconv.Quote(key)
		}
		b.WriteString(line + " },\n")
	}
	b.WriteString("  },\n")
	b.WriteString("} satisfies ContractRuntime;\n\n")
	return b.String()
}
