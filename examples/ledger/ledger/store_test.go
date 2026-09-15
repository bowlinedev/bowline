package ledger

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
}

func TestStoreSeedsAndPaginates(t *testing.T) {
	s := NewStore(fixedNow)
	page, next := s.ListInvoices("", 1, nil)
	if len(page) != 1 || next == "" {
		t.Fatalf("page %v next %q", page, next)
	}
	rest, next := s.ListInvoices(next, 10, nil)
	if len(rest) != 1 || next != "" {
		t.Fatalf("rest %v next %q", rest, next)
	}
	if page[0].Total != (Money{150000, "USD"}) {
		t.Fatalf("total %+v", page[0].Total)
	}
	paid := StatusPaid
	only, _ := s.ListInvoices("", 10, &paid)
	if len(only) != 1 || only[0].Status != StatusPaid {
		t.Fatalf("filter %v", only)
	}
}
