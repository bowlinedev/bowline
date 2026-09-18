package gateway

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type Diagnostic struct {
	Service string
	Message string
	Fix     string
}

func (d Diagnostic) String() string {
	subject := "gateway"
	if d.Service != "" {
		subject = "service " + d.Service
	}
	if d.Fix == "" {
		return subject + ": " + d.Message
	}
	return subject + ": " + d.Message + ". " + d.Fix
}

func Compose(services map[string]*contract.Document) (*contract.Document, []Diagnostic) {
	if len(services) == 0 {
		return nil, []Diagnostic{{Message: "no services to compose", Fix: "name at least one service in the gateway configuration"}}
	}
	names := slices.Sorted(maps.Keys(services))

	var diags []Diagnostic
	major := ""
	majorService := ""
	for _, name := range names {
		if !validServiceName(name) {
			diags = append(diags, Diagnostic{Service: name, Message: "service name is not a valid procedure segment", Fix: "use letters, digits, and underscores, and never a dot"})
		}
		doc := services[name]
		if doc == nil {
			diags = append(diags, Diagnostic{Service: name, Message: "has no contract document", Fix: "point the service at a contract file or a registry"})
			continue
		}
		m := majorOf(doc.Bowline)
		if major == "" {
			major, majorService = m, name
			continue
		}
		if m != major {
			diags = append(diags, Diagnostic{Service: name, Message: fmt.Sprintf("contract format %s does not match %s from service %s", doc.Bowline, services[majorService].Bowline, majorService), Fix: "regenerate every service with the same bowline major version"})
		}
	}
	if len(diags) > 0 {
		return nil, diags
	}

	owners := map[string]map[string]bool{}
	for _, name := range names {
		for _, decl := range services[name].Types {
			claim(owners, decl.Name, name)
		}
		for _, decl := range services[name].Errors {
			claim(owners, decl.Name, name)
		}
	}

	out := &contract.Document{
		Bowline: services[names[0]].Bowline,
		Types:   map[string]*contract.TypeDecl{},
		Errors:  map[string]*contract.ErrorDecl{},
	}
	for _, name := range names {
		doc := services[name]
		for id, decl := range doc.Types {
			copied := copyDecl(decl, name)
			if len(owners[decl.Name]) > 1 {
				copied.Name = title(name) + "_" + decl.Name
			}
			out.Types[prefixID(name, id)] = copied
		}
		for id, decl := range doc.Errors {
			copied := copyError(decl, name)
			if len(owners[decl.Name]) > 1 {
				copied.Name = title(name) + "_" + decl.Name
			}
			out.Errors[prefixID(name, id)] = copied
		}
		for _, p := range doc.Procedures {
			out.Procedures = append(out.Procedures, copyProcedure(p, name))
		}
	}
	slices.SortStableFunc(out.Procedures, func(a, b *contract.Procedure) int { return cmp.Compare(a.Path, b.Path) })
	if err := out.SetHash(); err != nil {
		return nil, []Diagnostic{{Message: "hashing the composed document: " + err.Error()}}
	}
	return out, nil
}

func claim(owners map[string]map[string]bool, declName, service string) {
	if owners[declName] == nil {
		owners[declName] = map[string]bool{}
	}
	owners[declName][service] = true
}

func validServiceName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func majorOf(version string) string {
	major, _, _ := strings.Cut(version, ".")
	return major
}

func title(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func prefixID(service, id string) string {
	return service + ":" + id
}

func copyDecl(decl *contract.TypeDecl, service string) *contract.TypeDecl {
	copied := *decl
	copied.Fields = copyFields(decl.Fields, service)
	if decl.Values != nil {
		copied.Values = append([]contract.EnumValue(nil), decl.Values...)
	}
	if decl.Params != nil {
		copied.Params = append([]string(nil), decl.Params...)
	}
	copied.Body = copyType(decl.Body, service)
	return &copied
}

func copyError(decl *contract.ErrorDecl, service string) *contract.ErrorDecl {
	copied := *decl
	copied.Fields = copyFields(decl.Fields, service)
	return &copied
}

func copyProcedure(p *contract.Procedure, service string) *contract.Procedure {
	copied := *p
	copied.Path = service + "." + p.Path
	copied.HTTPPath = ""
	copied.Input = copyType(p.Input, service)
	copied.Output = copyType(p.Output, service)
	if len(p.Errors) > 0 {
		errs := make([]string, len(p.Errors))
		for i, id := range p.Errors {
			errs[i] = prefixID(service, id)
		}
		copied.Errors = errs
	}
	if p.Tool != nil {
		tool := *p.Tool
		if p.Tool.Scopes != nil {
			tool.Scopes = append([]string(nil), p.Tool.Scopes...)
		}
		copied.Tool = &tool
	}
	if p.Meta != nil {
		meta := make(map[string]string, len(p.Meta))
		maps.Copy(meta, p.Meta)
		copied.Meta = meta
	}
	return &copied
}

func copyFields(fields []*contract.Field, service string) []*contract.Field {
	if fields == nil {
		return nil
	}
	out := make([]*contract.Field, len(fields))
	for i, f := range fields {
		copied := *f
		copied.Type = copyType(f.Type, service)
		if f.Rules != nil {
			copied.Rules = append([]contract.Rule(nil), f.Rules...)
		}
		out[i] = &copied
	}
	return out
}

func copyType(t *contract.Type, service string) *contract.Type {
	if t == nil {
		return nil
	}
	copied := *t
	if t.Kind == contract.Ref && t.ID != "" {
		copied.ID = prefixID(service, t.ID)
	}
	if len(t.Args) > 0 {
		args := make([]*contract.Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = copyType(a, service)
		}
		copied.Args = args
	}
	copied.Elem = copyType(t.Elem, service)
	copied.Key = copyType(t.Key, service)
	copied.Value = copyType(t.Value, service)
	copied.Fields = copyFields(t.Fields, service)
	return &copied
}
