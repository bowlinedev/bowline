package route

import "testing"

func benchTable(b *testing.B) *Table {
	b.Helper()
	t, err := New([]Entry{
		{Pattern: "invoices", Method: "GET", Key: "invoices.list"},
		{Pattern: "invoices", Method: "POST", Key: "invoices.create"},
		{Pattern: "invoices/{id}", Method: "GET", Key: "invoices.get"},
		{Pattern: "invoices/{id}", Method: "DELETE", Key: "invoices.void"},
		{Pattern: "invoices/{id}/lines/{line}", Method: "GET", Key: "invoices.line"},
		{Pattern: "customers", Method: "GET", Key: "customers.list"},
		{Pattern: "customers/{id}", Method: "GET", Key: "customers.get"},
	})
	if err != nil {
		b.Fatal(err)
	}
	return t
}

func BenchmarkMatchWithParam(b *testing.B) {
	table := benchTable(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := table.Match("/api/invoices/4821", "GET"); !ok {
			b.Fatal("no match")
		}
	}
}

func BenchmarkMatchLiteral(b *testing.B) {
	table := benchTable(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := table.Match("/api/invoices", "GET"); !ok {
			b.Fatal("no match")
		}
	}
}

func BenchmarkMatchTwoParams(b *testing.B) {
	table := benchTable(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := table.Match("/api/invoices/4821/lines/7", "GET"); !ok {
			b.Fatal("no match")
		}
	}
}
