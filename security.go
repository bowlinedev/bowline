package bowline

import (
	"errors"
	"fmt"
	"slices"
)

type SecuritySchemeKind string

const (
	SecurityHTTP   SecuritySchemeKind = "http"
	SecurityAPIKey SecuritySchemeKind = "apiKey"
)

type SecurityScheme struct {
	Kind        SecuritySchemeKind
	Scheme      string
	BearerAs    string
	In          string
	Header      string
	Description string
}

func BearerAuth(format string) SecurityScheme {
	return SecurityScheme{Kind: SecurityHTTP, Scheme: "bearer", BearerAs: format}
}

func BasicAuth() SecurityScheme {
	return SecurityScheme{Kind: SecurityHTTP, Scheme: "basic"}
}

func APIKeyAuth(in, name string) SecurityScheme {
	return SecurityScheme{Kind: SecurityAPIKey, In: in, Header: name}
}

func (s SecurityScheme) validate(name string) error {
	if name == "" {
		return errors.New("bowline: security scheme name is empty")
	}
	switch s.Kind {
	case SecurityHTTP:
		if s.Scheme != "bearer" && s.Scheme != "basic" {
			return fmt.Errorf("bowline: security scheme %q: http scheme %q is not supported", name, s.Scheme)
		}
	case SecurityAPIKey:
		switch s.In {
		case "header", "query", "cookie":
		default:
			return fmt.Errorf("bowline: security scheme %q: apiKey must be in a header, query or cookie, not %q", name, s.In)
		}
		if s.Header == "" {
			return fmt.Errorf("bowline: security scheme %q: apiKey needs a parameter name", name)
		}
	default:
		return fmt.Errorf("bowline: security scheme %q: unknown kind %q", name, s.Kind)
	}
	return nil
}

func (r *Router) Scheme(name string, scheme SecurityScheme) *Router {
	if err := scheme.validate(name); err != nil {
		panic(err.Error())
	}
	if r.schemes == nil {
		r.schemes = map[string]SecurityScheme{}
	}
	if existing, ok := r.schemes[name]; ok && existing != scheme {
		panic(fmt.Sprintf("bowline: security scheme %q is declared twice with different settings", name))
	}
	r.schemes[name] = scheme
	return r
}

func (r *Router) Secure(names ...string) *Router {
	for _, name := range names {
		if !slices.Contains(r.requires, name) {
			r.requires = append(r.requires, name)
		}
	}
	return r
}

func Public() ProcOption {
	return func(p *Procedure) { p.public = true }
}

func Requires(names ...string) ProcOption {
	return func(p *Procedure) { p.Security = append(p.Security, names...) }
}

func securityFor(p *Procedure, inherited []string) []string {
	if p.public {
		return nil
	}
	out := make([]string, 0, len(inherited)+len(p.Security))
	out = append(out, inherited...)
	for _, name := range p.Security {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}
