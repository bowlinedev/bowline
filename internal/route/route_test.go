package route

import (
	"net/http"
	"testing"
)

func mustTable(t *testing.T, entries ...Entry) *Table {
	t.Helper()
	tbl, err := New(entries)
	if err != nil {
		t.Fatalf("building the table: %v", err)
	}
	return tbl
}

func TestCanonicalPathMatchesUnderAnyPrefix(t *testing.T) {
	tbl := mustTable(t, Entry{Pattern: "invoices.list", Method: http.MethodGet, Key: "invoices.list"})
	for _, path := range []string{"/invoices.list", "/api/invoices.list", "/a/b/c/invoices.list", "/api/invoices.list/"} {
		m, ok := tbl.Match(path, http.MethodGet)
		if !ok {
			t.Fatalf("%s did not match", path)
		}
		if m.Key != "invoices.list" {
			t.Fatalf("%s resolved to %q", path, m.Key)
		}
	}
}

func TestWildcardCapturesTheSegment(t *testing.T) {
	tbl := mustTable(t, Entry{Pattern: "invoices/{id}", Method: http.MethodGet, Key: "invoices.get"})
	m, ok := tbl.Match("/api/invoices/3", http.MethodGet)
	if !ok {
		t.Fatal("no match")
	}
	if m.Key != "invoices.get" {
		t.Fatalf("key %q", m.Key)
	}
	if m.Params["id"] != "3" {
		t.Fatalf("params %v", m.Params)
	}
}

func TestLiteralBeatsWildcardAtEqualLength(t *testing.T) {
	tbl := mustTable(t,
		Entry{Pattern: "invoices/{id}", Method: http.MethodGet, Key: "get"},
		Entry{Pattern: "invoices/summary", Method: http.MethodGet, Key: "summary"},
	)
	m, ok := tbl.Match("/api/invoices/summary", http.MethodGet)
	if !ok || m.Key != "summary" {
		t.Fatalf("got %+v ok=%v, want the literal route", m, ok)
	}
	m, ok = tbl.Match("/api/invoices/7", http.MethodGet)
	if !ok || m.Key != "get" || m.Params["id"] != "7" {
		t.Fatalf("got %+v ok=%v, want the wildcard route", m, ok)
	}
}

func TestLongerPatternWins(t *testing.T) {
	tbl := mustTable(t,
		Entry{Pattern: "invoices", Method: http.MethodGet, Key: "list"},
		Entry{Pattern: "invoices/{id}", Method: http.MethodGet, Key: "get"},
	)
	if m, ok := tbl.Match("/api/invoices", http.MethodGet); !ok || m.Key != "list" {
		t.Fatalf("list: %+v ok=%v", m, ok)
	}
	if m, ok := tbl.Match("/api/invoices/3", http.MethodGet); !ok || m.Key != "get" {
		t.Fatalf("get: %+v ok=%v", m, ok)
	}
}

func TestSamePatternDifferentMethods(t *testing.T) {
	tbl := mustTable(t,
		Entry{Pattern: "invoices", Method: http.MethodGet, Key: "list"},
		Entry{Pattern: "invoices", Method: http.MethodPost, Key: "create"},
	)
	if m, _ := tbl.Match("/api/invoices", http.MethodGet); m.Key != "list" {
		t.Fatalf("GET resolved to %q", m.Key)
	}
	if m, _ := tbl.Match("/api/invoices", http.MethodPost); m.Key != "create" {
		t.Fatalf("POST resolved to %q", m.Key)
	}
}

func TestMethodMismatchReportsAllowedMethods(t *testing.T) {
	tbl := mustTable(t, Entry{Pattern: "invoices", Method: http.MethodPost, Key: "create"})
	if _, ok := tbl.Match("/api/invoices", http.MethodGet); ok {
		t.Fatal("GET matched a POST-only route")
	}
	allowed := tbl.Allowed("/api/invoices")
	if len(allowed) != 1 || allowed[0] != http.MethodPost {
		t.Fatalf("allowed %v", allowed)
	}
}

func TestUnknownPathDoesNotMatch(t *testing.T) {
	tbl := mustTable(t, Entry{Pattern: "invoices/{id}", Method: http.MethodGet, Key: "get"})
	for _, path := range []string{"/api/invoices", "/api/invoices/3/extra", "/api/other/3"} {
		if _, ok := tbl.Match(path, http.MethodGet); ok {
			t.Fatalf("%s matched but should not have", path)
		}
	}
}

