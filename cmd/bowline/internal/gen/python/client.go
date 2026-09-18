package python

import (
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/naming"
	"github.com/bowlinedev/bowline/contract"
)

type node struct {
	segments []string
	children map[string]*node
	proc     *contract.Procedure
}

func buildTree(procs []*contract.Procedure) *node {
	root := &node{children: map[string]*node{}}
	for _, p := range procs {
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
	return root
}

func (n *node) className(prefix string) string {
	if len(n.segments) == 0 {
		return prefix + "Client"
	}
	var b strings.Builder
	for _, s := range n.segments {
		b.WriteString(naming.UpperCamel(s))
	}
	return prefix + b.String() + "Client"
}

func sortedKeys(n *node) []string {
	keys := slices.Sorted(maps.Keys(n.children))
	return keys
}

func collectMounts(n *node, out *[]*node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			*out = append(*out, child)
			collectMounts(child, out)
		}
	}
}

type flavor struct {
	prefix    string
	transport string
	async     bool
}

var flavors = []flavor{{"", "Transport", true}, {"Sync", "SyncTransport", false}}

func (g *generator) clients() string {
	root := buildTree(g.doc.Procedures)
	var mounts []*node
	collectMounts(root, &mounts)
	var b strings.Builder
	for _, fl := range flavors {
		for _, n := range mounts {
			g.writeClass(&b, n, fl)
		}
		g.writeClass(&b, root, fl)
	}
	return b.String()
}

func (g *generator) writeClass(b *strings.Builder, n *node, fl flavor) {
	name := n.className(fl.prefix)
	b.WriteString("class ")
	b.WriteString(name)
	b.WriteString(":\n")
	if len(n.segments) == 0 {
		b.WriteString("    \"\"\"")
		b.WriteString(fl.transport)
		b.WriteString(" client for every procedure in the contract.\"\"\"\n\n")
	} else {
		b.WriteString("    \"\"\"Procedures under ")
		b.WriteString(strconv.Quote(strings.Join(n.segments, ".")))
		b.WriteString(".\"\"\"\n\n")
	}
	b.WriteString("    def __init__(self, transport: ")
	b.WriteString(fl.transport)
	b.WriteString(") -> None:\n")
	b.WriteString("        self._transport = transport\n")
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			b.WriteString("        self.")
			b.WriteString(identifier(k))
			b.WriteString(" = ")
			b.WriteString(child.className(fl.prefix))
			b.WriteString("(transport)\n")
		}
	}
	b.WriteString("\n")
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			continue
		}
		g.writeMethod(b, identifier(k), child.proc, fl)
	}
	b.WriteString("\n")
}

