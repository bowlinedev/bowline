package analyzer

import (
	"fmt"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/internal/validate"
)

type collector struct {
	prog      *Program
	doc       *contract.Document
	diags     []Diagnostic
	wireAs    map[string]types.Type
	declaring map[string]bool
}

func newCollector(p *Program, doc *contract.Document) *collector {
	return &collector{prog: p, doc: doc, wireAs: map[string]types.Type{}, declaring: map[string]bool{}}
}

func (c *collector) fail(pos token.Pos, path, message, fix string) *contract.Type {
	c.diags = append(c.diags, Diagnostic{Pos: c.prog.Position(pos), Path: path, Message: message, Fix: fix})
	return nil
}

func primitive(name string) *contract.Type {
	return &contract.Type{Kind: contract.Primitive, Name: name}
}

var basicNames = map[types.BasicKind]string{
	types.Bool: "bool", types.String: "string",
	types.Int: "int64", types.Int8: "int8", types.Int16: "int16", types.Int32: "int32", types.Int64: "int64",
	types.Uint: "uint64", types.Uint8: "uint8", types.Uint16: "uint16", types.Uint32: "uint32", types.Uint64: "uint64",
	types.Float32: "float32", types.Float64: "float64",
}

func (c *collector) typeNode(t types.Type, pos token.Pos, path string) *contract.Type {
	t = types.Unalias(t)
	switch t := t.(type) {
	case *types.Basic:
		if name, ok := basicNames[t.Kind()]; ok {
			return primitive(name)
		}
		return c.fail(pos, path, fmt.Sprintf("%s is not supported", t), "use a sized integer, float, string, or bool")
	case *types.Pointer:
		if _, double := types.Unalias(t.Elem()).(*types.Pointer); double {
			return c.fail(pos, path, "pointer to pointer is not supported", "use a single pointer")
		}
		node := c.typeNode(t.Elem(), pos, path)
		if node != nil {
			node.Nullable = true
		}
		return node
	case *types.Slice:
		if isByte(t.Elem()) {
			return primitive("bytes")
		}
		elem := c.typeNode(t.Elem(), pos, path)
		if elem == nil {
			return nil
		}
		return &contract.Type{Kind: contract.Array, Elem: elem}
	case *types.Array:
		elem := c.typeNode(t.Elem(), pos, path)
		if elem == nil {
			return nil
		}
		return &contract.Type{Kind: contract.Array, Elem: elem, Length: int(t.Len())}
	case *types.Map:
		key := c.mapKey(t.Key(), pos, path)
		if key == nil {
			return nil
		}
		value := c.typeNode(t.Elem(), pos, path)
		if value == nil {
			return nil
		}
		return &contract.Type{Kind: contract.Map, Key: key, Value: value}
	case *types.Struct:
		fields, ok := c.fields(t, pos, path)
		if !ok {
			return nil
		}
		return &contract.Type{Kind: contract.Struct, Fields: fields}
	case *types.Named:
		return c.named(t, pos, path)
	case *types.TypeParam:
		return &contract.Type{Kind: contract.Param, Name: t.Obj().Name()}
	case *types.Interface:
		return c.fail(pos, path, "interfaces are not supported", "use a concrete struct, or json.RawMessage for an untyped payload")
	}
	return c.fail(pos, path, fmt.Sprintf("%s is not supported", t), "use a struct, slice, map, or primitive")
}

func isByte(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.Uint8
}

func (c *collector) mapKey(t types.Type, pos token.Pos, path string) *contract.Type {
	u := types.Unalias(t)
	if named, ok := u.(*types.Named); ok && implementsMethod(named, "MarshalText") {
		return primitive("string")
	}
	if b, ok := u.Underlying().(*types.Basic); ok {
		if name, ok := basicNames[b.Kind()]; ok && (b.Info()&types.IsString != 0 || b.Info()&types.IsInteger != 0) {
			return primitive(name)
		}
	}
	return c.fail(pos, path, fmt.Sprintf("map key type %s is not supported", t), "use a string, integer, or encoding.TextMarshaler key")
}

