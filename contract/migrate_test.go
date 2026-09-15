package contract

import (
	"strings"
	"testing"
)

func TestParseRejectsZeroPointX(t *testing.T) {
	_, err := Parse([]byte(`{"bowline":"0.1","types":{},"procedures":[]}`))
	if err == nil || !strings.Contains(err.Error(), "bowline migrate-contract") {
		t.Fatalf("got %v", err)
	}
}

func TestMigrateZeroToOne(t *testing.T) {
	old := []byte(`{"bowline":"0.1","hash":"sha256:stale","types":{},"procedures":[{"path":"a","kind":"query","method":"GET","input":{"kind":"struct"},"output":{"kind":"struct"}}]}`)
	migrated, err := Migrate(old)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(migrated)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Bowline != Version || doc.Errors == nil || len(doc.Procedures) != 1 || doc.Procedures[0].Path != "a" {
		t.Fatalf("%+v", doc)
	}
	want, _ := doc.ComputeHash()
	if doc.Hash != want || doc.Hash == "sha256:stale" {
		t.Fatalf("hash %s want %s", doc.Hash, want)
	}
	if again, err := Migrate(migrated); err != nil || string(again) != string(migrated) {
		t.Fatalf("migrating a current document must be a no-op: %v", err)
	}
}

func TestMigrateRejectsUnknownMajor(t *testing.T) {
	if _, err := Migrate([]byte(`{"bowline":"7.0","types":{},"procedures":[]}`)); err == nil {
		t.Fatal("expected error")
	}
}
