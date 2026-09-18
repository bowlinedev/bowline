package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const Version = "1.2"

type Kind string

const (
	Primitive Kind = "primitive"
	Ref       Kind = "ref"
	Array     Kind = "array"
	Map       Kind = "map"
	Struct    Kind = "struct"
	Enum      Kind = "enum"
	Generic   Kind = "generic"
	Param     Kind = "param"
)

type Document struct {
	Bowline    string                `json:"bowline"`
	Hash       string                `json:"hash,omitempty"`
	Types      map[string]*TypeDecl  `json:"types"`
	Errors     map[string]*ErrorDecl `json:"errors"`
	Procedures []*Procedure          `json:"procedures"`
	Positions  map[string]Position   `json:"positions,omitempty"`
}

type ErrorDecl struct {
	Name   string   `json:"name"`
	Code   string   `json:"code"`
	Doc    string   `json:"doc,omitempty"`
	Fields []*Field `json:"fields,omitempty"`
}

type TypeDecl struct {
	Kind      Kind        `json:"kind"`
	Name      string      `json:"name"`
	Doc       string      `json:"doc,omitempty"`
	Fields    []*Field    `json:"fields,omitempty"`
	Base      string      `json:"base,omitempty"`
	Values    []EnumValue `json:"values,omitempty"`
	Params    []string    `json:"params,omitempty"`
	Body      *Type       `json:"body,omitempty"`
	Primitive string      `json:"primitive,omitempty"`
	Origin    string      `json:"origin,omitempty"`
}

type Type struct {
	Kind     Kind     `json:"kind"`
	Name     string   `json:"name,omitempty"`
	Encoding string   `json:"encoding,omitempty"`
	ID       string   `json:"id,omitempty"`
	Args     []*Type  `json:"args,omitempty"`
	Elem     *Type    `json:"elem,omitempty"`
	Length   int      `json:"length,omitempty"`
	Key      *Type    `json:"key,omitempty"`
	Value    *Type    `json:"value,omitempty"`
	Fields   []*Field `json:"fields,omitempty"`
	Nullable bool     `json:"nullable,omitempty"`
}

type Field struct {
	Name     string `json:"name"`
	Type     *Type  `json:"type"`
	Doc      string `json:"doc,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
	Rules    []Rule `json:"rules,omitempty"`
	Example  any    `json:"example,omitempty"`
}

type Rule struct {
	Rule  string `json:"rule"`
	Param string `json:"param,omitempty"`
}

type EnumValue struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type Procedure struct {
	Path       string            `json:"path"`
	Kind       string            `json:"kind"`
	Method     string            `json:"method"`
	HTTPPath   string            `json:"httpPath,omitempty"`
	Input      *Type             `json:"input"`
	Output     *Type             `json:"output"`
	GoInput    string            `json:"goInput,omitempty"`
	GoOutput   string            `json:"goOutput,omitempty"`
	Errors     []string          `json:"errors,omitempty"`
	Idempotent bool              `json:"idempotent,omitempty"`
	Tool       *Tool             `json:"tool,omitempty"`
	Schemas    *Schemas          `json:"schemas,omitempty"`
	Doc        string            `json:"doc,omitempty"`
	Deprecated string            `json:"deprecated,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

type Tool struct {
	Scopes      []string `json:"scopes,omitempty"`
	ReadOnly    bool     `json:"readOnly,omitempty"`
	Destructive bool     `json:"destructive,omitempty"`
}

type Schemas struct {
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
}

func (s *Schemas) InputOrNil() json.RawMessage {
	if s == nil {
		return nil
	}
	return s.Input
}

func (s *Schemas) OutputOrNil() json.RawMessage {
	if s == nil {
		return nil
	}
	return s.Output
}

type Position struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

func (d *Document) Marshal() ([]byte, error) {
	sorted := *d
	sorted.Procedures = slices.Clone(d.Procedures)
	slices.SortStableFunc(sorted.Procedures, func(a, b *Procedure) int {
		return strings.Compare(a.Path, b.Path)
	})
	if sorted.Types == nil {
		sorted.Types = map[string]*TypeDecl{}
	}
	if sorted.Errors == nil {
		sorted.Errors = map[string]*ErrorDecl{}
	}
	if sorted.Procedures == nil {
		sorted.Procedures = []*Procedure{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&sorted); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Parse(data []byte) (*Document, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("contract: parse: %w", err)
	}
	if err := checkVersion(doc.Bowline); err != nil {
		return nil, err
	}
	return &doc, nil
}

func checkVersion(v string) error {
	major, _, ok := strings.Cut(v, ".")
	wantMajor, _, _ := strings.Cut(Version, ".")
	if !ok || major != wantMajor {
		return fmt.Errorf("contract: document version %q is not compatible with reader version %q; run bowline migrate-contract", v, Version)
	}
	return nil
}
