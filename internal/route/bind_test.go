package route

import (
	"reflect"
	"testing"
)

type bindInput struct {
	ID    int64  `json:"id"`
	Slug  string `json:"slug"`
	Count uint32 `json:"count"`
	Limit int32  `json:"limit"`
}

func params(pairs ...string) []Param {
	out := make([]Param, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, Param{Name: pairs[i], Value: pairs[i+1]})
	}
	return out
}

func TestBindSetsEveryKind(t *testing.T) {
	var in bindInput
	if err := Bind(&in, params("id", "42", "slug", "ada", "count", "7")); err != nil {
		t.Fatal(err)
	}
	if in.ID != 42 || in.Slug != "ada" || in.Count != 7 {
		t.Fatalf("got %+v", in)
	}
}

func TestBindLeavesOtherFieldsAlone(t *testing.T) {
	in := bindInput{Limit: 20, Slug: "keep"}
	if err := Bind(&in, params("id", "1")); err != nil {
		t.Fatal(err)
	}
	if in.Limit != 20 || in.Slug != "keep" {
		t.Fatalf("binding clobbered a field it was not given: %+v", in)
	}
}

func TestBindRejectsBadValues(t *testing.T) {
	for _, bad := range [][]Param{
		params("id", "abc"),
		params("id", ""),
		params("count", "-1"),
		params("limit", "99999999999999999999"),
	} {
		var in bindInput
		if err := Bind(&in, bad); err == nil {
			t.Fatalf("%v was accepted", bad)
		}
	}
}

func TestBindRejectsUnknownParam(t *testing.T) {
	var in bindInput
	if err := Bind(&in, params("nope", "1")); err == nil {
		t.Fatal("an unmatched parameter must be an error")
	}
}

func TestBindIgnoresUnexportedAndSkipped(t *testing.T) {
	type skipped struct {
		Secret string `json:"-"`
	}
	var s skipped
	if err := Bind(&s, params("-", "x")); err == nil {
		t.Fatal("a json:\"-\" field must not be bindable")
	}
	var in bindInput
	if err := Bind(&in, params("Slug", "x")); err == nil {
		t.Fatal("the Go field name must not bind when a json tag renames it")
	}
}

func TestBindableKinds(t *testing.T) {
	ok := []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[int64](), reflect.TypeFor[int32](), reflect.TypeFor[uint64](), reflect.TypeFor[int]()}
	for _, v := range ok {
		if !Bindable(v) {
			t.Fatalf("%T should be bindable", v)
		}
	}
	bad := []reflect.Type{reflect.TypeFor[float64](), reflect.TypeFor[bool](), reflect.TypeFor[[]string](), reflect.TypeFor[map[string]string](), reflect.TypeFor[struct{}]()}
	for _, v := range bad {
		if Bindable(v) {
			t.Fatalf("%T should not be bindable", v)
		}
	}
}

func TestFieldByWireNameUsesTheJSONName(t *testing.T) {
	if _, ok := FieldByWireName(reflect.TypeFor[bindInput](), "id"); !ok {
		t.Fatal("id not found")
	}
	if _, ok := FieldByWireName(reflect.TypeFor[bindInput](), "ID"); ok {
		t.Fatal("the Go name must not match when a json tag renames the field")
	}
}
