package bowline

import (
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func documentFor(r *Router, mutate func(*contract.Document)) []byte {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{}}
	for _, p := range r.Procedures() {
		doc.Procedures = append(doc.Procedures, &contract.Procedure{
			Path:     p.Path,
			Kind:     string(p.Kind),
			Method:   p.Method(),
			Input:    &contract.Type{Kind: contract.Ref, ID: contract.GoTypeName(p.In)},
			Output:   &contract.Type{Kind: contract.Ref, ID: contract.GoTypeName(p.Out)},
			GoInput:  contract.GoTypeName(p.In),
			GoOutput: contract.GoTypeName(p.Out),
		})
	}
	if mutate != nil {
		mutate(doc)
	}
	data, err := doc.Marshal()
	if err != nil {
		panic(err)
	}
	return data
}

func TestVerifyAcceptsMatchingDocument(t *testing.T) {
	r := NewRouter(Mount("users", NewRouter(Query("get", getUser))), Query("health", health))
	if err := r.Verify(documentFor(r, nil)); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyReportsEveryDifference(t *testing.T) {
	r := NewRouter(Mount("users", NewRouter(Query("get", getUser), Mutation("create", createUser))))
	data := documentFor(r, func(d *contract.Document) {
		d.Procedures[0].Method = "POST"
		d.Procedures[1].GoOutput = "example.com/other.Type"
		d.Procedures = append(d.Procedures, &contract.Procedure{Path: "users.delete", Kind: "mutation", Method: "POST", Input: &contract.Type{Kind: contract.Ref}, Output: &contract.Type{Kind: contract.Ref}})
	})
	err := r.Verify(data)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"users.get: method", "users.create: output", "users.delete: in contract but not registered"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in:\n%s", want, msg)
		}
	}
}

func TestVerifyReportsUnregisteredInContract(t *testing.T) {
	r := NewRouter(Query("a", getUser), Query("b", getUser))
	data := documentFor(NewRouter(Query("a", getUser)), nil)
	err := r.Verify(data)
	if err == nil || !strings.Contains(err.Error(), "b: registered but not in contract") {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyRejectsUnparseable(t *testing.T) {
	if err := NewRouter().Verify([]byte("{")); err == nil {
		t.Fatal("expected parse error")
	}
}
