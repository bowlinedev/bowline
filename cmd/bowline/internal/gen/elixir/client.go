package elixir

import (
	"sort"
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

func sortedKeys(n *node) []string {
	keys := make([]string, 0, len(n.children))
	for k := range n.children {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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

func (g *generator) writeClients(b *strings.Builder) {
	root := buildTree(g.doc.Procedures)
	var mounts []*node
	collectMounts(root, &mounts)
	for _, mount := range mounts {
		module := segmentsModule(g.root, mount.segments)
		body := &strings.Builder{}
		g.helpers = helperPool{names: map[string]bool{}}
		g.writeProcedures(body, mount)
		g.appendHelpers(body)
		g.emitModule(b, module, "  @moduledoc \"Procedures under "+strings.Join(mount.segments, ".")+".\"\n\n", strings.TrimPrefix(body.String(), "\n"))
	}
	doc := &strings.Builder{}
	doc.WriteString("  @moduledoc \"\"\"\n")
	doc.WriteString("  Entry point for the API: `new/2` builds the transport every module takes first.\n")
	if len(mounts) > 0 {
		doc.WriteString("\n  Mounted modules:\n")
		for _, mount := range mounts {
			doc.WriteString("  - `" + segmentsModule(g.root, mount.segments) + "`\n")
		}
	}
	doc.WriteString("  \"\"\"\n\n")
	body := &strings.Builder{}
	g.helpers = helperPool{names: map[string]bool{}}
	body.WriteString("  @spec new(String.t(), keyword()) :: Transport.t()\n")
	body.WriteString("  def new(base_url, opts \\\\ []) do\n")
	body.WriteString("    Transport.new(Keyword.put(opts, :base_url, base_url))\n")
	body.WriteString("  end\n")
	g.writeProcedures(body, root)
	if len(g.errorOrder) > 0 {
		body.WriteString("\n  @doc \"Decodes the details of a declared error variant into its struct; nil for other errors.\"\n")
		body.WriteString("  @spec error_details(BowlineClient.Error.t()) :: struct() | nil\n")
		for _, id := range g.errorOrder {
			decl := g.doc.Errors[id]
			body.WriteString("  def error_details(%BowlineClient.Error{type: " + quote(decl.Name) + ", details: details})\n")
			body.WriteString("      when is_map(details) do\n")
			body.WriteString("    Types." + g.names[id] + ".from_map(details)\n")
			body.WriteString("  end\n\n")
		}
		body.WriteString("  def error_details(%BowlineClient.Error{}), do: nil\n")
	}
	g.appendHelpers(body)
	g.emitModule(b, g.root+".Client", doc.String(), body.String())
}

func (g *generator) appendHelpers(body *strings.Builder) {
	for _, def := range g.helpers.defs {
		body.WriteString("\n" + def)
	}
}

func (g *generator) writeProcedures(b *strings.Builder, n *node) {
	for _, k := range sortedKeys(n) {
		child := n.children[k]
		if child.proc == nil {
			continue
		}
		p := child.proc
		name := functionName(k)
		b.WriteString("\n")
		doc := docOr(p.Doc, name+" calls "+p.Path+".")
		if p.Deprecated != "" {
			doc += "\n\nDeprecated: " + p.Deprecated
		}
		writeModuleDoc(b, "  ", "@doc", doc)
		switch p.Kind {
		case "subscription":
			g.writeSubscription(b, name, p)
		case "upload":
			g.writeUpload(b, name, p)
		default:
			g.writeCall(b, name, p)
		}
	}
}

func (g *generator) inputSpec(p *contract.Procedure) string {
	if isEmptyStruct(p.Input) {
		return ""
	}
	return g.typeSpec(p.Input, g.types, "input")
}

func (g *generator) outputSpec(p *contract.Procedure) string {
	if isEmptyStruct(p.Output) {
		return "BowlineClient.Empty.t()"
	}
	return g.typeSpec(p.Output, g.types, "output")
}

func (g *generator) inputEncode(p *contract.Procedure) string {
	if isEmptyStruct(p.Input) {
		return "%{}"
	}
	return g.encodeBare(p.Input, g.types, "input", "input")
}

func (g *generator) outputDecoder(p *contract.Procedure) string {
	if isEmptyStruct(p.Output) {
		return "&BowlineClient.Empty.from_map/1"
	}
	return g.decoderFn(p.Output, g.types, "output", p.Path)
}

func (g *generator) inputPattern(p *contract.Procedure) string {
	if p.Input.Kind == contract.Ref {
		if decl, ok := g.doc.Types[p.Input.ID]; ok && decl.Kind == contract.Struct {
			return "%" + "Types." + g.names[p.Input.ID] + "{} = input"
		}
	}
	return "input"
}

func writeSpec(b *strings.Builder, name string, args []string, result string) {
	head := name + "(" + strings.Join(args, ", ") + ")"
	line := "  @spec " + head + " :: " + result
	if len(line) <= lineWidth {
		b.WriteString(line + "\n")
		return
	}
	if len("  @spec "+head+" ::") <= lineWidth {
		b.WriteString("  @spec " + head + " ::\n          " + result + "\n")
		return
	}
	b.WriteString("  @spec " + name + "(\n")
	for _, a := range args {
		b.WriteString("          " + a + ",\n")
	}
	trimComma(b)
	b.WriteString("\n        ) ::\n          " + result + "\n")
}

func writeArgs(b *strings.Builder, indent, call string, args []string) {
	line := indent + call + "(" + strings.Join(args, ", ") + ")"
	if len(line) <= lineWidth {
		b.WriteString(line + "\n")
		return
	}
	b.WriteString(indent + call + "(\n")
	for _, a := range args {
		b.WriteString(indent + "  " + a + ",\n")
	}
	trimComma(b)
	b.WriteString("\n" + indent + ")\n")
}

func (g *generator) writeCall(b *strings.Builder, name string, p *contract.Procedure) {
	method := ":post"
	if p.Method == "GET" {
		method = ":get"
	}
	result := "{:ok, " + g.outputSpec(p) + "} | {:error, BowlineClient.Error.t()}"
	if isEmptyStruct(p.Input) {
		writeSpec(b, name, []string{"Transport.t()", "Transport.call_opts()"}, result)
		b.WriteString("  def " + name + "(transport, opts \\\\ []) do\n")
		writeArgs(b, "    ", "Transport.call", []string{"transport", quote(p.Path), method, "%{}", g.outputDecoder(p), "opts"})
		b.WriteString("  end\n\n")
		writeSpec(b, name+"!", []string{"Transport.t()", "Transport.call_opts()"}, g.outputSpec(p))
		b.WriteString("  def " + name + "!(transport, opts \\\\ []) do\n")
		b.WriteString("    case " + name + "(transport, opts) do\n")
		b.WriteString("      {:ok, value} -> value\n      {:error, error} -> raise error\n    end\n  end\n")
		return
	}
	writeSpec(b, name, []string{"Transport.t()", g.inputSpec(p), "Transport.call_opts()"}, result)
	b.WriteString("  def " + name + "(transport, " + g.inputPattern(p) + ", opts \\\\ []) do\n")
	writeArgs(b, "    ", "Transport.call", []string{"transport", quote(p.Path), method, g.inputEncode(p), g.outputDecoder(p), "opts"})
	b.WriteString("  end\n\n")
	writeSpec(b, name+"!", []string{"Transport.t()", g.inputSpec(p), "Transport.call_opts()"}, g.outputSpec(p))
	b.WriteString("  def " + name + "!(transport, input, opts \\\\ []) do\n")
	b.WriteString("    case " + name + "(transport, input, opts) do\n")
	b.WriteString("      {:ok, value} -> value\n      {:error, error} -> raise error\n    end\n  end\n")
}

func (g *generator) writeSubscription(b *strings.Builder, name string, p *contract.Procedure) {
	result := "{:ok, Enumerable.t()} | {:error, BowlineClient.Error.t()}"
	if isEmptyStruct(p.Input) {
		writeSpec(b, name, []string{"Transport.t()", "Transport.call_opts()"}, result)
		b.WriteString("  def " + name + "(transport, opts \\\\ []) do\n")
		writeArgs(b, "    ", "Transport.subscribe", []string{"transport", quote(p.Path), "%{}", g.outputDecoder(p), "opts"})
		b.WriteString("  end\n")
		return
	}
	writeSpec(b, name, []string{"Transport.t()", g.inputSpec(p), "Transport.call_opts()"}, result)
	b.WriteString("  def " + name + "(transport, " + g.inputPattern(p) + ", opts \\\\ []) do\n")
	writeArgs(b, "    ", "Transport.subscribe", []string{"transport", quote(p.Path), g.inputEncode(p), g.outputDecoder(p), "opts"})
	b.WriteString("  end\n")
}

func (g *generator) writeUpload(b *strings.Builder, name string, p *contract.Procedure) {
	result := "{:ok, " + g.outputSpec(p) + "} | {:error, BowlineClient.Error.t()}"
	input := g.inputSpec(p)
	if input == "" {
		input = "BowlineClient.Empty.t()"
	}
	writeSpec(b, name, []string{"Transport.t()", input, "Enumerable.t() | binary()", "String.t()", "Transport.call_opts()"}, result)
	b.WriteString("  def " + name + "(transport, " + g.inputPattern(p) + ", file, filename, opts \\\\ []) do\n")
	writeArgs(b, "    ", "Transport.upload", []string{"transport", quote(p.Path), g.inputEncode(p), "file", "filename", g.outputDecoder(p), "opts"})
	b.WriteString("  end\n")
}

func isEmptyStruct(t *contract.Type) bool {
	return t != nil && t.Kind == contract.Struct && len(t.Fields) == 0
}
