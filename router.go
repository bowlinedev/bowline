package bowline

import (
	"fmt"
	"strings"
)

type Router struct {
	entries    []entry
	middleware []Middleware
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

func (r *Router) Use(mw ...Middleware) *Router {
	r.middleware = append(r.middleware, mw...)
	return r
}

func (r *Router) Procedures() []Procedure {
	routes := r.routes()
	out := make([]Procedure, 0, len(routes))
	for _, rt := range routes {
		p := *rt.proc
		p.Path = rt.path
		out = append(out, p)
	}
	return out
}

func (r *Router) routes() []route {
	var out []route
	r.walk("", nil, &out)
	return out
}

func (r *Router) walk(prefix string, inherited []Middleware, out *[]route) {
	chain := make([]Middleware, 0, len(inherited)+len(r.middleware))
	chain = append(chain, inherited...)
	chain = append(chain, r.middleware...)
	for _, e := range r.entries {
		path := e.name
		if prefix != "" {
			path = prefix + "." + e.name
		}
		if e.child != nil {
			e.child.walk(path, chain, out)
			continue
		}
		full := make([]Middleware, 0, len(chain)+len(e.proc.middleware))
		full = append(full, chain...)
		full = append(full, e.proc.middleware...)
		next := e.proc.call
		for i := len(full) - 1; i >= 0; i-- {
			next = full[i](next)
		}
		procedure := *e.proc
		procedure.Path = path
		*out = append(*out, route{path: path, proc: e.proc, procedure: procedure, next: next})
	}
}
