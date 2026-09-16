package rust

import (
	"maps"
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
	if p.Method == "GET" {
		return "Method::Get"
	}
	return "Method::Post"
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
	b.WriteString("        self.transport\n            .call(")
	b.WriteString(strconv.Quote(p.Path))
	b.WriteString(", ")
	b.WriteString(g.method(p))
	b.WriteString(", ")
	b.WriteString(arg)
	b.WriteString(", options)\n            .await")
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
	b.WriteString("        self.transport\n            .subscribe(")
	b.WriteString(strconv.Quote(p.Path))
	b.WriteString(", ")
	b.WriteString(g.method(p))
	b.WriteString(", ")
	b.WriteString(arg)
	b.WriteString(", options)\n    }\n")
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
	b.WriteString("        self.transport\n            .upload(")
	b.WriteString(strconv.Quote(p.Path))
	b.WriteString(", ")
	b.WriteString(arg)
	b.WriteString(", file, filename, options)\n            .await")
	b.WriteString(mapping)
	b.WriteString("\n    }\n")
}
