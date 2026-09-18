package elixir

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
			doc.WriteString("  - `")
			doc.WriteString(segmentsModule(g.root, mount.segments))
			doc.WriteString("`\n")
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
			body.WriteString("  def error_details(%BowlineClient.Error{type: ")
			body.WriteString(quote(decl.Name))
			body.WriteString(", details: details})\n")
			body.WriteString("      when is_map(details) do\n")
			body.WriteString("    Types.")
			body.WriteString(g.names[id])
			body.WriteString(".from_map(details)\n")
			body.WriteString("  end\n\n")
		}
		body.WriteString("  def error_details(%BowlineClient.Error{}), do: nil\n")
	}
	g.appendHelpers(body)
	g.emitModule(b, g.root+".Client", doc.String(), body.String())
}

func (g *generator) appendHelpers(body *strings.Builder) {
	for _, def := range g.helpers.defs {
		body.WriteString("\n")
		body.WriteString(def)
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
	encoded := g.encodeBare(p.Input, g.types, "input", "input")
	params := pathParams(p)
	if len(params) == 0 {
		return encoded
	}
	quoted := make([]string, len(params))
	for i, name := range params {
		quoted[i] = quote(name)
	}
	return "Map.drop(" + encoded + ", [" + strings.Join(quoted, ", ") + "])"
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
		b.WriteString(line)
		b.WriteString("\n")
		return
	}
	if len("  @spec "+head+" ::") <= lineWidth {
		b.WriteString("  @spec ")
		b.WriteString(head)
		b.WriteString(" ::\n          ")
		b.WriteString(result)
		b.WriteString("\n")
		return
	}
	b.WriteString("  @spec ")
	b.WriteString(name)
	b.WriteString("(\n")
	for _, a := range args {
		b.WriteString("          ")
		b.WriteString(a)
		b.WriteString(",\n")
	}
	trimComma(b)
	b.WriteString("\n        ) ::\n          ")
	b.WriteString(result)
	b.WriteString("\n")
}

func writeArgs(b *strings.Builder, call string, args []string) {
	const indent = "    "
	line := indent + call + "(" + strings.Join(args, ", ") + ")"
	if len(line) <= lineWidth {
		b.WriteString(line)
		b.WriteString("\n")
		return
	}
	b.WriteString(indent)
	b.WriteString(call)
	b.WriteString("(\n")
	for _, a := range args {
		b.WriteString(indent)
		b.WriteString("  ")
		b.WriteString(a)
		b.WriteString(",\n")
	}
	trimComma(b)
	b.WriteString("\n")
	b.WriteString(indent)
	b.WriteString(")\n")
}

func (g *generator) writeCall(b *strings.Builder, name string, p *contract.Procedure) {
	method := methodAtom(p.Method)
	call := "Transport.call"
	if queryPath(p) {
		call = "Transport.rest"
	}
	result := "{:ok, " + g.outputSpec(p) + "} | {:error, BowlineClient.Error.t()}"
	if isEmptyStruct(p.Input) {
		writeSpec(b, name, []string{"Transport.t()", "Transport.call_opts()"}, result)
		b.WriteString("  def ")
		b.WriteString(name)
		b.WriteString("(transport, opts \\\\ []) do\n")
		writeArgs(b, call, []string{"transport", pathArg(p), method, "%{}", g.outputDecoder(p), "opts"})
		b.WriteString("  end\n\n")
		writeSpec(b, name+"!", []string{"Transport.t()", "Transport.call_opts()"}, g.outputSpec(p))
		b.WriteString("  def ")
		b.WriteString(name)
		b.WriteString("!(transport, opts \\\\ []) do\n")
		b.WriteString("    case ")
		b.WriteString(name)
		b.WriteString("(transport, opts) do\n")
		b.WriteString("      {:ok, value} -> value\n      {:error, error} -> raise error\n    end\n  end\n")
		return
	}
	writeSpec(b, name, []string{"Transport.t()", g.inputSpec(p), "Transport.call_opts()"}, result)
	b.WriteString("  def ")
	b.WriteString(name)
	b.WriteString("(transport, ")
	b.WriteString(g.inputPattern(p))
	b.WriteString(", opts \\\\ []) do\n")
	writeArgs(b, call, []string{"transport", pathArg(p), method, g.inputEncode(p), g.outputDecoder(p), "opts"})
	b.WriteString("  end\n\n")
	writeSpec(b, name+"!", []string{"Transport.t()", g.inputSpec(p), "Transport.call_opts()"}, g.outputSpec(p))
	b.WriteString("  def ")
	b.WriteString(name)
	b.WriteString("!(transport, input, opts \\\\ []) do\n")
	b.WriteString("    case ")
	b.WriteString(name)
	b.WriteString("(transport, input, opts) do\n")
	b.WriteString("      {:ok, value} -> value\n      {:error, error} -> raise error\n    end\n  end\n")
}

