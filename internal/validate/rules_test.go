package validate

import (
	"reflect"
	"testing"
)

func TestParseTag(t *testing.T) {
	rules, err := ParseTag("required,min=2,oneof=a b c")
	if err != nil {
		t.Fatal(err)
	}
	want := []Rule{{"required", ""}, {"min", "2"}, {"oneof", "a b c"}}
	if !reflect.DeepEqual(rules, want) {
		t.Fatalf("got %v", rules)
	}
	if rules, err := ParseTag(""); err != nil || rules != nil {
		t.Fatalf("empty tag: %v %v", rules, err)
	}
}

func TestParseTagRejectsUnsupported(t *testing.T) {
	for _, tag := range []string{"dive", "required,gte=1", "min", "email=x", "len=abc"} {
		if _, err := ParseTag(tag); err == nil {
			t.Errorf("%q: expected error", tag)
		}
	}
}

func TestApplies(t *testing.T) {
	cases := []struct {
		rule string
		c    Class
		ok   bool
	}{
		{"required", Struct, true}, {"min", String, true}, {"min", Integer, true}, {"min", Collection, true},
		{"min", Bool, false}, {"email", String, true}, {"email", Integer, false}, {"oneof", Integer, true},
		{"oneof", Float, false}, {"uuid", Collection, false},
	}
	for _, tc := range cases {
		if got := Applies(tc.rule, tc.c); got != tc.ok {
			t.Errorf("%s on %v: got %v", tc.rule, tc.c, got)
		}
	}
}

func TestClassOf(t *testing.T) {
	type named string
	cases := map[Class]reflect.Type{
		String: reflect.TypeFor[named](), Integer: reflect.TypeFor[*int32](), Float: reflect.TypeFor[float64](),
		Collection: reflect.TypeFor[map[string]int](), Bool: reflect.TypeFor[bool](), Struct: reflect.TypeFor[struct{ A int }](),
	}
	for want, typ := range cases {
		if got := ClassOf(typ); got != want {
			t.Errorf("%v: got %v want %v", typ, got, want)
		}
	}
}