func (c *collector) named(t *types.Named, pos token.Pos, path string) *contract.Type {
	obj := t.Obj()
	if obj.Pkg() != nil {
		switch obj.Pkg().Path() + "." + obj.Name() {
		case "time.Time":
			return primitive("timestamp")
		case "time.Duration":
			return primitive("duration")
		case "encoding/json.RawMessage":
			return primitive("raw")
		}
	}
	id := goTypeName(t)
	if wire, ok := c.wireAs[id]; ok {
		return c.typeNode(wire, pos, path)
	}
	if implementsMethod(t, "MarshalJSON") {
		return c.fail(pos, path, fmt.Sprintf("%s implements json.Marshaler, so its wire shape cannot be inferred", obj.Name()), fmt.Sprintf("declare it with var _ = bowline.WireAs[%s, W]() where W is the type it marshals as", obj.Name()))
	}
	if implementsMethod(t, "MarshalText") {
		return primitive("string")
	}
	if t.TypeArgs() != nil || t.TypeParams().Len() > 0 {
		return c.generic(t, pos, path)
	}
	switch t.Underlying().(type) {
	case *types.Slice, *types.Array, *types.Map:
		return c.typeNode(t.Underlying(), pos, path)
	case *types.Interface:
		return c.fail(pos, path, fmt.Sprintf("%s is an interface type, which is not supported", obj.Name()), "use a concrete struct, or json.RawMessage for an untyped payload")
	}
	if _, done := c.doc.Types[id]; !done {
		c.declare(t, id)
	}
	return &contract.Type{Kind: contract.Ref, ID: id}
}

func (c *collector) declare(t *types.Named, id string) {
	obj := t.Obj()
	pkg := c.prog.Package(pkgPath(obj))
	decl := &contract.TypeDecl{Name: obj.Name(), Doc: c.prog.Doc(pkg, obj.Pos())}
	c.doc.Types[id] = decl
	c.recordPosition(id, obj.Pos())
	switch u := t.Underlying().(type) {
	case *types.Struct:
		decl.Kind = contract.Struct
		fields, _ := c.fields(u, obj.Pos(), obj.Name())
		decl.Fields = fields
	case *types.Basic:
		c.basicDecl(t, u, decl)
	default:
		c.fail(obj.Pos(), obj.Name(), fmt.Sprintf("%s is not supported as a named type", u), "use a struct or a named basic type")
		delete(c.doc.Types, id)
	}
}

func pkgPath(obj types.Object) string {
	if obj.Pkg() == nil {
		return ""
	}
	return obj.Pkg().Path()
}

func (c *collector) recordPosition(key string, pos token.Pos) {
	p := c.prog.Position(pos)
	if p.Filename == "" || strings.HasPrefix(p.Filename, "..") || strings.HasPrefix(p.Filename, "/") {
		return
	}
	c.doc.Positions[key] = contract.Position{File: strings.ReplaceAll(p.Filename, "\\", "/"), Line: p.Line}
}

type fieldEntry struct {
	field *contract.Field
	depth int
}

func (c *collector) fields(st *types.Struct, pos token.Pos, path string) ([]*contract.Field, bool) {
	entries, ok := c.collectFields(st, path, 0)
	if !ok {
		return nil, false
	}
	byName := map[string][]fieldEntry{}
	var order []string
	for _, e := range entries {
		if _, seen := byName[e.field.Name]; !seen {
			order = append(order, e.field.Name)
		}
		byName[e.field.Name] = append(byName[e.field.Name], e)
	}
	var out []*contract.Field
	for _, name := range order {
		candidates := byName[name]
		best := candidates[0]
		tie := false
		for _, e := range candidates[1:] {
			switch {
			case e.depth < best.depth:
				best, tie = e, false
			case e.depth == best.depth:
				tie = true
			}
		}
		if !tie {
			out = append(out, best.field)
		}
	}
	return out, true
}

