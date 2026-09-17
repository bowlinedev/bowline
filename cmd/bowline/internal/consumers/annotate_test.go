package consumers

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func TestAnnotateGolden(t *testing.T) {
	old := ledger(t)
	current := ledger(t)
	decl := typeByName(current, "Invoice")
	var kept []*contract.Field
	for _, f := range decl.Fields {
		if f.Name != "total" {
			kept = append(kept, f)
		}
	}
	decl.Fields = kept
	typeByName(current, "Attachment").Fields = typeByName(current, "Attachment").Fields[1:]
	find(current, "invoices.void").Errors = nil
	find(current, "customers.get").Method = "POST"
	changes := contract.Diff(old, current)
	annotated := Annotate(old, changes, []Consumer{consumer(t)})
	got := FormatMarkdown(annotated) + "\n" + FormatText(annotated)
	golden := filepath.Join("testdata", "annotated.golden.md")
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
}

func TestParsePath(t *testing.T) {
	cases := map[string][]string{
		"procedure invoices.list output field items elem field total": {"invoices.list", "output", "items,*,total"},
		"procedure invoices.get input field id":                       {"invoices.get", "input", "id"},
		"procedure invoices.void error InvoiceLocked":                 {"invoices.void", "error", "InvoiceLocked"},
		"procedure invoices.void":                                     {"invoices.void", "", ""},
		"procedure a.b output field m value field x":                  {"a.b", "output", "m,*,x"},
	}
	for in, want := range cases {
		procedure, side, path := parsePath(in)
		joined := ""
		var joinedSb53 strings.Builder
		for i, p := range path {
			if i > 0 {
				joinedSb53.WriteString(",")
			}
			joinedSb53.WriteString(p)
		}
		joined += joinedSb53.String()
		if procedure != want[0] || side != want[1] || joined != want[2] {
			t.Fatalf("%q: got %s %s %q", in, procedure, side, joined)
		}
	}
}
