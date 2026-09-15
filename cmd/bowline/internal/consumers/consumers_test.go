package consumers

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var update = flag.Bool("update", false, "rewrite golden files")

func ledger(t *testing.T) *contract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "examples", "ledger", "api", "bowline.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

const consumerFile = `{
  "bowline": "1.2",
  "consumer": "ledger-web",
  "provider": "ledger",
  "interactions": [
    {"procedure": "invoices.list", "method": "POST", "input": {"limit": 20},
     "response": {"status": 200, "body": {"items": [{"id": 3, "customerId": 1, "status": "sent", "total": "USD 1500.00", "lines": [{"description": "Consulting", "quantity": 10, "unitPrice": "USD 150.00"}], "note": "net 30", "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z"}]}}},
    {"procedure": "invoices.get", "method": "GET", "input": {"id": 999},
     "response": {"status": 404, "body": {"error": {"code": "NOT_FOUND", "message": "invoice 999 not found"}}}},
    {"procedure": "invoices.void", "method": "POST", "input": {"id": 4},
     "response": {"status": 412, "body": {"error": {"code": "FAILED_PRECONDITION", "message": "invoice 4 is paid", "type": "InvoiceLocked", "details": {"id": 4, "status": "paid"}}}}},
    {"procedure": "customers.search", "method": "POST", "input": {"query": "ada"},
     "response": {"status": 200, "body": {"items": [{"id": 1, "name": "Ada Lovelace", "email": "ada@example.com", "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z"}]}}}
  ]
}`

func consumer(t *testing.T) Consumer {
	t.Helper()
	c, err := Parse([]byte(consumerFile))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func find(doc *contract.Document, path string) *contract.Procedure {
	for _, p := range doc.Procedures {
		if p.Path == path {
			return p
		}
	}
	return nil
}

func typeByName(doc *contract.Document, name string) *contract.TypeDecl {
	for _, decl := range doc.Types {
		if decl.Name == name {
			return decl
		}
	}
	return nil
}

func TestVerifyGoldens(t *testing.T) {
	cases := map[string]func(doc *contract.Document){
		"unchanged": func(doc *contract.Document) {},
		"removed-procedure": func(doc *contract.Document) {
			var kept []*contract.Procedure
			for _, p := range doc.Procedures {
				if p.Path != "customers.search" {
					kept = append(kept, p)
				}
			}
			doc.Procedures = kept
		},
		"changed-method": func(doc *contract.Document) {
			find(doc, "invoices.get").Method = "POST"
		},
		"removed-response-field": func(doc *contract.Document) {
			decl := typeByName(doc, "Invoice")
			var kept []*contract.Field
			for _, f := range decl.Fields {
				if f.Name != "total" {
					kept = append(kept, f)
				}
			}
			decl.Fields = kept
		},
		"widened-field-passes": func(doc *contract.Document) {
			decl := typeByName(doc, "Invoice")
			for _, f := range decl.Fields {
				if f.Name == "note" {
					f.Nullable = true
				}
			}
			decl.Fields = append(decl.Fields, &contract.Field{Name: "dueAt", Type: &contract.Type{Kind: contract.Primitive, Name: "timestamp"}, Optional: true})
		},
		"enum-value-removed": func(doc *contract.Document) {
			decl := typeByName(doc, "Status")
			var kept []contract.EnumValue
			for _, v := range decl.Values {
				if v.Value != "sent" {
					kept = append(kept, v)
				}
			}
			decl.Values = kept
		},
		"new-required-input": func(doc *contract.Document) {
			decl := typeByName(doc, "SearchCustomersInput")
			decl.Fields = append(decl.Fields, &contract.Field{Name: "tenant", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}, Rules: []contract.Rule{{Rule: "required"}}})
		},
		"error-variant-removed": func(doc *contract.Document) {
			find(doc, "invoices.void").Errors = nil
		},
		"type-changed": func(doc *contract.Document) {
			decl := typeByName(doc, "Customer")
			for _, f := range decl.Fields {
				if f.Name == "email" {
					f.Type = &contract.Type{Kind: contract.Primitive, Name: "int64"}
				}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			doc := ledger(t)
			mutate(doc)
			list := []Consumer{consumer(t)}
			got := Report(Verify(doc, list), list)
			golden := filepath.Join("testdata", name+".golden.txt")
			if *update {
				os.WriteFile(golden, []byte(got), 0o644)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v\n%s", err, got)
			}
			if !bytes.Equal([]byte(got), want) {
				t.Fatalf("golden mismatch; run go test ./internal/consumers -update after reviewing\n%s", got)
			}
			if strings.HasSuffix(name, "passes") || name == "unchanged" {
				if !strings.HasPrefix(got, "ok") {
					t.Fatalf("expected no problems:\n%s", got)
				}
			} else if !strings.Contains(got, "broken") {
				t.Fatalf("expected a problem:\n%s", got)
			}
		})
	}
}

func TestLoadAndTouches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ledger-web.json"), []byte(consumerFile), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o644)
	list, err := Load(dir)
	if err != nil || len(list) != 1 || list[0].Consumer != "ledger-web" {
		t.Fatalf("%v %+v", err, list)
	}
	c := list[0]
	if n := Touches(c, "invoices.list", "output", []string{"items", "*", "total"}); n != 1 {
		t.Fatalf("total touched %d", n)
	}
	if n := Touches(c, "invoices.list", "output", []string{"items", "*", "dueAt"}); n != 0 {
		t.Fatalf("dueAt touched %d", n)
	}
	if n := Touches(c, "invoices.get", "output", nil); n != 0 {
		t.Fatalf("error responses must not count as output use: %d", n)
	}
	if n := Touches(c, "customers.search", "input", []string{"query"}); n != 1 {
		t.Fatalf("query touched %d", n)
	}
	if _, err := Parse([]byte(`{"bowline":"2.0","consumer":"x","interactions":[]}`)); err == nil || !strings.Contains(err.Error(), "2.0") {
		t.Fatalf("version check: %v", err)
	}
	if _, err := Parse([]byte(`{"bowline":"1.2","interactions":[]}`)); err == nil {
		t.Fatal("missing consumer name accepted")
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(consumerFile), &raw); err != nil {
		t.Fatal(err)
	}
}
