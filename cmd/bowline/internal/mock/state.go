package mock

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/fake"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/shape"
	"github.com/bowlinedev/bowline/contract"
)

type state struct {
	mu     sync.Mutex
	doc    *contract.Document
	gen    *fake.Generator
	tables map[string]*table
}

type table struct {
	order []string
	rows  map[string]map[string]any
}

func newState(doc *contract.Document, gen *fake.Generator) *state {
	return &state{doc: doc, gen: gen, tables: map[string]*table{}}
}

func (s *state) storedType(t *contract.Type) (string, string) {
	if t == nil || t.Kind != contract.Ref {
		return "", ""
	}
	decl, ok := s.doc.Types[t.ID]
	if !ok || decl.Kind != contract.Struct {
		return "", ""
	}
	for _, f := range decl.Fields {
		if isID(f.Name) && isScalar(s.doc, f.Type) {
			return t.ID, f.Name
		}
	}
	return "", ""
}

func isID(name string) bool {
	return name == "id" || strings.HasSuffix(name, "Id") || strings.HasSuffix(name, "ID")
}

func isScalar(doc *contract.Document, t *contract.Type) bool {
	name := ""
	switch t.Kind {
	case contract.Primitive:
		name = t.Name
	case contract.Ref:
		if decl, ok := doc.Types[t.ID]; ok && decl.Kind == contract.Primitive {
			name = decl.Primitive
		}
	}
	switch name {
	case "string", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		return true
	}
	return false
}

func key(v any) string {
	switch x := v.(type) {
	case json.Number:
		return x.String()
	case string:
		return x
	case int64:
		return fmt.Sprint(x)
	case float64:
		return fmt.Sprint(int64(x))
	}
	return fmt.Sprint(v)
}

func (s *state) table(typeID string) *table {
	tb, ok := s.tables[typeID]
	if !ok {
		tb = &table{rows: map[string]map[string]any{}}
		s.tables[typeID] = tb
	}
	return tb
}

func (s *state) put(typeID, idField string, row map[string]any) {
	tb := s.table(typeID)
	k := key(row[idField])
	if _, exists := tb.rows[k]; !exists {
		tb.order = append(tb.order, k)
	}
	tb.rows[k] = row
}

func (s *state) seed(p *contract.Procedure, elem *contract.Type, typeID, idField string) {
	tb := s.table(typeID)
	if len(tb.order) > 0 {
		return
	}
	for i := 0; i < 5; i++ {
		row, ok := s.gen.Value(elem, p.Path, []string{"seed", fmt.Sprint(i)}).(map[string]any)
		if !ok {
			return
		}
		s.put(typeID, idField, row)
	}
}

func (s *state) get(p *contract.Procedure, input map[string]any) (any, bool) {
	typeID, idField := s.storedType(p.Output)
	if typeID == "" {
		return nil, false
	}
	inputID := ""
	count := 0
	for k, v := range input {
		if isID(k) {
			inputID = key(v)
			count++
		}
	}
	if count != 1 {
		return nil, false
	}
	tb := s.table(typeID)
	if row, ok := tb.rows[inputID]; ok {
		return row, true
	}
	row, ok := s.gen.Value(p.Output, p.Path, nil).(map[string]any)
	if !ok {
		return nil, false
	}
	row[idField] = idValue(input, inputID)
	s.put(typeID, idField, row)
	return row, true
}

func idValue(input map[string]any, fallback string) any {
	for k, v := range input {
		if isID(k) {
			return v
		}
	}
	return fallback
}

func (s *state) list(p *contract.Procedure) (any, bool) {
	out := p.Output
	if out == nil {
		return nil, false
	}
	if out.Kind == contract.Array {
		typeID, idField := s.storedType(out.Elem)
		if typeID == "" {
			return nil, false
		}
		s.seed(p, out.Elem, typeID, idField)
		return s.rows(typeID), true
	}
	fields, env := s.fieldsOf(out)
	if fields == nil {
		return nil, false
	}
	var listField *contract.Field
	for _, f := range fields {
		if f.Type.Kind == contract.Array {
			if listField != nil {
				return nil, false
			}
			listField = f
		}
	}
	if listField == nil {
		return nil, false
	}
	elem := shape.Substitute(listField.Type.Elem, env)
	typeID, idField := s.storedType(elem)
	if typeID == "" {
		return nil, false
	}
	s.seed(p, elem, typeID, idField)
	wrapper, ok := s.gen.Value(out, p.Path, nil).(map[string]any)
	if !ok {
		return nil, false
	}
	wrapper[listField.Name] = s.rows(typeID)
	return wrapper, true
}

func (s *state) fieldsOf(t *contract.Type) ([]*contract.Field, map[string]*contract.Type) {
	switch t.Kind {
	case contract.Struct:
		return t.Fields, nil
	case contract.Ref:
		decl, ok := s.doc.Types[t.ID]
		if !ok {
			return nil, nil
		}
		switch decl.Kind {
		case contract.Struct:
			return decl.Fields, nil
		case contract.Generic:
			env := map[string]*contract.Type{}
			for i, param := range decl.Params {
				if i < len(t.Args) {
					env[param] = t.Args[i]
				}
			}
			if decl.Body != nil {
				return decl.Body.Fields, env
			}
		}
	}
	return nil, nil
}

func (s *state) rows(typeID string) []any {
	tb := s.table(typeID)
	out := make([]any, 0, len(tb.order))
	for _, k := range tb.order {
		out = append(out, tb.rows[k])
	}
	return out
}

func (s *state) create(p *contract.Procedure, input map[string]any) (any, bool) {
	typeID, idField := s.storedType(p.Output)
	if typeID == "" {
		return nil, false
	}
	row, ok := s.gen.Value(p.Output, p.Path, []string{"create", fmt.Sprint(len(s.table(typeID).order))}).(map[string]any)
	if !ok {
		return nil, false
	}
	outFields, _ := s.fieldsOf(p.Output)
	inFields, _ := s.fieldsOf(p.Input)
	kinds := map[string]contract.Kind{}
	for _, f := range inFields {
		kinds[f.Name] = f.Type.Kind
	}
	for _, f := range outFields {
		if v, present := input[f.Name]; present && kinds[f.Name] == f.Type.Kind && f.Name != idField {
			row[f.Name] = v
		}
	}
	row[idField] = json.Number(fmt.Sprint(len(s.table(typeID).order) + 1))
	if isStringID(s.doc, outFields, idField) {
		row[idField] = fmt.Sprint(len(s.table(typeID).order) + 1)
	}
	s.put(typeID, idField, row)
	return row, true
}

func isStringID(doc *contract.Document, fields []*contract.Field, name string) bool {
	for _, f := range fields {
		if f.Name == name {
			t := f.Type
			if t.Kind == contract.Primitive {
				return t.Name == "string" || t.Encoding == "string"
			}
			if decl, ok := doc.Types[t.ID]; ok && decl.Kind == contract.Primitive {
				return decl.Primitive == "string"
			}
		}
	}
	return false
}
