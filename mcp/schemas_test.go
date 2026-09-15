package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func TestSchemasFromContractRequiresSchemas(t *testing.T) {
	doc := testDocument()
	for _, p := range doc.Procedures {
		p.Schemas = nil
	}
	if _, err := SchemasFromContract(doc); !errors.Is(err, ErrNoSchemas) {
		t.Fatalf("err = %v", err)
	}
	doc = testDocument()
	doc.Procedures = doc.Procedures[2:]
	if _, err := SchemasFromContract(doc); err != nil {
		t.Fatalf("contract without tools should not need schemas: %v", err)
	}
}

func TestSchemasFromContractLookup(t *testing.T) {
	source, err := SchemasFromContract(testDocument())
	if err != nil {
		t.Fatal(err)
	}
	input, output, ok := source.Schemas("invoices.get")
	if !ok || string(input) != getInputSchema || string(output) != invoiceSchema {
		t.Errorf("lookup = %v %s %s", ok, input, output)
	}
	if _, _, ok := source.Schemas("nope"); ok {
		t.Error("unknown procedure reported schemas")
	}
}

func TestToolsFromContract(t *testing.T) {
	tools := testTools(t)
	if len(tools) != 2 {
		t.Fatalf("tools = %+v", tools)
	}
	get := tools[0]
	if get.Name != "invoices_get" || get.Procedure != "invoices.get" || get.Method != "GET" {
		t.Errorf("get = %+v", get)
	}
	if !get.ReadOnly || get.Destructive || len(get.Scopes) != 1 || get.Scopes[0] != "invoices:read" {
		t.Errorf("get flags = %+v", get)
	}
	if !strings.HasSuffix(get.Description, "\nErrors: InvoiceNotFound") {
		t.Errorf("description = %q", get.Description)
	}
	create := tools[1]
	if create.Name != "invoices_create" || create.Method != "POST" || !create.Destructive || create.ReadOnly {
		t.Errorf("create = %+v", create)
	}
	if create.Description != "Create makes an invoice." {
		t.Errorf("description = %q", create.Description)
	}
}

func TestToolsFromContractNameCollision(t *testing.T) {
	doc := testDocument()
	doc.Procedures = append(doc.Procedures, &contract.Procedure{
		Path:    "invoices_get",
		Kind:    "query",
		Method:  "GET",
		Tool:    &contract.Tool{},
		Schemas: &contract.Schemas{Input: json.RawMessage(`{}`), Output: json.RawMessage(`{}`)},
	})
	source, err := SchemasFromContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ToolsFromContract(doc, source); err == nil || !strings.Contains(err.Error(), "invoices_get") {
		t.Fatalf("err = %v", err)
	}
}

type emptySource struct{}

func (emptySource) Schemas(string) (json.RawMessage, json.RawMessage, bool) {
	return nil, nil, false
}

func TestToolsFromContractMissingSchema(t *testing.T) {
	if _, err := ToolsFromContract(testDocument(), emptySource{}); err == nil || !strings.Contains(err.Error(), "invoices.get") {
		t.Fatalf("err = %v", err)
	}
}
