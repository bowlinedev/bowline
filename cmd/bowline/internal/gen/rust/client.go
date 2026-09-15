package rust

import (
	"sort"
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
	keys := make([]string, 0, len(n.children))
	for k := range n.children {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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
		b.WriteString("/// " + sub.typeName() + " groups the procedures under " + strconv.Quote(strings.Join(sub.segments, ".")) + ".\n")
		b.WriteString("pub struct " + sub.typeName() + "<'a> {\n    transport: &'a Transport,\n}\n\n")
		b.WriteString("impl<'a> " + sub.typeName() + "<'a> {")
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
		b.WriteString("\n    pub fn " + methodName(k) + "(&self) -> " + child.typeName() + "<" + lifetime + "> {\n")
		if lifetime == "'_" {
			b.WriteString("        " + child.typeName() + " {\n            transport: &self.transport,\n        }\n    }\n")
		} else {
			b.WriteString("        " + child.typeName() + " {\n            transport: self.transport,\n        }\n    }\n")
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
			b.WriteString("    #[deprecated(note = " + strconv.Quote(p.Deprecated) + ")]\n")
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
	b.WriteString("    pub async fn " + methodName(name) + "(&self, " + param + "options: Option<&CallOptions>) -> Result<" + out + ", Error> {\n")
	b.WriteString("        self.transport\n            .call(" + strconv.Quote(p.Path) + ", " + g.method(p) + ", " + arg + ", options)\n            .await" + mapping + "\n    }\n")
}

func (g *generator) writeSubscription(b *strings.Builder, name string, p *contract.Procedure) {
	g.uses["Stream"] = true
	param, arg := g.inputParam(p)
	out := g.rustType(p.Output, scope{}, false)
	b.WriteString("    pub fn " + methodName(name) + "(&self, " + param + "options: Option<&CallOptions>) -> impl Stream<Item = Result<" + out + ", Error>> + Send + 'static {\n")
	b.WriteString("        self.transport\n            .subscribe(" + strconv.Quote(p.Path) + ", " + g.method(p) + ", " + arg + ", options)\n    }\n")
}

func (g *generator) writeUpload(b *strings.Builder, name string, p *contract.Procedure) {
	g.uses["Body"] = true
	param, arg := g.inputParam(p)
	out, mapping := g.output(p)
	b.WriteString("    pub async fn " + methodName(name) + "(&self, " + param + "file: Body, filename: &str, options: Option<&CallOptions>) -> Result<" + out + ", Error> {\n")
	b.WriteString("        self.transport\n            .upload(" + strconv.Quote(p.Path) + ", " + arg + ", file, filename, options)\n            .await" + mapping + "\n    }\n")
}
