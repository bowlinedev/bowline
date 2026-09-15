package ts

import (
	"sort"
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
			if p.Kind == "mutation" {
				kind = "Mutation"
			}
			g.needs[kind] = true
			b.WriteString(indent + propertyKey(k) + ": " + kind + "<" + g.tsType(p.Input, false) + ", " + g.tsType(p.Output, false) + ">;\n")
			continue
		}
		b.WriteString(indent + propertyKey(k) + ": {\n")
		g.writeNode(b, child, indent+"  ")
		b.WriteString(indent + "};\n")
	}
}
