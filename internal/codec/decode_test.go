package codec

import "testing"

type in struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func TestDecodeEmptyIsEmptyObject(t *testing.T) {
	for _, data := range []string{"", "  \n"} {
		var v in
		if err := Decode([]byte(data), &v, false); err != nil {
			t.Fatalf("%q: %v", data, err)
		}
		if v != (in{}) {
			t.Fatalf("%q: got %+v", data, v)
		}
	}
}

func TestDecodeUnknownFields(t *testing.T) {
	var v in
	if err := Decode([]byte(`{"id":1,"extra":true}`), &v, false); err != nil {
		t.Fatalf("lenient: %v", err)
	}
	if err := Decode([]byte(`{"id":1,"extra":true}`), &v, true); err == nil {
		t.Fatal("strict: expected error")
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	var v in
	if err := Decode([]byte(`{"id":1} {"id":2}`), &v, false); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeRejectsFractionForInt(t *testing.T) {
	var v in
	if err := Decode([]byte(`{"id":1.5}`), &v, false); err == nil {
		t.Fatal("expected error")
	}
}