func (g *generator) writeSubscription(b *strings.Builder, name string, p *contract.Procedure) {
	result := "{:ok, Enumerable.t()} | {:error, BowlineClient.Error.t()}"
	if isEmptyStruct(p.Input) {
		writeSpec(b, name, []string{"Transport.t()", "Transport.call_opts()"}, result)
		b.WriteString("  def ")
		b.WriteString(name)
		b.WriteString("(transport, opts \\\\ []) do\n")
		writeArgs(b, "Transport.subscribe", []string{"transport", quote(p.Path), "%{}", g.outputDecoder(p), "opts"})
		b.WriteString("  end\n")
		return
	}
	writeSpec(b, name, []string{"Transport.t()", g.inputSpec(p), "Transport.call_opts()"}, result)
	b.WriteString("  def ")
	b.WriteString(name)
	b.WriteString("(transport, ")
	b.WriteString(g.inputPattern(p))
	b.WriteString(", opts \\\\ []) do\n")
	writeArgs(b, "Transport.subscribe", []string{"transport", quote(p.Path), g.inputEncode(p), g.outputDecoder(p), "opts"})
	b.WriteString("  end\n")
}

func (g *generator) writeUpload(b *strings.Builder, name string, p *contract.Procedure) {
	result := "{:ok, " + g.outputSpec(p) + "} | {:error, BowlineClient.Error.t()}"
	input := g.inputSpec(p)
	if input == "" {
		input = "BowlineClient.Empty.t()"
	}
	writeSpec(b, name, []string{"Transport.t()", input, "Enumerable.t() | binary()", "String.t()", "Transport.call_opts()"}, result)
	b.WriteString("  def ")
	b.WriteString(name)
	b.WriteString("(transport, ")
	b.WriteString(g.inputPattern(p))
	b.WriteString(", file, filename, opts \\\\ []) do\n")
	writeArgs(b, "Transport.upload", []string{"transport", quote(p.Path), g.inputEncode(p), "file", "filename", g.outputDecoder(p), "opts"})
	b.WriteString("  end\n")
}

func isEmptyStruct(t *contract.Type) bool {
	return t != nil && t.Kind == contract.Struct && len(t.Fields) == 0
}

func methodAtom(method string) string {
	switch method {
	case http.MethodGet:
		return ":get"
	case http.MethodPut:
		return ":put"
	case http.MethodPatch:
		return ":patch"
	case http.MethodDelete:
		return ":delete"
	}
	return ":post"
}

func sendsBody(method string) bool {
	switch method {
	case http.MethodGet, http.MethodDelete, http.MethodHead:
		return false
	}
	return true
}

func queryPath(p *contract.Procedure) bool {
	return p.HTTPPath != "" && !sendsBody(p.Method)
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

func pathArg(p *contract.Procedure) string {
	if p.HTTPPath == "" {
		return quote(p.Path)
	}
	segments, err := contract.ParsePath(p.HTTPPath)
	if err != nil {
		return quote(p.Path)
	}
	var b strings.Builder
	b.WriteString("\"")
	for i, seg := range segments {
		if i > 0 {
			b.WriteString("/")
		}
		if !seg.Param {
			b.WriteString(seg.Text)
			continue
		}
		b.WriteString("#{URI.encode_www_form(to_string(input.")
		b.WriteString(fieldAtom(seg.Text))
		b.WriteString("))}")
	}
	b.WriteString("\"")
	return b.String()
}
