package tools

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

var update = flag.Bool("update", false, "rewrite golden files")

func toolsDoc(t *testing.T) *contract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "analyzer", "testdata", "fidelity", "rows", "tools", "expected.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestEncodeGoldens(t *testing.T) {
	doc := toolsDoc(t)
	list, err := FromContract(doc, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Name != "get" || list[1].Name != "remove" || list[2].Name != "search" {
		t.Fatalf("tools %+v", list)
	}
	for _, format := range []string{FormatAnthropic, FormatOpenAI, FormatJSONSchema} {
		got, err := Encode(list, format)
		if err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("testdata", format+".golden.json")
		if *update {
			os.WriteFile(golden, got, 0o644)
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v\n%s", err, got)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("golden mismatch for %s", format)
		}
	}
}

func TestLedgerGoldens(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "examples", "ledger", "api", "bowline.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	list, err := FromContract(doc, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 4 || list[0].Name != "customers_search" || list[3].Name != "invoices_void" || !list[3].Destructive {
		t.Fatalf("tools %+v", list)
	}
	for _, format := range []string{FormatAnthropic, FormatOpenAI, FormatJSONSchema} {
		got, err := Encode(list, format)
		if err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("testdata", "ledger."+format+".golden.json")
		if *update {
			os.WriteFile(golden, got, 0o644)
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("ledger golden mismatch for %s; run go test ./internal/tools -update after reviewing", format)
		}
	}
}

func TestFilters(t *testing.T) {
	doc := toolsDoc(t)
	billing, _ := FromContract(doc, Filter{Scopes: []string{"billing"}})
	if len(billing) != 3 {
		t.Fatalf("billing %d", len(billing))
	}
	crm, _ := FromContract(doc, Filter{Scopes: []string{"crm"}})
	if len(crm) != 1 || crm[0].Name != "search" {
		t.Fatalf("crm %+v", crm)
	}
	readOnly, _ := FromContract(doc, Filter{ReadOnly: true})
	if len(readOnly) != 2 || readOnly[0].Name != "get" || readOnly[1].Name != "search" {
		t.Fatalf("read-only %+v", readOnly)
	}
	none, _ := FromContract(doc, Filter{Scopes: []string{"nope"}})
	if len(none) != 0 {
		t.Fatalf("nope %+v", none)
	}
}

func TestDescriptionCarriesErrors(t *testing.T) {
	data, _ := os.ReadFile(filepath.Join("..", "analyzer", "testdata", "fidelity", "rows", "errors", "expected.contract.json"))
	doc, _ := contract.Parse(data)
	for _, p := range doc.Procedures {
		if p.Path == "void" {
			if got := Describe(doc, p); got != "Errors: InvoiceLocked, QuotaExceeded" {
				t.Fatalf("got %q", got)
			}
		}
	}
}

func TestUnknownFormat(t *testing.T) {
	if _, err := Encode(nil, "gemini"); err == nil || !strings.Contains(err.Error(), "gemini") {
		t.Fatalf("got %v", err)
	}
}
