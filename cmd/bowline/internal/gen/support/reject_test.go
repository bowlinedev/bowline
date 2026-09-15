package support

import (
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func base() *contract.Document {
	return &contract.Document{
		Bowline: contract.Version,
		Types:   map[string]*contract.TypeDecl{},
		Errors:  map[string]*contract.ErrorDecl{},
	}
}

func TestRejectNamesTheTypeAndTheTarget(t *testing.T) {
	cases := map[string]struct {
		mutate func(*contract.Document)
		want   []string
	}{
		"unknown declaration kind": {
			func(d *contract.Document) {
				d.Types["example.com/app.Money"] = &contract.TypeDecl{Kind: contract.Kind("union"), Name: "Money"}
			},
			[]string{"example.com/app.Money", `"union"`},
		},
		"unknown nominal primitive": {
			func(d *contract.Document) {
				d.Types["example.com/app.Amount"] = &contract.TypeDecl{Kind: contract.Primitive, Name: "Amount", Primitive: "decimal"}
			},
			[]string{"example.com/app.Amount", `"decimal"`},
		},
		"unknown node kind in a field": {
			func(d *contract.Document) {
				d.Types["example.com/app.User"] = &contract.TypeDecl{Kind: contract.Struct, Name: "User", Fields: []*contract.Field{
					{Name: "tags", Type: &contract.Type{Kind: contract.Kind("set"), Elem: &contract.Type{Kind: contract.Primitive, Name: "string"}}},
				}}
			},
			[]string{"example.com/app.User.tags", `"set"`},
		},
		"unknown primitive in an output": {
			func(d *contract.Document) {
				d.Procedures = append(d.Procedures, &contract.Procedure{
					Path: "users.get", Kind: "query", Method: "GET",
					Input:  &contract.Type{Kind: contract.Struct},
					Output: &contract.Type{Kind: contract.Array, Elem: &contract.Type{Kind: contract.Primitive, Name: "decimal"}},
				})
			},
			[]string{"users.get output", `"decimal"`},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			doc := base()
			c.mutate(doc)
			err := Reject(doc, "dart")
			if err == nil {
				t.Fatal("expected a rejection")
			}
			for _, want := range append(c.want, "dart") {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

func TestRejectAcceptsEveryFidelityRow(t *testing.T) {
	doc := base()
	doc.Types["example.com/app.Page"] = &contract.TypeDecl{Kind: contract.Generic, Name: "Page", Params: []string{"T"}, Body: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{
		{Name: "items", Type: &contract.Type{Kind: contract.Array, Elem: &contract.Type{Kind: contract.Param, Name: "T"}}},
	}}}
	doc.Types["example.com/app.Status"] = &contract.TypeDecl{Kind: contract.Enum, Name: "Status", Base: "string", Values: []contract.EnumValue{{Name: "Draft", Value: "draft"}}}
	doc.Errors["example.com/app.Locked"] = &contract.ErrorDecl{Name: "Locked", Code: "FAILED_PRECONDITION", Fields: []*contract.Field{
		{Name: "at", Type: &contract.Type{Kind: contract.Primitive, Name: "timestamp"}},
	}}
	doc.Procedures = append(doc.Procedures, &contract.Procedure{
		Path: "users.list", Kind: "query", Method: "GET",
		Input:  &contract.Type{Kind: contract.Struct},
		Output: &contract.Type{Kind: contract.Ref, ID: "example.com/app.Page", Args: []*contract.Type{{Kind: contract.Primitive, Name: "bytes"}}},
	})
	if err := Reject(doc, "rust"); err != nil {
		t.Fatal(err)
	}
}

func TestEveryGeneratorRejectsAnUnrepresentableContract(t *testing.T) {
	for _, target := range []string{"ts", "go", "dart", "python", "rust", "elixir"} {
		t.Run(target, func(t *testing.T) {
			doc := base()
			doc.Types["example.com/app.Money"] = &contract.TypeDecl{Kind: contract.Kind("union"), Name: "Money"}
			err := Reject(doc, target)
			if err == nil || !strings.Contains(err.Error(), target) || !strings.Contains(err.Error(), "example.com/app.Money") {
				t.Fatalf("got %v", err)
			}
		})
	}
}
