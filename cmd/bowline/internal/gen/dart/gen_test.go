package dart

import (
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/goldens"
	"github.com/bowlinedev/bowline/contract"
)

func TestGoldens(t *testing.T) {
	goldens.Run(t, "dart", "dart", Generator{})
}

func generate(t *testing.T, doc *contract.Document) string {
	t.Helper()
	out, err := Generator{}.Generate(doc, "bowline.dart")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestEnumEmission(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"app.Status": {Kind: contract.Enum, Name: "Status", Base: "string", Values: []contract.EnumValue{{Name: "StatusDraft", Value: "draft"}, {Name: "StatusIn", Value: "in"}}},
		"app.Level":  {Kind: contract.Enum, Name: "Level", Base: "int32", Values: []contract.EnumValue{{Name: "LevelLow", Value: int64(1)}, {Name: "LevelHigh", Value: int64(10)}}},
	}, Errors: map[string]*contract.ErrorDecl{}}
	out := generate(t, doc)
	for _, want := range []string{
		"enum Status {\n  draft('draft'),\n  in_('in');",
		"final String value;",
		"enum Level {\n  low(1),\n  high(10);",
		"final int value;",
		"static Status fromValue(Object? value)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestGenericFromJsonSignature(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"app.Page": {Kind: contract.Generic, Name: "Page", Params: []string{"T"}, Body: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{
			{Name: "items", Type: &contract.Type{Kind: contract.Array, Elem: &contract.Type{Kind: contract.Param, Name: "T"}}},
			{Name: "next", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}, Optional: true},
		}}},
		"app.User": {Kind: contract.Struct, Name: "User", Fields: []*contract.Field{{Name: "name", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}}}},
	}, Errors: map[string]*contract.ErrorDecl{}, Procedures: []*contract.Procedure{
		{Path: "users.list", Kind: "query", Method: "GET", Input: &contract.Type{Kind: contract.Struct}, Output: &contract.Type{Kind: contract.Ref, ID: "app.Page", Args: []*contract.Type{{Kind: contract.Ref, ID: "app.User"}}}},
	}}
	out := generate(t, doc)
	for _, want := range []string{
		"class Page<T> {",
		"factory Page.fromJson(Map<String, Object?> json, T Function(Object?) fromJsonT) => Page(",
		"items: readList(json, 'items', fromJsonT),",
		"next: readOptionalString(json, 'next'),",
		"Map<String, Object?> toJson(Object? Function(T) toJsonT) => {",
		"'items': [for (final e0 in items) toJsonT(e0)],",
		"if (next != null) 'next': next,",
		"List<Issue> validate(List<Issue> Function(T) validateT) => [",
		"for (final (i0, e0) in items.indexed) ...[...prefixed(['items', '$i0'], validateT(e0))],",
		"Future<Page<User>> list({CallOptions? options}) {",
		"(json) => Page.fromJson(asObject(json), (v0) => User.fromJson(asObject(v0)))",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestInlineStructNaming(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"app.Person": {Kind: contract.Struct, Name: "Person", Fields: []*contract.Field{
			{Name: "home", Type: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{{Name: "city", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}, Rules: []contract.Rule{{Rule: "required"}}}}}},
			{Name: "stops", Type: &contract.Type{Kind: contract.Array, Elem: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{{Name: "at", Type: &contract.Type{Kind: contract.Primitive, Name: "timestamp"}}}}}},
		}},
	}, Errors: map[string]*contract.ErrorDecl{}, Procedures: []*contract.Procedure{
		{Path: "get", Kind: "query", Method: "GET", Input: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{{Name: "id", Type: &contract.Type{Kind: contract.Primitive, Name: "int64"}}}}, Output: &contract.Type{Kind: contract.Ref, ID: "app.Person"}},
	}}
	out := generate(t, doc)
	for _, want := range []string{
		"class PersonHome {",
		"class PersonStopsItem {",
		"class GetInput {",
		"final PersonHome home;",
		"final List<PersonStopsItem> stops;",
		"...prefixed(['home'], home.validate()),",
		"Future<Person> get(GetInput input, {CallOptions? options}) {",
		"'at': encodeTimestamp(at),",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestReservedAndCollidingNames(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"app.String":      {Kind: contract.Struct, Name: "String", Fields: []*contract.Field{{Name: "class", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}}, {Name: "toJson", Type: &contract.Type{Kind: contract.Primitive, Name: "bool"}}}},
		"app/a.Event":     {Kind: contract.Struct, Name: "Event"},
		"app/b.Event":     {Kind: contract.Struct, Name: "Event"},
		"app.Range_int64": {Kind: contract.Struct, Name: "Range_int64", Origin: "app.Range"},
	}, Errors: map[string]*contract.ErrorDecl{}}
	out := generate(t, doc)
	for _, want := range []string{"class String_ {", "final String class_;", "final bool toJson_;", "class AEvent {", "class BEvent {", "class RangeInt64 {"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestRejectsAnUnrepresentableContract(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"example.com/app.Money": {Kind: contract.Kind("union"), Name: "Money"},
	}, Errors: map[string]*contract.ErrorDecl{}}
	_, err := Generator{}.Generate(doc, "bowline.out")
	if err == nil || !strings.Contains(err.Error(), "dart") || !strings.Contains(err.Error(), "example.com/app.Money") {
		t.Fatalf("got %v", err)
	}
}
