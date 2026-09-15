package ledger

import (
	"encoding/json"
	"testing"
)

func TestMoneyRoundTrip(t *testing.T) {
	for _, m := range []Money{{1234, "USD"}, {-5, "EUR"}, {0, "GBP"}} {
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		var back Money
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("%s: %v", data, err)
		}
		if back != m {
			t.Fatalf("%s: got %+v want %+v", data, back, m)
		}
	}
	if string(mustJSON(Money{1234, "USD"})) != `"USD 12.34"` {
		t.Fatal("unexpected format")
	}
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
