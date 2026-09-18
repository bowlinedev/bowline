package rust

import (
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

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

func (n *node) typeName() string {
	var b strings.Builder
	for _, s := range n.segments {
		b.WriteString(typeName(s))
	}
	return b.String() + "Client"
}

func sortedKeys(n *node) []string {
	keys := slices.Sorted(maps.Keys(n.children))
	return keys
}

func (g *generator) client() string {
	if len(g.doc.Procedures) == 0 {
		return ""
	}
	g.uses["Transport"] = true
	g.uses["CallOptions"] = true
	g.uses["Error"] = true
	root := buildTree(g.doc.Procedures)
	var b strings.Builder
	b.WriteString("/// Client calls the API described by the contract.\n")
	b.WriteString("pub struct Client {\n    transport: Transport,\n}\n\n")
	b.WriteString("impl Client {\n")
	b.WriteString("    pub fn new(transport: Transport) -> Self {\n        Self { transport }\n    }\n\n")
	b.WriteString("    pub fn transport(&self) -> &Transport {\n        &self.transport\n    }\n")
	g.writeAccessors(&b, root, "'_")
	g.writeMethods(&b, root)
	b.WriteString("}\n\n")
	var subs []*node
	collectMounts(root, &subs)
	for _, sub := range subs {
		b.WriteString("/// ")
		b.WriteString(sub.typeName())
		b.WriteString(" groups the procedures under ")
		b.WriteString(strconv.Quote(strings.Join(sub.segments, ".")))
		b.WriteString(".\n")
		b.WriteString("pub struct ")
		b.WriteString(sub.typeName())
		b.WriteString("<'a> {\n    transport: &'a Transport,\n}\n\n")
		b.WriteString("impl<'a> ")
		b.WriteString(sub.typeName())
		b.WriteString("<'a> {")
		g.writeAccessors(&b, sub, "'a")
		g.writeMethods(&b, sub)
		b.WriteString("}\n\n")
	}
	return b.String()
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

func (g *generator) writeAccessors(b *strings.Builder, n *node, lifetime string) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc != nil {
			continue
		}
		b.WriteString("\n    pub fn ")
		b.WriteString(methodName(k))
		b.WriteString("(&self) -> ")
		b.WriteString(child.typeName())
		b.WriteString("<")
		b.WriteString(lifetime)
		b.WriteString("> {\n")
		if lifetime == "'_" {
			b.WriteString("        ")
			b.WriteString(child.typeName())
			b.WriteString(" {\n            transport: &self.transport,\n        }\n    }\n")
		} else {
			b.WriteString("        ")
			b.WriteString(child.typeName())
			b.WriteString(" {\n            transport: self.transport,\n        }\n    }\n")
		}
	}
}

func (g *generator) writeMethods(b *strings.Builder, n *node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			continue
		}
		p := child.proc
		b.WriteString("\n")
		writeDoc(b, "    ", p.Doc)
		if p.Deprecated != "" {
			b.WriteString("    #[deprecated(note = ")
			b.WriteString(strconv.Quote(p.Deprecated))
			b.WriteString(")]\n")
		}
		switch p.Kind {
		case "subscription":
			g.writeSubscription(b, k, p)
		case "upload":
			g.writeUpload(b, k, p)
		default:
			g.writeCall(b, k, p)
		}
	}
}

func (g *generator) inputParam(p *contract.Procedure) (param, arg string) {
	if isEmptyStruct(p.Input) {
		g.uses["Empty"] = true
		return "", "&Empty {}"
	}
	return "input: &" + g.rustType(p.Input, scope{}, false) + ", ", "input"
}

func (g *generator) method(p *contract.Procedure) string {
	g.uses["Method"] = true
	switch p.Method {
	case http.MethodGet:
		return "Method::Get"
	case http.MethodPut:
		return "Method::Put"
	case http.MethodPatch:
		return "Method::Patch"
	case http.MethodDelete:
		return "Method::Delete"
	}
	return "Method::Post"
}

func pathParams(p *contract.Procedure) []string {
	if p.HTTPPath == "" {
		return nil
	}
	names, err := contract.PathParams(p.HTTPPath)
	if err != nil {
		return nil
	}
	return names
}

func dropped(p *contract.Procedure) string {
	params := pathParams(p)
	names := make([]string, len(params))
	for i, name := range params {
		names[i] = strconv.Quote(name)
	}
	return "&[" + strings.Join(names, ", ") + "]"
}