func (c *collector) collectFields(st *types.Struct, path string, depth int) ([]fieldEntry, bool) {
	var entries []fieldEntry
	ok := true
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag := reflect.StructTag(st.Tag(i))
		jsonName, opts, _ := strings.Cut(tag.Get("json"), ",")
		if jsonName == "-" {
			continue
		}
		if f.Embedded() && jsonName == "" {
			et := types.Unalias(f.Type())
			if p, isPtr := et.(*types.Pointer); isPtr {
				et = types.Unalias(p.Elem())
			}
			if inner, isStruct := et.Underlying().(*types.Struct); isStruct {
				if _, isNamed := et.(*types.Named); isNamed && implementsMethod(et, "MarshalJSON") {
					c.fail(f.Pos(), path+"."+f.Name(), "embedded json.Marshaler types are not supported", "give the field a json tag or declare it with WireAs")
					ok = false
					continue
				}
				nested, nestedOK := c.collectFields(inner, path+"."+f.Name(), depth+1)
				ok = ok && nestedOK
				entries = append(entries, nested...)
				continue
			}
		}
		if !f.Exported() {
			continue
		}
		field, fieldOK := c.field(f, jsonName, opts, tag.Get("validate"), path)
		if !fieldOK {
			ok = false
			continue
		}
		entries = append(entries, fieldEntry{field: field, depth: depth})
	}
	return entries, ok
}

func (c *collector) field(f *types.Var, jsonName, opts, validateTag, path string) (*contract.Field, bool) {
	name := jsonName
	if name == "" {
		name = f.Name()
	}
	fieldPath := path + "." + f.Name()
	pkg := c.prog.Package(pkgPath(f))
	field := &contract.Field{Name: name, Doc: c.prog.Doc(pkg, f.Pos())}
	field.Optional = hasOption(opts, "omitempty") || hasOption(opts, "omitzero")
	ft := types.Unalias(f.Type())
	if p, isPtr := ft.(*types.Pointer); isPtr {
		if _, double := types.Unalias(p.Elem()).(*types.Pointer); double {
			c.fail(f.Pos(), fieldPath, "pointer to pointer is not supported", "use a single pointer")
			return nil, false
		}
		field.Nullable = !field.Optional
		ft = types.Unalias(p.Elem())
	}
	node := c.typeNode(ft, f.Pos(), fieldPath)
	if node == nil {
		return nil, false
	}
	if hasOption(opts, "string") {
		if node.Kind != contract.Primitive || (node.Name != "int64" && node.Name != "uint64") {
			c.fail(f.Pos(), fieldPath, "the json \",string\" option is only supported on int64 and uint64 fields", "remove the option or change the field type")
			return nil, false
		}
		node.Encoding = "string"
	}
	field.Type = node
	rules, err := validate.ParseTag(validateTag)
	if err != nil {
		c.fail(f.Pos(), fieldPath, err.Error(), "use only required, min, max, len, oneof, email, url, uuid")
		return nil, false
	}
	class := classOf(ft)
	for _, r := range rules {
		if !validate.Applies(r.Name, class) {
			c.fail(f.Pos(), fieldPath, fmt.Sprintf("validation rule %q does not apply to %s", r.Name, ft), "remove the rule or change the field type")
			return nil, false
		}
		field.Rules = append(field.Rules, contract.Rule{Rule: r.Name, Param: r.Param})
	}
	return field, true
}

func classOf(t types.Type) validate.Class {
	switch u := types.Unalias(t).Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			return validate.String
		case u.Info()&types.IsInteger != 0:
			return validate.Integer
		case u.Info()&types.IsFloat != 0:
			return validate.Float
		case u.Info()&types.IsBoolean != 0:
			return validate.Bool
		}
	case *types.Slice, *types.Array, *types.Map:
		return validate.Collection
	case *types.Struct:
		return validate.Struct
	}
	return validate.Other
}

func hasOption(opts, name string) bool {
	for _, o := range strings.Split(opts, ",") {
		if o == name {
			return true
		}
	}
	return false
}

func implementsMethod(t types.Type, name string) bool {
	for _, candidate := range []types.Type{t, types.NewPointer(t)} {
		obj, _, _ := types.LookupFieldOrMethod(candidate, true, nil, name)
		if fn, ok := obj.(*types.Func); ok && fn.Exported() {
			return true
		}
	}
	return false
}

func (c *collector) basicDecl(t *types.Named, basic *types.Basic, decl *contract.TypeDecl) {
	decl.Kind = contract.Primitive
	decl.Primitive = basicNames[basic.Kind()]
}

func (c *collector) generic(t *types.Named, pos token.Pos, path string) *contract.Type {
	return c.fail(pos, path, "generic types are not supported yet", "")
}
