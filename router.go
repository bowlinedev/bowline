package bowline

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Router struct {
	entries    []entry
	middleware []Middleware
	schemes    map[string]SecurityScheme
	requires   []string
}

type entry struct {
	name  string
	proc  *Procedure
	child *Router
}

type route struct {
	path      string
	proc      *Procedure
	procedure Procedure
	next      Next
}

func NewRouter(items ...Item) *Router {
	r := &Router{}
	for _, it := range items {
		it.apply(r)
	}
	return r
}

type mountItem struct {
	name  string
	child *Router
}

func Mount(name string, child *Router) Item {
	return mountItem{name: name, child: child}
}

func (m mountItem) apply(r *Router) {
	if m.child == nil {
		panic(fmt.Sprintf("bowline: Mount(%q): nil router", m.name))
	}
	r.add(entry{name: m.name, child: m.child})
}

func (r *Router) add(e entry) {
	if e.name == "" {
		panic("bowline: empty procedure or mount name")
	}
	if strings.ContainsAny(e.name, "./ ") {
		panic(fmt.Sprintf("bowline: name %q must not contain '.', '/', or spaces", e.name))
	}
	for _, existing := range r.entries {
		if existing.name == e.name {
			panic(fmt.Sprintf("bowline: duplicate name %q", e.name))
		}
	}
	r.entries = append(r.entries, e)
}

func (r *Router) Schemes() map[string]SecurityScheme {
	out := map[string]SecurityScheme{}
	r.collectSchemes(out)
	return out
}

func (r *Router) Use(mw ...Middleware) *Router {
	r.middleware = append(r.middleware, mw...)
	return r
}

func (r *Router) Procedures() []Procedure {
	routes := r.routes()
	out := make([]Procedure, 0, len(routes))
	for _, rt := range routes {
		out = append(out, rt.procedure)
	}
	return out
}

func (r *Router) routes() []route {
	var out []route
	r.walk("", nil, &out)
	return out
}

func (r *Router) collectSchemes(out map[string]SecurityScheme) {
	maps.Copy(out, r.schemes)
	for _, e := range r.entries {
		if e.child != nil {
			e.child.collectSchemes(out)
		}
	}
}

func (r *Router) walk(prefix string, inherited []Middleware, out *[]route) {
	r.walkSecured(prefix, inherited, nil, out)
}

func (r *Router) walkSecured(prefix string, inherited []Middleware, required []string, out *[]route) {
	chain := make([]Middleware, 0, len(inherited)+len(r.middleware))
	chain = append(chain, inherited...)
	chain = append(chain, r.middleware...)
	guards := make([]string, 0, len(required)+len(r.requires))
	guards = append(guards, required...)
	for _, name := range r.requires {
		if !slices.Contains(guards, name) {
			guards = append(guards, name)
		}
	}
	for _, e := range r.entries {
		path := e.name
		if prefix != "" {
			path = prefix + "." + e.name
		}
		if e.child != nil {
			e.child.walkSecured(path, chain, guards, out)
			continue
		}
		full := make([]Middleware, 0, len(chain)+len(e.proc.middleware))
		full = append(full, chain...)
		full = append(full, e.proc.middleware...)
		next := e.proc.call
		for _, f := range slices.Backward(full) {
			next = f(next)
		}
		procedure := *e.proc
		procedure.Path = path
		procedure.Security = securityFor(e.proc, guards)
		*out = append(*out, route{path: path, proc: e.proc, procedure: procedure, next: next})
	}
}
