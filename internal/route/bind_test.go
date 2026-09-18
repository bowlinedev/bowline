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

func TestBindSetsEveryKind(t *testing.T) {
	var in bindInput
	if err := Bind(&in, map[string]string{"id": "42", "slug": "ada", "count": "7"}); err != nil {
		t.Fatal(err)
	}
	if in.ID != 42 || in.Slug != "ada" || in.Count != 7 {
		t.Fatalf("got %+v", in)
	}
}

func TestBindLeavesOtherFieldsAlone(t *testing.T) {
	in := bindInput{Limit: 20, Slug: "keep"}
	if err := Bind(&in, map[string]string{"id": "1"}); err != nil {
		t.Fatal(err)
	}
	if in.Limit != 20 || in.Slug != "keep" {
		t.Fatalf("binding clobbered a field it was not given: %+v", in)
	}
}

func TestBindRejectsBadValues(t *testing.T) {
	for _, params := range []map[string]string{
		{"id": "abc"},
		{"id": ""},
		{"count": "-1"},
		{"limit": "99999999999999999999"},
	} {
		var in bindInput
		if err := Bind(&in, params); err == nil {
			t.Fatalf("%v was accepted", params)
		}
	}
}

func TestBindRejectsUnknownParam(t *testing.T) {
	var in bindInput
	if err := Bind(&in, map[string]string{"nope": "1"}); err == nil {
		t.Fatal("an unmatched parameter must be an error")
	}
}

func TestBindIgnoresUnexportedAndSkipped(t *testing.T) {
	type skipped struct {
		Secret string `json:"-"`
	}
	var s skipped
	if err := Bind(&s, map[string]string{"-": "x"}); err == nil {
		t.Fatal("a json:\"-\" field must not be bindable")
	}
	var in bindInput
	if err := Bind(&in, map[string]string{"Slug": "x"}); err == nil {
		t.Fatal("the Go field name must not bind when a json tag renames it")
	}
}

func TestBindableKinds(t *testing.T) {
	ok := []any{"", int64(0), int32(0), uint64(0), 0}
	for _, v := range ok {
		if !Bindable(reflect.TypeOf(v)) {
			t.Fatalf("%T should be bindable", v)
		}
	}
	bad := []any{1.5, true, []string{}, map[string]string{}, struct{}{}}
	for _, v := range bad {
		if Bindable(reflect.TypeOf(v)) {
			t.Fatalf("%T should not be bindable", v)
		}
	}
}

func TestFieldByWireNameUsesTheJSONName(t *testing.T) {
	if _, ok := FieldByWireName(reflect.TypeOf(bindInput{}), "id"); !ok {
		t.Fatal("id not found")
	}
	if _, ok := FieldByWireName(reflect.TypeOf(bindInput{}), "ID"); ok {
		t.Fatal("the Go name must not match when a json tag renames the field")
	}
}
