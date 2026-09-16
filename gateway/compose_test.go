package gateway

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

func load(t *testing.T, name string) *contract.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "compose", name))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := contract.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func golden(t *testing.T, name string, doc *contract.Document) {
	t.Helper()
	got, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "compose", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v\n%s", err, got)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden %s differs; run go test ./gateway -update after review:\n%s", name, got)
	}
}

func TestComposeTwoServices(t *testing.T) {
	composed, diags := Compose(map[string]*contract.Document{
		"ledger":  load(t, "ledger.contract.json"),
		"billing": load(t, "billing.contract.json"),
	})
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	golden(t, "two-services.golden.json", composed)

	paths := make([]string, 0, len(composed.Procedures))
	for _, p := range composed.Procedures {
		paths = append(paths, p.Path)
	}
	want := "billing.charges.create,ledger.invoices.attach,ledger.invoices.get,ledger.invoices.list,ledger.invoices.watch"
	if strings.Join(paths, ",") != want {
		t.Fatalf("paths %v, want %s", paths, want)
	}
	if composed.Types["ledger:github.com/acme/ledger/ledger.Status"].Name != "Ledger_Status" {
		t.Fatal("colliding ledger Status was not renamed")
	}
	if composed.Types["billing:github.com/acme/billing/billing.Status"].Name != "Billing_Status" {
		t.Fatal("colliding billing Status was not renamed")
	}
	if composed.Types["ledger:github.com/acme/ledger/ledger.Invoice"].Name != "Invoice" {
		t.Fatal("a name owned by one service must not be renamed")
	}
	for _, p := range composed.Procedures {
		if p.Path == "ledger.invoices.get" {
			if p.GoInput != "github.com/acme/ledger/api.GetInvoiceInput" || p.GoOutput != "github.com/acme/ledger/ledger.Invoice" {
				t.Fatalf("go names rewritten: %+v", p)
			}
			if p.Errors[0] != "ledger:github.com/acme/ledger/api.InvoiceLocked" {
				t.Fatalf("error id not prefixed: %v", p.Errors)
			}
		}
	}
	if composed.Hash == "" {
		t.Fatal("hash not recomputed")
	}
	if len(composed.Positions) != 0 {
		t.Fatal("positions must be dropped")
	}
}

func TestComposeRewritesEveryRef(t *testing.T) {
	composed, diags := Compose(map[string]*contract.Document{"ledger": load(t, "ledger.contract.json")})
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	invoice := composed.Types["ledger:github.com/acme/ledger/ledger.Invoice"]
	byName := map[string]*contract.Field{}
	for _, f := range invoice.Fields {
		byName[f.Name] = f
	}
	if got := byName["status"].Type.ID; got != "ledger:github.com/acme/ledger/ledger.Status" {
		t.Fatalf("field ref %q", got)
	}
	if got := byName["lines"].Type.Elem.Fields[0].Type.ID; got != "ledger:github.com/acme/ledger/ledger.Status" {
		t.Fatalf("inline struct ref %q", got)
	}
	if got := byName["byTag"].Type.Value.ID; got != "ledger:github.com/acme/ledger/ledger.Status" {
		t.Fatalf("map value ref %q", got)
	}
	if got := composed.Errors["ledger:github.com/acme/ledger/api.InvoiceLocked"].Fields[0].Type.ID; got != "ledger:github.com/acme/ledger/ledger.Status" {
		t.Fatalf("error field ref %q", got)
	}
	for _, p := range composed.Procedures {
		switch p.Path {
		case "ledger.invoices.list":
			if got := p.Output.Args[0].ID; got != "ledger:github.com/acme/ledger/ledger.Invoice" {
				t.Fatalf("generic arg ref %q", got)
			}
		case "ledger.invoices.watch":
			if p.Kind != "subscription" {
				t.Fatalf("subscription kind lost: %q", p.Kind)
			}
		case "ledger.invoices.attach":
			if p.Kind != "upload" {
				t.Fatalf("upload kind lost: %q", p.Kind)
			}
		}
	}
}

func TestComposeThreeServices(t *testing.T) {
	composed, diags := Compose(map[string]*contract.Document{
		"ledger":  load(t, "ledger.contract.json"),
		"billing": load(t, "billing.contract.json"),
		"search":  load(t, "search.contract.json"),
	})
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	golden(t, "three-services.golden.json", composed)
}

func TestComposeIsDeterministic(t *testing.T) {
	first, _ := Compose(map[string]*contract.Document{
		"ledger":  load(t, "ledger.contract.json"),
		"billing": load(t, "billing.contract.json"),
		"search":  load(t, "search.contract.json"),
	})
	for range 20 {
		again, _ := Compose(map[string]*contract.Document{
			"search":  load(t, "search.contract.json"),
			"billing": load(t, "billing.contract.json"),
			"ledger":  load(t, "ledger.contract.json"),
		})
		if first.Hash != again.Hash {
			t.Fatalf("hash %s then %s", first.Hash, again.Hash)
		}
	}
}

func TestComposeDiagnostics(t *testing.T) {
	if _, diags := Compose(nil); len(diags) != 1 || !strings.Contains(diags[0].Message, "no services") {
		t.Fatalf("empty map: %v", diags)
	}

	_, diags := Compose(map[string]*contract.Document{"led.ger": load(t, "ledger.contract.json")})
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "not a valid procedure segment") || diags[0].Service != "led.ger" {
		t.Fatalf("bad name: %v", diags)
	}
	if !strings.Contains(diags[0].String(), "never a dot") {
		t.Fatalf("diagnostic lacks a fix: %s", diags[0])
	}

	old := load(t, "billing.contract.json")
	old.Bowline = "2.0"
	_, diags = Compose(map[string]*contract.Document{
		"ledger":  load(t, "ledger.contract.json"),
		"billing": old,
	})
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "does not match") {
		t.Fatalf("version mismatch: %v", diags)
	}

	_, diags = Compose(map[string]*contract.Document{"ledger": nil})
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "no contract document") {
		t.Fatalf("nil document: %v", diags)
	}
}

func TestComposeDoesNotMutateInputs(t *testing.T) {
	ledger := load(t, "ledger.contract.json")
	before, err := ledger.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, diags := Compose(map[string]*contract.Document{"ledger": ledger, "billing": load(t, "billing.contract.json")}); len(diags) > 0 {
		t.Fatal(diags)
	}
	after, err := ledger.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Compose mutated its input document")
	}
}
