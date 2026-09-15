package python

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/fake"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/goldens"
	"github.com/bowlinedev/bowline/contract"
)

func TestGoldens(t *testing.T) {
	goldens.Run(t, "python", "py", Generator{})
	for row, doc := range goldens.Rows(t) {
		t.Run(row+"/samples", func(t *testing.T) {
			models := Models(doc)
			gen := fake.New(doc, 1)
			samples := map[string]map[string]any{}
			for _, p := range doc.Procedures {
				samples[p.Path] = map[string]any{
					"model":  models[p.Path].Model,
					"input":  models[p.Path].Input,
					"output": gen.Output(p),
				}
			}
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			if err := enc.Encode(samples); err != nil {
				t.Fatal(err)
			}
			goldens.Compare(t, "python", filepath.Join("testdata", row+".samples.json"), buf.Bytes())
		})
	}
}

func generate(t *testing.T, doc *contract.Document) string {
	t.Helper()
	out, err := Generator{}.Generate(doc, "bowline.py")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func structDoc(types map[string]*contract.TypeDecl, procs ...*contract.Procedure) *contract.Document {
	return &contract.Document{Bowline: contract.Version, Types: types, Errors: map[string]*contract.ErrorDecl{}, Procedures: procs}
}

func TestOneofBecomesLiteral(t *testing.T) {
	doc := structDoc(map[string]*contract.TypeDecl{
		"app.Status": {Kind: contract.Enum, Name: "Status", Base: "string", Values: []contract.EnumValue{{Name: "StatusDraft", Value: "draft"}, {Name: "StatusSent", Value: "sent"}, {Name: "statusVoid", Value: "void"}}},
		"app.Order": {Kind: contract.Struct, Name: "Order", Fields: []*contract.Field{
			{Name: "kind", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}, Rules: []contract.Rule{{Rule: "oneof", Param: "a b"}}},
			{Name: "status", Type: &contract.Type{Kind: contract.Ref, ID: "app.Status"}, Rules: []contract.Rule{{Rule: "oneof", Param: "draft void"}}},
			{Name: "level", Type: &contract.Type{Kind: contract.Primitive, Name: "int32"}, Rules: []contract.Rule{{Rule: "oneof", Param: "1 10"}}},
		}},
	})
	out := generate(t, doc)
	for _, want := range []string{
		`    kind: Literal["a", "b"]`,
		`    status: Literal[Status.StatusDraft, Status.StatusVoid]`,
		`    level: Literal[1, 10]`,
		`    StatusVoid = "void"`,
		"from typing import Annotated, Literal",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestGenericsEmitTypeVars(t *testing.T) {
	doc := structDoc(map[string]*contract.TypeDecl{
		"app.Page": {Kind: contract.Generic, Name: "Page", Params: []string{"T"}, Body: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{
			{Name: "items", Type: &contract.Type{Kind: contract.Array, Elem: &contract.Type{Kind: contract.Param, Name: "T"}}},
			{Name: "next", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}, Optional: true},
		}}},
		"app.User": {Kind: contract.Struct, Name: "User", Fields: []*contract.Field{{Name: "name", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}}}},
	}, &contract.Procedure{Path: "users.list", Kind: "query", Method: "GET", Input: &contract.Type{Kind: contract.Struct}, Output: &contract.Type{Kind: contract.Ref, ID: "app.Page", Args: []*contract.Type{{Kind: contract.Ref, ID: "app.User"}}}})
	out := generate(t, doc)
	for _, want := range []string{
		`T = TypeVar("T")`,
		"class Page(BaseModel, Generic[T]):",
		"    items: list[T]",
		"    next: str | None = None",
		"    ) -> Page[User]:",
		"        return await self._transport.call(\n            \"users.list\", Method.GET, input or Empty(), Page[User], options\n        )",
		"        self.users = UsersClient(transport)",
		"class SyncUsersClient:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestAliasesForNonIdentifierNames(t *testing.T) {
	doc := structDoc(map[string]*contract.TypeDecl{
		"app.Row": {Kind: contract.Struct, Name: "Row", Fields: []*contract.Field{
			{Name: "x-y", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}},
			{Name: "class", Type: &contract.Type{Kind: contract.Primitive, Name: "bool"}},
			{Name: "model_id", Type: &contract.Type{Kind: contract.Primitive, Name: "int64"}},
			{Name: "plain", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}, Doc: "Plain text."},
		}},
	})
	out := generate(t, doc)
	for _, want := range []string{
		`    model_config = ConfigDict(extra="ignore", populate_by_name=True)`,
		`    x_y: str = Field(alias="x-y")`,
		`    class_: bool = Field(alias="class")`,
		`    field_model_id: int = Field(alias="model_id")`,
		`    plain: str = Field(description="Plain text.")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestRecursiveTypesAreRebuilt(t *testing.T) {
	node := &contract.Type{Kind: contract.Ref, ID: "app.Node"}
	doc := structDoc(map[string]*contract.TypeDecl{
		"app.Node": {Kind: contract.Struct, Name: "Node", Fields: []*contract.Field{
			{Name: "children", Type: &contract.Type{Kind: contract.Array, Elem: node}},
			{Name: "leaf", Type: &contract.Type{Kind: contract.Ref, ID: "app.Leaf"}},
		}},
		"app.Leaf": {Kind: contract.Struct, Name: "Leaf", Fields: []*contract.Field{{Name: "v", Type: &contract.Type{Kind: contract.Primitive, Name: "int32"}}}},
		"app.A":    {Kind: contract.Struct, Name: "A", Fields: []*contract.Field{{Name: "b", Type: &contract.Type{Kind: contract.Ref, ID: "app.B"}, Optional: true}}},
		"app.B":    {Kind: contract.Struct, Name: "B", Fields: []*contract.Field{{Name: "a", Type: &contract.Type{Kind: contract.Ref, ID: "app.A"}, Optional: true}}},
	})
	out := generate(t, doc)
	leaf := strings.Index(out, "class Leaf(")
	nodeAt := strings.Index(out, "class Node(")
	if leaf < 0 || nodeAt < 0 || leaf > nodeAt {
		t.Fatalf("Leaf must precede Node:\n%s", out)
	}
	rebuild := strings.Index(out, "A.model_rebuild()\nB.model_rebuild()\nNode.model_rebuild()\n")
	if rebuild < 0 || rebuild < strings.Index(out, "class B(") {
		t.Fatalf("missing rebuild block after the models:\n%s", out)
	}
	if strings.Contains(out, "Leaf.model_rebuild()") {
		t.Fatalf("Leaf is not recursive:\n%s", out)
	}
}

func TestInlineStructsAreSynthesized(t *testing.T) {
	doc := structDoc(map[string]*contract.TypeDecl{
		"app.Person": {Kind: contract.Struct, Name: "Person", Fields: []*contract.Field{
			{Name: "home", Type: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{{Name: "city", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}}}}},
		}},
	})
	out := generate(t, doc)
	home := strings.Index(out, "class Person_Home(BaseModel):")
	person := strings.Index(out, "class Person(BaseModel):")
	if home < 0 || person < 0 || home > person {
		t.Fatalf("inline model must precede its parent:\n%s", out)
	}
	if !strings.Contains(out, "    home: Person_Home\n") {
		t.Fatalf("field must reference the synthesized model:\n%s", out)
	}
}
