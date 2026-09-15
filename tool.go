package bowline

import "fmt"

type ToolOption func(*Procedure)

func Tool(opts ...ToolOption) ProcOption {
	return func(p *Procedure) {
		if p.Kind == KindSubscription || p.Kind == KindUpload {
			panic(fmt.Sprintf("bowline: %s %q: subscriptions and uploads cannot be exposed as tools", p.Kind, p.Name))
		}
		p.Exposed = true
		p.ReadOnly = p.Kind == KindQuery
		for _, opt := range opts {
			opt(p)
		}
	}
}

func Scope(names ...string) ToolOption {
	return func(p *Procedure) {
		for _, name := range names {
			if name == "" {
				panic(fmt.Sprintf("bowline: %s %q: empty scope name", p.Kind, p.Name))
			}
			known := false
			for _, existing := range p.Scopes {
				if existing == name {
					known = true
					break
				}
			}
			if !known {
				p.Scopes = append(p.Scopes, name)
			}
		}
	}
}

func Destructive() ToolOption {
	return func(p *Procedure) { p.Destructive = true }
}
