package contract

import (
	"strings"
	"testing"
)

func TestHashIgnoresPositionsAndExistingHash(t *testing.T) {
	a := sampleDocument()
	b := sampleDocument()
	b.Positions = map[string]Position{"users.get": {File: "moved.go", Line: 99}}
	b.Hash = "sha256:stale"
	ha, err := a.ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	hb, err := b.ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("hash changed with positions: %s vs %s", ha, hb)
	}
	if !strings.HasPrefix(ha, "sha256:") || len(ha) != len("sha256:")+64 {
		t.Fatalf("unexpected hash format %q", ha)
	}
}

func TestHashChangesWithContent(t *testing.T) {
	a := sampleDocument()
	b := sampleDocument()
	b.Procedures[1].Doc = "changed"
	ha, _ := a.ComputeHash()
	hb, _ := b.ComputeHash()
	if ha == hb {
		t.Fatal("hash must change when content changes")
	}
}

func TestSetHashRoundTrips(t *testing.T) {
	doc := sampleDocument()
	if err := doc.SetHash(); err != nil {
		t.Fatal(err)
	}
	data, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := parsed.ComputeHash()
	if parsed.Hash != want {
		t.Fatalf("stored hash %s does not match recomputed %s", parsed.Hash, want)
	}
}
