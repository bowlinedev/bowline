package contract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Category string

const (
	Added    Category = "added"
	Removed  Category = "removed"
	Widened  Category = "widened"
	Narrowed Category = "narrowed"
	Breaking Category = "breaking"
)

type Change struct {
	Path     string   `json:"path"`
	Category Category `json:"category"`
	Message  string   `json:"message"`
}

type Changes []Change

func (c Changes) Breaking() bool {
	for _, ch := range c {
		if ch.Category == Breaking {
			return true
		}
	}
	return false
}

type variance int

const (
	input variance = iota
	output
)

type differ struct {
	old, new *Document
	changes  Changes
	visited  map[string]bool
}

func Diff(old, new *Document) Changes {
	d := &differ{old: old, new: new, visited: map[string]bool{}}
	oldProcs := map[string]*Procedure{}
	for _, p := range old.Procedures {
		oldProcs[p.Path] = p
	}
	newProcs := map[string]*Procedure{}
	for _, p := range new.Procedures {
		newProcs[p.Path] = p
	}
	for path, op := range oldProcs {
		np, ok := newProcs[path]
		if !ok {
			d.add("procedure "+path, Breaking, "procedure removed")
			continue
		}
		d.procedure(op, np)
	}
	for path := range newProcs {
		if _, ok := oldProcs[path]; !ok {
			d.add("procedure "+path, Added, "procedure added")
		}
	}
	sort.SliceStable(d.changes, func(i, j int) bool {
		if d.changes[i].Path != d.changes[j].Path {
			return d.changes[i].Path < d.changes[j].Path
		}
		return d.changes[i].Category < d.changes[j].Category
	})
	return d.changes
}

func (d *differ) add(path string, category Category, message string) {
	d.changes = append(d.changes, Change{Path: path, Category: category, Message: message})
}

func (d *differ) procedure(op, np *Procedure) {
	base := "procedure " + op.Path
	if op.Kind != np.Kind {
		d.add(base, Breaking, fmt.Sprintf("kind changed from %s to %s", op.Kind, np.Kind))
		return
	}
	if op.Method != np.Method {
		d.add(base, Breaking, fmt.Sprintf("method changed from %s to %s", op.Method, np.Method))
	}
	d.typeNode(base+" input", op.Input, np.Input, nil, nil, input)
	d.typeNode(base+" output", op.Output, np.Output, nil, nil, output)
	d.errors(base, op, np)
	if op.Idempotent && !np.Idempotent {
		d.add(base, Breaking, "idempotency removed")
	}
	if !op.Idempotent && np.Idempotent {
		d.add(base, Added, "idempotency added")
	}
	if op.Deprecated == "" && np.Deprecated != "" {
		d.add(base, Added, "deprecated: "+np.Deprecated)
	}
}

func (d *differ) errors(base string, op, np *Procedure) {
	oldSet := map[string]bool{}
	for _, id := range op.Errors {
		oldSet[id] = true
	}
	newSet := map[string]bool{}
	for _, id := range np.Errors {
		newSet[id] = true
	}
	for _, id := range op.Errors {
		od := d.old.Errors[id]
		nd := d.new.Errors[id]
		if !newSet[id] || od == nil || nd == nil {
			d.add(base+" error "+errorName(d.old, id), Narrowed, "error variant removed")
			continue
		}
		path := base + " error " + od.Name
		if od.Code != nd.Code {
			d.add(path, Breaking, fmt.Sprintf("code changed from %s to %s", od.Code, nd.Code))
		}
		d.fields(path, od.Fields, nd.Fields, nil, nil, output)
	}
	for _, id := range np.Errors {
		if !oldSet[id] {
			d.add(base+" error "+errorName(d.new, id), Widened, "error variant added")
		}
	}
}

func errorName(doc *Document, id string) string {
	if decl, ok := doc.Errors[id]; ok && decl != nil {
		return decl.Name
	}
	return id
}

type env map[string]*Type

func (d *differ) typeNode(path string, ot, nt *Type, oenv, nenv env, v variance) {
	ot = resolveParam(ot, oenv)
	nt = resolveParam(nt, nenv)
	if ot == nil || nt == nil {
		if ot != nt {
			d.add(path, Breaking, "type changed")
		}
		return
	}
	if ot.Nullable != nt.Nullable {
		switch {
		case nt.Nullable && v == input:
			d.add(path, Widened, "now accepts null")
		case nt.Nullable && v == output:
			d.add(path, Breaking, "may now be null")
		case !nt.Nullable && v == input:
			d.add(path, Breaking, "no longer accepts null")
		default:
			d.add(path, Narrowed, "no longer null")
		}
	}
	if ot.Kind != nt.Kind {
		d.add(path, Breaking, fmt.Sprintf("type changed from %s to %s", describe(ot), describe(nt)))
		return
	}
	switch ot.Kind {
	case Primitive:
		if ot.Name != nt.Name || ot.Encoding != nt.Encoding {
			d.add(path, Breaking, fmt.Sprintf("type changed from %s to %s", describe(ot), describe(nt)))
		}
	case Array:
		if ot.Length != nt.Length {
			d.add(path, Breaking, fmt.Sprintf("array length changed from %d to %d", ot.Length, nt.Length))
		}
		d.typeNode(path+" elem", ot.Elem, nt.Elem, oenv, nenv, v)
	case Map:
		d.typeNode(path+" value", ot.Value, nt.Value, oenv, nenv, v)
	case Struct:
		d.fields(path, ot.Fields, nt.Fields, oenv, nenv, v)
	case Param:
		if ot.Name != nt.Name {
			d.add(path, Breaking, "type parameter changed")
		}
	case Ref:
		d.ref(path, ot, nt, oenv, nenv, v)
	}
}