func TestLongerPatternWinsOverAShorterSuffix(t *testing.T) {
	tbl := mustTable(t,
		Entry{Pattern: "b", Method: http.MethodGet, Key: "short"},
		Entry{Pattern: "a/b", Method: http.MethodGet, Key: "long"},
	)
	if m, ok := tbl.Match("/a/b", http.MethodGet); !ok || m.Key != "long" {
		t.Fatalf("/a/b resolved to %+v, want the longer pattern", m)
	}
	if m, ok := tbl.Match("/x/b", http.MethodGet); !ok || m.Key != "short" {
		t.Fatalf("/x/b resolved to %+v, want the shorter pattern", m)
	}
}

func TestRestPairOfListAndGetIsAllowed(t *testing.T) {
	tbl := mustTable(t,
		Entry{Pattern: "invoices", Method: http.MethodGet, Key: "list"},
		Entry{Pattern: "invoices/{id}", Method: http.MethodGet, Key: "get"},
	)
	if m, ok := tbl.Match("/api/invoices", http.MethodGet); !ok || m.Key != "list" {
		t.Fatalf("list: %+v ok=%v", m, ok)
	}
	if m, ok := tbl.Match("/api/invoices/invoices", http.MethodGet); !ok || m.Key != "get" || m.Params["id"] != "invoices" {
		t.Fatalf("the longer pattern must win even when the segment repeats the literal: %+v ok=%v", m, ok)
	}
}

func TestSameShapeOnTheSameMethodIsRejected(t *testing.T) {
	_, err := New([]Entry{
		{Pattern: "invoices/{id}", Method: http.MethodGet, Key: "one"},
		{Pattern: "invoices/{slug}", Method: http.MethodGet, Key: "two"},
	})
	if err == nil {
		t.Fatal("two patterns with the same shape on one method must be rejected")
	}
}

func TestSameShapeOnDifferentMethodsIsAllowed(t *testing.T) {
	mustTable(t,
		Entry{Pattern: "invoices/{id}", Method: http.MethodGet, Key: "get"},
		Entry{Pattern: "invoices/{id}", Method: http.MethodPost, Key: "update"},
	)
}

func TestDuplicatePatternAndMethodIsRejected(t *testing.T) {
	_, err := New([]Entry{
		{Pattern: "invoices", Method: http.MethodGet, Key: "one"},
		{Pattern: "invoices", Method: http.MethodGet, Key: "two"},
	})
	if err == nil {
		t.Fatal("two routes cannot share a pattern and method")
	}
}

func TestMalformedPatternsAreRejected(t *testing.T) {
	for _, pattern := range []string{"", "/invoices", "invoices/", "invoices//get", "{}", "in{id}voices", "invoices/{id}/{id}"} {
		if _, err := New([]Entry{{Pattern: pattern, Method: http.MethodGet, Key: "k"}}); err == nil {
			t.Fatalf("pattern %q was accepted", pattern)
		}
	}
}

func TestParamNamesAreReported(t *testing.T) {
	names, err := Params("invoices/{invoiceId}/lines/{lineId}")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "invoiceId" || names[1] != "lineId" {
		t.Fatalf("names %v", names)
	}
}

func TestEncodedSegmentsAreDecodedAfterSplitting(t *testing.T) {
	tbl := mustTable(t, Entry{Pattern: "docs/{slug}", Method: http.MethodGet, Key: "docs.get"})
	m, ok := tbl.Match("/api/docs/a%2Fb", http.MethodGet)
	if !ok {
		t.Fatal("an encoded slash must stay inside one segment")
	}
	if m.Params["slug"] != "a/b" {
		t.Fatalf("slug %q, want %q", m.Params["slug"], "a/b")
	}
	if m, ok := tbl.Match("/api/docs/caf%C3%A9", http.MethodGet); !ok || m.Params["slug"] != "café" {
		t.Fatalf("utf-8 segment decoded to %q", m.Params["slug"])
	}
}

func TestMalformedEscapeDoesNotMatch(t *testing.T) {
	tbl := mustTable(t, Entry{Pattern: "docs/{slug}", Method: http.MethodGet, Key: "docs.get"})
	if _, ok := tbl.Match("/api/docs/%zz", http.MethodGet); ok {
		t.Fatal("a malformed escape must not match")
	}
}
