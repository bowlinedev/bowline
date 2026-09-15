package ts

import (
	"sort"
	"strconv"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type clientNode struct {
	children map[string]*clientNode
	proc     *contract.Procedure
}

func buildTree(procs []*contract.Procedure) *clientNode {
	root := &clientNode{children: map[string]*clientNode{}}
	for _, p := range procs {
		node := root
		segments := strings.Split(p.Path, ".")
		for _, s := range segments[:len(segments)-1] {
			child, ok := node.children[s]
			if !ok {
				child = &clientNode{children: map[string]*clientNode{}}
				node.children[s] = child
			}
			node = child
		}
		node.children[segments[len(segments)-1]] = &clientNode{proc: p}
	}
	return root
}

func (g *generator) clientInterface() string {
	var b strings.Builder
	b.WriteString("export interface Client {\n")
	g.writeNode(&b, buildTree(g.doc.Procedures), "  ")
	b.WriteString("}\n\n")
	return b.String()
}

func (g *generator) errorsInterface() string {
	var b strings.Builder
	b.WriteString("export interface Errors {\n")
	for _, p := range g.doc.Procedures {
		b.WriteString("  " + strconv.Quote(p.Path) + ": " + g.errorUnion(p) + ";\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("export type ProcedureError<P extends keyof Errors> = Errors[P];\n\n")
	return b.String()
}

func (g *generator) errorUnion(p *contract.Procedure) string {
	if len(p.Errors) == 0 {
		g.needs["BowlineError"] = true
		return "BowlineError"
	}
	g.needs["TypedError"] = true
	g.needs["UntypedError"] = true
	parts := make([]string, 0, len(p.Errors)+1)
	for _, id := range p.Errors {
		name := g.names[id]
		parts = append(parts, "TypedError<"+strconv.Quote(name)+", "+name+">")
	}
	parts = append(parts, "UntypedError")
	return strings.Join(parts, " | ")
}

func (g *generator) writeNode(b *strings.Builder, node *clientNode, indent string) {
	keys := make([]string, 0, len(node.children))
	for k := range node.children {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		child := node.children[k]
		if child.proc != nil {
			p := child.proc
			deprecated := ""
			if p.Deprecated != "" {
				deprecated = "@deprecated " + p.Deprecated
			}
			b.WriteString(jsdoc(indent, p.Doc, deprecated))
			kind := "Query"
			switch p.Kind {
			case "mutation":
				kind = "Mutation"
			case "subscription":
				kind = "Subscription"
			case "upload":
				kind = "Upload"
			}
			g.needs[kind] = true
			args := g.tsType(p.Input, false) + ", " + g.tsType(p.Output, false)
			if len(p.Errors) > 0 {
				args += ", Errors[" + strconv.Quote(p.Path) + "]"
			}
			b.WriteString(indent + propertyKey(k) + ": " + kind + "<" + args + ">;\n")
			continue
		}
		b.WriteString(indent + propertyKey(k) + ": {\n")
		g.writeNode(b, child, indent+"  ")
		b.WriteString(indent + "};\n")
	}
}