func resolveParam(t *Type, e env) *Type {
	if t != nil && t.Kind == Param && e != nil {
		if bound, ok := e[t.Name]; ok {
			return bound
		}
	}
	return t
}

func describe(t *Type) string {
	switch t.Kind {
	case Primitive:
		if t.Encoding != "" {
			return t.Name + " as " + t.Encoding
		}
		return t.Name
	case Ref:
		if i := strings.LastIndex(t.ID, "."); i >= 0 {
			return t.ID[i+1:]
		}
		return t.ID
	}
	return string(t.Kind)
}

func (d *differ) ref(path string, ot, nt *Type, oenv, nenv env, v variance) {
	od := d.old.Types[ot.ID]
	nd := d.new.Types[nt.ID]
	if od == nil || nd == nil {
		if ot.ID != nt.ID {
			d.add(path, Breaking, fmt.Sprintf("type changed from %s to %s", describe(ot), describe(nt)))
		}
		return
	}
	if od.Kind != nd.Kind {
		d.add(path, Breaking, fmt.Sprintf("type changed from %s %s to %s %s", od.Kind, od.Name, nd.Kind, nd.Name))
		return
	}
	oargs := make([]*Type, len(ot.Args))
	for i, a := range ot.Args {
		oargs[i] = resolveParam(a, oenv)
	}
	nargs := make([]*Type, len(nt.Args))
	for i, a := range nt.Args {
		nargs[i] = resolveParam(a, nenv)
	}
	key := ot.ID + "|" + argsKey(oargs) + "=>" + nt.ID + "|" + argsKey(nargs) + "|" + strconv.Itoa(int(v))
	if d.visited[key] {
		return
	}
	d.visited[key] = true
	switch od.Kind {
	case Struct:
		d.fields(path, od.Fields, nd.Fields, nil, nil, v)
	case Enum:
		d.enum(path, od, nd, v)
	case Primitive:
		if od.Primitive != nd.Primitive {
			d.add(path, Breaking, fmt.Sprintf("type changed from %s to %s", od.Primitive, nd.Primitive))
		}
	case Generic:
		if len(od.Params) != len(nd.Params) || len(oargs) != len(od.Params) || len(nargs) != len(nd.Params) {
			d.add(path, Breaking, "generic arity changed")
			return
		}
		oe := env{}
		for i, p := range od.Params {
			oe[p] = oargs[i]
		}
		ne := env{}
		for i, p := range nd.Params {
			ne[p] = nargs[i]
		}
		d.typeNode(path, od.Body, nd.Body, oe, ne, v)
	}
}

func argsKey(args []*Type) string {
	parts := make([]string, len(args))
	for i, a := range args {
		data, _ := json.Marshal(a)
		parts[i] = string(data)
	}
	return strings.Join(parts, ",")
}

func (d *differ) fields(path string, oldFields, newFields []*Field, oenv, nenv env, v variance) {
	newByName := map[string]*Field{}
	for _, f := range newFields {
		newByName[f.Name] = f
	}
	oldByName := map[string]*Field{}
	for _, f := range oldFields {
		oldByName[f.Name] = f
	}
	for _, of := range oldFields {
		fieldPath := path + " field " + of.Name
		nf, ok := newByName[of.Name]
		if !ok {
			if v == input {
				d.add(fieldPath, Widened, "field removed")
			} else {
				d.add(fieldPath, Breaking, "field removed")
			}
			continue
		}
		d.optionality(fieldPath, of, nf, v)
		d.typeNode(fieldPath, of.Type, nf.Type, oenv, nenv, v)
		d.rules(fieldPath, of.Rules, nf.Rules, v)
	}
	for _, nf := range newFields {
		if _, ok := oldByName[nf.Name]; ok {
			continue
		}
		fieldPath := path + " field " + nf.Name
		switch {
		case v == input && !nf.Optional:
			d.add(fieldPath, Breaking, "required field added")
		case v == input:
			d.add(fieldPath, Widened, "optional field added")
		default:
			d.add(fieldPath, Added, "field added")
		}
	}
}

