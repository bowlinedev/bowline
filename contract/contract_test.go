package contract

import (
	"bytes"
	"strings"
	"testing"
)

func sampleDocument() *Document {
	return &Document{
		Bowline: Version,
		Types: map[string]*TypeDecl{
			"example.com/app/users.User": {
				Kind: Struct,
				Name: "User",
				Doc:  "A registered user.",
				Fields: []*Field{
					{Name: "id", Type: &Type{Kind: Primitive, Name: "int64"}},
					{Name: "email", Type: &Type{Kind: Primitive, Name: "string"}, Rules: []Rule{{Rule: "email"}}},
					{Name: "nickname", Type: &Type{Kind: Primitive, Name: "string"}, Optional: true, Nullable: true},
				},
			},
			"example.com/app/users.Status": {
				Kind:   Enum,
				Name:   "Status",
				Base:   "string",
				Values: []EnumValue{{Name: "StatusActive", Value: "active"}},
			},
		},
		Procedures: []*Procedure{
			{
				Path: "users.list", Kind: "query", Method: "GET",
				Input:    &Type{Kind: Ref, ID: "example.com/app/users.ListInput"},
				Output:   &Type{Kind: Ref, ID: "example.com/app/users.User"},
				GoInput:  "example.com/app/users.ListInput",
				GoOutput: "example.com/app/users.User",
			},
			{
				Path: "users.get", Kind: "query", Method: "GET",
				Input:    &Type{Kind: Ref, ID: "example.com/app/users.GetInput"},
				Output:   &Type{Kind: Ref, ID: "example.com/app/users.User"},
				GoInput:  "example.com/app/users.GetInput",
				GoOutput: "example.com/app/users.User",
				Doc:      "Get returns one user.",
			},
		},
		Positions: map[string]Position{
			"users.get": {File: "users/router.go", Line: 9},
		},
	}
}

func TestMarshalIsCanonical(t *testing.T) {
	doc := sampleDocument()
	first, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(first, []byte("\n")) {
		t.Fatal("output must end with a newline")
	}
	if !strings.HasPrefix(string(first), "{\n  \"bowline\": \"1.2\"") {
		t.Fatalf("unexpected prefix: %q", first[:40])
	}
	getIdx := strings.Index(string(first), `"path": "users.get"`)
	listIdx := strings.Index(string(first), `"path": "users.list"`)
	if getIdx < 0 || listIdx < 0 || getIdx > listIdx {
		t.Fatal("procedures must be sorted by path")
	}
	if strings.Contains(string(first), `"optional": false`) || strings.Contains(string(first), `"doc": ""`) {
		t.Fatal("zero-valued optional fields must be omitted")
	}
	second, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("marshal is not deterministic")
	}
}

func TestMarshalDoesNotReorderInput(t *testing.T) {
	doc := sampleDocument()
	if _, err := doc.Marshal(); err != nil {
		t.Fatal(err)
	}
	if doc.Procedures[0].Path != "users.list" {
		t.Fatal("Marshal must not mutate the document")
	}
}

func TestParseRoundTrip(t *testing.T) {
	data, err := sampleDocument().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := parsed.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, again) {
		t.Fatalf("round trip differs:\n%s\n---\n%s", data, again)
	}
}

func TestParseRejectsOtherMajor(t *testing.T) {
	_, err := Parse([]byte(`{"bowline":"2.0","types":{},"procedures":[]}`))
	if err == nil {
		t.Fatal("expected error for major version mismatch")
	}
	if !strings.Contains(err.Error(), "2.0") {
		t.Fatalf("error should name the version: %v", err)
	}
}

func TestParseAcceptsSameMajorHigherMinor(t *testing.T) {
	_, err := Parse([]byte(`{"bowline":"1.9","types":{},"procedures":[],"future":true}`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseKeepsIntegerEnumValues(t *testing.T) {
	doc, err := Parse([]byte(`{"bowline":"1.0","types":{"p.Level":{"kind":"enum","name":"Level","base":"int64","values":[{"name":"High","value":9007199254740993}]}},"procedures":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	got := doc.Types["p.Level"].Values[0].Value
	if s, ok := got.(interface{ String() string }); !ok || s.String() != "9007199254740993" {
		t.Fatalf("integer enum value lost precision: %#v", got)
	}
}
