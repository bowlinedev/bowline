package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func ledger(t *testing.T) *contract.Document {
	t.Helper()
	doc, err := Load(filepath.Join("..", "examples", "ledger", "api", "bowline.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func names(list []Tool) []string {
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, t.Name)
	}
	return out
}

func TestToolsFromLedger(t *testing.T) {
	list, err := Tools(ledger(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(list), ","); got != "customers_search,invoices_get,invoices_list,invoices_void" {
		t.Fatalf("tools %s", got)
	}
	void := list[3]
	if !void.Destructive || void.ReadOnly || void.Procedure != "invoices.void" || len(void.InputSchema) == 0 || len(void.OutputSchema) == 0 {
		t.Fatalf("void %+v", void)
	}
	if !strings.Contains(void.Description, "Errors: InvoiceLocked") {
		t.Fatalf("description %q", void.Description)
	}
	if !list[1].ReadOnly || strings.Join(list[1].Scopes, ",") != "billing" {
		t.Fatalf("get %+v", list[1])
	}
}

func TestToolsFilters(t *testing.T) {
	doc := ledger(t)
	crm, err := Tools(doc, Scopes("crm"))
	if err != nil || strings.Join(names(crm), ",") != "customers_search" {
		t.Fatalf("crm %v %v", names(crm), err)
	}
	readOnly, err := Tools(doc, ReadOnly())
	if err != nil || strings.Join(names(readOnly), ",") != "customers_search,invoices_get,invoices_list" {
		t.Fatalf("read-only %v %v", names(readOnly), err)
	}
	none, err := Tools(doc, Scopes("nope"))
	if err != nil || len(none) != 0 {
		t.Fatalf("nope %v %v", names(none), err)
	}
}

func TestToolsRequireSchemas(t *testing.T) {
	doc := ledger(t)
	for _, p := range doc.Procedures {
		if p.Path == "invoices.get" {
			p.Schemas = nil
		}
	}
	_, err := Tools(doc)
	if err == nil || !strings.Contains(err.Error(), "invoices.get") || !strings.Contains(err.Error(), `"schemas": true`) {
		t.Fatalf("got %v", err)
	}
}