func (d *differ) optionality(path string, of, nf *Field, v variance) {
	if of.Optional && !nf.Optional {
		if v == input {
			d.add(path, Breaking, "field is now required")
		} else {
			d.add(path, Narrowed, "field is now always present")
		}
	}
	if !of.Optional && nf.Optional {
		if v == input {
			d.add(path, Widened, "field is now optional")
		} else {
			d.add(path, Breaking, "field may now be absent")
		}
	}
	if !of.Nullable && nf.Nullable {
		if v == input {
			d.add(path, Widened, "field now accepts null")
		} else {
			d.add(path, Breaking, "field may now be null")
		}
	}
	if of.Nullable && !nf.Nullable {
		if v == input {
			d.add(path, Breaking, "field no longer accepts null")
		} else {
			d.add(path, Narrowed, "field is no longer null")
		}
	}
}

func (d *differ) enum(path string, od, nd *TypeDecl, v variance) {
	if od.Base != nd.Base {
		d.add(path, Breaking, fmt.Sprintf("enum base changed from %s to %s", od.Base, nd.Base))
		return
	}
	oldValues := map[string]bool{}
	for _, val := range od.Values {
		oldValues[fmt.Sprint(val.Value)] = true
	}
	newValues := map[string]bool{}
	for _, val := range nd.Values {
		newValues[fmt.Sprint(val.Value)] = true
	}
	for _, val := range od.Values {
		key := fmt.Sprint(val.Value)
		if !newValues[key] {
			if v == input {
				d.add(path, Breaking, "enum value "+key+" removed")
			} else {
				d.add(path, Narrowed, "enum value "+key+" removed")
			}
		}
	}
	for _, val := range nd.Values {
		key := fmt.Sprint(val.Value)
		if !oldValues[key] {
			if v == input {
				d.add(path, Widened, "enum value "+key+" added")
			} else {
				d.add(path, Breaking, "enum value "+key+" added")
			}
		}
	}
}

func (d *differ) rules(path string, oldRules, newRules []Rule, v variance) {
	oldByName := map[string]Rule{}
	for _, r := range oldRules {
		oldByName[r.Rule] = r
	}
	newByName := map[string]Rule{}
	for _, r := range newRules {
		newByName[r.Rule] = r
	}
	report := func(rule string, tightened bool, message string) {
		switch {
		case tightened && v == input:
			d.add(path, Breaking, message)
		case tightened:
			d.add(path, Narrowed, message)
		case v == input:
			d.add(path, Widened, message)
		default:
			d.add(path, Added, message)
		}
	}
	for _, or := range oldRules {
		nr, ok := newByName[or.Rule]
		if !ok {
			report(or.Rule, false, "validation rule "+or.Rule+" removed")
			continue
		}
		if or.Param == nr.Param {
			continue
		}
		switch or.Rule {
		case "min":
			report(or.Rule, numeric(nr.Param) > numeric(or.Param), fmt.Sprintf("validation rule min changed from %s to %s", or.Param, nr.Param))
		case "max":
			report(or.Rule, numeric(nr.Param) < numeric(or.Param), fmt.Sprintf("validation rule max changed from %s to %s", or.Param, nr.Param))
		case "oneof":
			d.oneof(path, or.Param, nr.Param, v)
		default:
			report(or.Rule, true, fmt.Sprintf("validation rule %s changed from %s to %s", or.Rule, or.Param, nr.Param))
		}
	}
	for _, nr := range newRules {
		if _, ok := oldByName[nr.Rule]; !ok {
			report(nr.Rule, true, "validation rule "+nr.Rule+" added")
		}
	}
}

func (d *differ) oneof(path, oldParam, newParam string, v variance) {
	oldSet := map[string]bool{}
	for _, s := range strings.Fields(oldParam) {
		oldSet[s] = true
	}
	newSet := map[string]bool{}
	for _, s := range strings.Fields(newParam) {
		newSet[s] = true
	}
	removed, added := false, false
	for s := range oldSet {
		if !newSet[s] {
			removed = true
		}
	}
	for s := range newSet {
		if !oldSet[s] {
			added = true
		}
	}
	message := fmt.Sprintf("validation rule oneof changed from %q to %q", oldParam, newParam)
	switch {
	case removed && v == input:
		d.add(path, Breaking, message)
	case removed:
		d.add(path, Narrowed, message)
	case added && v == input:
		d.add(path, Widened, message)
	case added:
		d.add(path, Added, message)
	}
}

func numeric(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func FormatText(c Changes) string {
	if len(c) == 0 {
		return "no contract changes\n"
	}
	var b strings.Builder
	for _, ch := range c {
		fmt.Fprintf(&b, "%-9s %s: %s\n", ch.Category, ch.Path, ch.Message)
	}
	return b.String()
}

func FormatMarkdown(c Changes) string {
	if len(c) == 0 {
		return "no contract changes\n"
	}
	breaking := 0
	for _, ch := range c {
		if ch.Category == Breaking {
			breaking++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**%d contract change(s), %d breaking**\n\n", len(c), breaking)
	for _, ch := range c {
		fmt.Fprintf(&b, "- **%s** `%s`: %s\n", ch.Category, ch.Path, ch.Message)
	}
	return b.String()
}

func FormatJSON(c Changes) ([]byte, error) {
	if c == nil {
		c = Changes{}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