func (g *generator) pathExpr(p *contract.Procedure) (string, bool) {
	if p.HTTPPath == "" {
		return strconv.Quote(p.Path), false
	}
	segments, err := contract.ParsePath(p.HTTPPath)
	if err != nil {
		return strconv.Quote(p.Path), false
	}
	var template strings.Builder
	var args []string
	for i, seg := range segments {
		if i > 0 {
			template.WriteString("/")
		}
		if !seg.Param {
			template.WriteString(seg.Text)
			continue
		}
		template.WriteString("{}")
		g.uses["encode_segment"] = true
		args = append(args, "encode_segment(&input."+fieldName(seg.Text)+")")
	}
	if len(args) == 0 {
		return strconv.Quote(template.String()), true
	}
	return "&format!(" + strconv.Quote(template.String()) + ", " + strings.Join(args, ", ") + ")", true
}

func writeTransportCall(b *strings.Builder, fn string, args []string) {
	b.WriteString("        self.transport\n            .")
	b.WriteString(fn)
	line := "(" + strings.Join(args, ", ") + ")"
	if len("            ."+fn+line) <= 100 {
		b.WriteString(line)
		return
	}
	b.WriteString("(\n")
	for _, arg := range args {
		b.WriteString("                ")
		b.WriteString(arg)
		b.WriteString(",\n")
	}
	b.WriteString("            )")
}

func (g *generator) output(p *contract.Procedure) (typ, mapping string) {
	if isEmptyStruct(p.Output) {
		g.uses["Empty"] = true
		return "()", "\n            .map(|_: Empty| ())"
	}
	return g.rustType(p.Output, scope{}, false), ""
}

func (g *generator) writeCall(b *strings.Builder, name string, p *contract.Procedure) {
	param, arg := g.inputParam(p)
	out, mapping := g.output(p)
	b.WriteString("    pub async fn ")
	b.WriteString(methodName(name))
	b.WriteString("(&self, ")
	b.WriteString(param)
	b.WriteString("options: Option<&CallOptions>) -> Result<")
	b.WriteString(out)
	b.WriteString(", Error> {\n")
	path, rest := g.pathExpr(p)
	if rest {
		writeTransportCall(b, "call_rest", []string{path, g.method(p), arg, dropped(p), "options"})
	} else {
		writeTransportCall(b, "call", []string{path, g.method(p), arg, "options"})
	}
	b.WriteString("\n            .await")
	b.WriteString(mapping)
	b.WriteString("\n    }\n")
}

func (g *generator) writeSubscription(b *strings.Builder, name string, p *contract.Procedure) {
	g.uses["Stream"] = true
	param, arg := g.inputParam(p)
	out := g.rustType(p.Output, scope{}, false)
	b.WriteString("    pub fn ")
	b.WriteString(methodName(name))
	b.WriteString("(&self, ")
	b.WriteString(param)
	b.WriteString("options: Option<&CallOptions>) -> impl Stream<Item = Result<")
	b.WriteString(out)
	b.WriteString(", Error>> + Send + 'static {\n")
	path, rest := g.pathExpr(p)
	if rest {
		writeTransportCall(b, "subscribe_rest", []string{path, g.method(p), arg, dropped(p), "options"})
	} else {
		writeTransportCall(b, "subscribe", []string{path, g.method(p), arg, "options"})
	}
	b.WriteString("\n    }\n")
}

func (g *generator) writeUpload(b *strings.Builder, name string, p *contract.Procedure) {
	g.uses["Body"] = true
	param, arg := g.inputParam(p)
	out, mapping := g.output(p)
	b.WriteString("    pub async fn ")
	b.WriteString(methodName(name))
	b.WriteString("(&self, ")
	b.WriteString(param)
	b.WriteString("file: Body, filename: &str, options: Option<&CallOptions>) -> Result<")
	b.WriteString(out)
	b.WriteString(", Error> {\n")
	path, rest := g.pathExpr(p)
	if rest {
		writeTransportCall(b, "upload_rest", []string{path, arg, dropped(p), "file", "filename", "options"})
	} else {
		writeTransportCall(b, "upload", []string{path, arg, "file", "filename", "options"})
	}
	b.WriteString("\n            .await")
	b.WriteString(mapping)
	b.WriteString("\n    }\n")
}
