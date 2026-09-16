package dart

import (
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type node struct {
	segments []string
	children map[string]*node
	proc     *contract.Procedure
	name     string
}

func (g *generator) buildTree() *node {
	root := &node{children: map[string]*node{}, name: "Client"}
	for _, p := range g.doc.Procedures {
		current := root
		segments := strings.Split(p.Path, ".")
		for i, s := range segments[:len(segments)-1] {
			child, ok := current.children[s]
			if !ok {
				child = &node{segments: segments[:i+1], children: map[string]*node{}}
				current.children[s] = child
			}
			current = child
		}
		current.children[segments[len(segments)-1]] = &node{segments: segments, proc: p}
	}
	g.nameMounts(root)
	return root
}

func (g *generator) nameMounts(n *node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			child.name = g.uniqueType(procTypeName(strings.Join(child.segments, ".")) + "Client")
			g.nameMounts(child)
		}
	}
}

func sortedKeys(n *node) []string {
	keys := slices.Sorted(maps.Keys(n.children))
	return keys
}

func (g *generator) clients(b *strings.Builder) {
	root := g.buildTree()
	var mounts []*node
	collectMounts(root, &mounts)
	for _, m := range mounts {
		g.writeClient(b, m)
	}
	g.writeClient(b, root)
}

func collectMounts(n *node, out *[]*node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			collectMounts(child, out)
			*out = append(*out, child)
		}
	}
}

func (g *generator) writeClient(b *strings.Builder, n *node) {
	members := map[string]string{}
	used := map[string]bool{}
	for _, k := range sortedKeys(n) {
		name := fieldName(k)
		for used[name] {
			name += "_"
		}
		used[name] = true
		members[k] = name
	}
	b.WriteString("class ")
	b.WriteString(n.name)
	b.WriteString(" {\n")
	var inits []string
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			inits = append(inits, members[k]+" = "+child.name+"(transport)")
		}
	}
	if len(inits) == 0 {
		b.WriteString("  ")
		b.WriteString(n.name)
		b.WriteString("(this._transport);\n")
	} else {
		b.WriteString("  ")
		b.WriteString(n.name)
		b.WriteString("(Transport transport)\n      : _transport = transport,\n        ")
		b.WriteString(strings.Join(inits, ",\n        "))
		b.WriteString(";\n")
	}
	b.WriteString("\n  final Transport _transport;\n")
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			b.WriteString("\n  final ")
			b.WriteString(child.name)
			b.WriteString(" ")
			b.WriteString(members[k])
			b.WriteString(";\n")
		}
	}
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc != nil {
			g.writeMethod(b, child.proc, members[k])
		}
	}
	b.WriteString("}\n\n")
}

func (g *generator) writeMethod(b *strings.Builder, p *contract.Procedure, name string) {
	b.WriteString("\n")
	writeDoc(b, "  ", p.Doc)
	if p.Deprecated != "" {
		b.WriteString("  @Deprecated(")
		b.WriteString(quote(p.Deprecated))
		b.WriteString(")\n")
	}
	method := "Method.post"
	if p.Method == http.MethodGet {
		method = "Method.get"
	}
	emptyIn := isEmptyStruct(p.Input)
	emptyOut := isEmptyStruct(p.Output)
	out := "void"
	decode := "(_) {}"
	if !emptyOut {
		out = g.dartType(p.Output)
		decode = "(json) => " + g.decodeBody(p.Output, "output", "json")
	}
	params := []string{}
	input := "const {}"
	if !emptyIn {
		params = append(params, g.dartType(p.Input)+" input")
		input = "input.toJson(" + g.toJsonArgs(p.Input) + ")"
	}
	call := "call"
	switch p.Kind {
	case "subscription":
		call = "subscribe"
	case "upload":
		call = "upload"
		params = append(params, "Stream<List<int>> file", "String filename")
	}
	params = append(params, "{CallOptions? options}")
	returnType := "Future<" + out + ">"
	if p.Kind == "subscription" {
		returnType = "Stream<" + out + ">"
	}
	b.WriteString("  ")
	b.WriteString(returnType)
	b.WriteString(" ")
	b.WriteString(name)
	b.WriteString("(")
	b.WriteString(strings.Join(params, ", "))
	b.WriteString(") {\n")
	if !emptyIn {
		b.WriteString("    ensureValid(input.validate(")
		b.WriteString(g.validateArgs(p.Input))
		b.WriteString("));\n")
	}
	args := []string{quote(p.Path)}
	switch p.Kind {
	case "subscription":
		args = append(args, input, decode)
	case "upload":
		args = append(args, input, "file", "filename", decode)
	default:
		args = append(args, method, input, decode)
	}
	typeArg := ""
	if emptyOut {
		typeArg = "<void>"
	}
	b.WriteString("    return _transport.")
	b.WriteString(call)
	b.WriteString(typeArg)
	b.WriteString("(")
	b.WriteString(strings.Join(args, ", "))
	b.WriteString(", options: options);\n")
	b.WriteString("  }\n")
}

func (g *generator) toJsonArgs(t *contract.Type) string {
	if t.Kind != contract.Ref || len(t.Args) == 0 {
		return ""
	}
	args := make([]string, len(t.Args))
	for i, a := range t.Args {
		args[i] = "(e0) => " + g.encode(a, "e0", 1)
	}
	return strings.Join(args, ", ")
}

func (g *generator) validateArgs(t *contract.Type) string {
	if t.Kind != contract.Ref || len(t.Args) == 0 {
		return ""
	}
	args := make([]string, len(t.Args))
	for i, a := range t.Args {
		inner := g.nested(a, "v0", nil, 1)
		if len(inner) == 0 {
			args[i] = "(v0) => const []"
		} else {
			args[i] = "(v0) => [" + strings.Join(inner, ", ") + "]"
		}
	}
	return strings.Join(args, ", ")
}

func isEmptyStruct(t *contract.Type) bool {
	return t != nil && t.Kind == contract.Struct && len(t.Fields) == 0
}