func (g *generator) writeMethod(b *strings.Builder, name string, p *contract.Procedure, fl flavor) {
	owner := naming.UpperCamel(strings.ReplaceAll(p.Path, ".", "_"))
	input := g.pyType(p.Input, owner+"Input")
	output := g.pyType(p.Output, owner+"Output")
	inputParam := "input: " + input
	inputArg := "input"
	if input == "Empty" {
		inputParam = "input: Empty | None = None"
		inputArg = "input or Empty()"
	}
	method := methodConst(p.Method)
	def := "def"
	await := ""
	if fl.async {
		def = "async def"
		await = "await "
	}
	path := g.pathArg(p)
	drop := dropArg(p)
	var params []string
	var call string
	var returns string
	switch p.Kind {
	case "subscription":
		iter := "Iterator"
		if fl.async {
			iter = "AsyncIterator"
		}
		g.uses[iter] = true
		def = "def"
		params = []string{"self", inputParam, "options: CallOptions | None = None"}
		returns = iter + "[" + output + "]"
		args := []string{path, inputArg, output, "options"}
		if p.Method != http.MethodGet {
			args = append(args, "method="+method)
		}
		if p.HTTPPath != "" {
			args = append(args, "rest=True")
		}
		if drop != "" {
			args = append(args, drop)
		}
		call = "self._transport.subscribe(" + strings.Join(args, ", ") + ")"
	case "upload":
		g.uses["BinaryIO"] = true
		params = []string{"self", inputParam, "file: BinaryIO", "filename: str", "options: CallOptions | None = None"}
		returns = output
		uploadArgs := []string{path, inputArg, "file", "filename", output, "options"}
		if drop != "" {
			uploadArgs = append(uploadArgs, drop)
		}
		call = await + "self._transport.upload(" + strings.Join(uploadArgs, ", ") + ")"
	default:
		params = []string{"self", inputParam, "options: CallOptions | None = None"}
		returns = output
		callArgs := []string{path, method, inputArg, output, "options"}
		if p.HTTPPath != "" {
			callArgs = append(callArgs, "rest=True")
		}
		if drop != "" {
			callArgs = append(callArgs, drop)
		}
		call = await + "self._transport.call(" + strings.Join(callArgs, ", ") + ")"
	}
	signature := "    " + def + " " + name + "(" + strings.Join(params, ", ") + ") -> " + returns + ":"
	if len(signature) > 100 {
		signature = "    " + def + " " + name + "(\n        " + strings.Join(params, ",\n        ") + ",\n    ) -> " + returns + ":"
	}
	b.WriteString(signature)
	b.WriteString("\n")
	docstring(b, "        ", p.Doc, p.Deprecated)
	line := "        return " + call
	if len(line) > 100 {
		open := strings.Index(call, "(")
		args := splitArgs(call[open+1 : len(call)-1])
		joined := strings.Join(args, ", ")
		if len(joined)+12 > 100 {
			joined = strings.Join(args, ",\n            ") + ","
		}
		line = "        return " + call[:open+1] + "\n            " + joined + "\n        )"
	}
	b.WriteString(line)
	b.WriteString("\n\n")
}

func splitArgs(args string) []string {
	var out []string
	var quote byte
	depth := 0
	start := 0
	for i := 0; i < len(args); i++ {
		c := args[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			out = append(out, strings.TrimSpace(args[start:i]))
			start = i + 1
		}
	}
	return append(out, strings.TrimSpace(args[start:]))
}

func methodConst(method string) string {
	switch method {
	case http.MethodGet:
		return "Method.GET"
	case http.MethodPut:
		return "Method.PUT"
	case http.MethodPatch:
		return "Method.PATCH"
	case http.MethodDelete:
		return "Method.DELETE"
	}
	return "Method.POST"
}

func dropArg(p *contract.Procedure) string {
	if p.HTTPPath == "" {
		return ""
	}
	names, err := contract.PathParams(p.HTTPPath)
	if err != nil || len(names) == 0 {
		return ""
	}
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = strconv.Quote(name)
	}
	tail := ""
	if len(quoted) == 1 {
		tail = ","
	}
	return "drop=(" + strings.Join(quoted, ", ") + tail + ")"
}

func (g *generator) pathArg(p *contract.Procedure) string {
	if p.HTTPPath == "" {
		return strconv.Quote(p.Path)
	}
	segments, err := contract.ParsePath(p.HTTPPath)
	if err != nil {
		return strconv.Quote(p.Path)
	}
	var b strings.Builder
	interpolated := false
	for i, seg := range segments {
		if i > 0 {
			b.WriteString("/")
		}
		if !seg.Param {
			b.WriteString(seg.Text)
			continue
		}
		interpolated = true
		g.uses["quote"] = true
		b.WriteString("{quote(str(input.")
		b.WriteString(attrName(seg.Text))
		b.WriteString("), safe='')}")
	}
	if !interpolated {
		return strconv.Quote(b.String())
	}
	return "f" + strconv.Quote(b.String())
}

type Sample struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

func Models(doc *contract.Document) map[string]Sample {
	gen := newGenerator(doc)
	out := map[string]Sample{}
	for _, p := range doc.Procedures {
		owner := naming.UpperCamel(strings.ReplaceAll(p.Path, ".", "_"))
		out[p.Path] = Sample{Model: gen.pyType(p.Output, owner+"Output"), Input: gen.pyType(p.Input, owner+"Input")}
	}
	return out
}
