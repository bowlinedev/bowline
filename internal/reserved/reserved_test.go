package reserved

import "testing"

func TestPathMatchesOnlyOnASegmentBoundary(t *testing.T) {
	cases := map[string]string{
		".bowline/contract":            ContractPath,
		"/.bowline/contract":           ContractPath,
		"/api/.bowline/contract":       ContractPath,
		"/api/v2/.bowline/contract":    ContractPath,
		"/api/.bowline/contract/":      ContractPath,
		".bowline/health":              HealthPath,
		"/api/.bowline/health":         HealthPath,
		"/api/evil.bowline/contract":   "",
		"/api/x.bowline/health":        "",
		"/apibowline/contract":         "",
		"/api/.bowline/contracts":      "",
		"/api/.bowline/contract/extra": "",
		"/api/bowline/contract":        "",
		"/api/.bowlinecontract":        "",
		"/api/invoices.get":            "",
		"":                             "",
		"/":                            "",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			if got := Path(path); got != want {
				t.Fatalf("Path(%q) = %q, want %q", path, got, want)
			}
		})
	}
}

func TestReservedNamesCarryNoSeparatorOfTheirOwn(t *testing.T) {
	for _, name := range []string{ContractPath, HealthPath} {
		if name == "" || name[0] == '/' {
			t.Fatalf("%q must be relative, so that suffix matching stays anchored to a segment", name)
		}
	}
}
