package contract

import "testing"

func TestParsePathAcceptsValidTemplates(t *testing.T) {
	cases := map[string][]PathSegment{
		"users":                     {{Text: "users"}},
		"invoices/{id}":             {{Text: "invoices"}, {Text: "id", Param: true}},
		"a/{b}/c/{d}":               {{Text: "a"}, {Text: "b", Param: true}, {Text: "c"}, {Text: "d", Param: true}},
		"invoices.list":             {{Text: "invoices.list"}},
		"v1/orders/{orderId}/lines": {{Text: "v1"}, {Text: "orders"}, {Text: "orderId", Param: true}, {Text: "lines"}},
	}
	for template, want := range cases {
		got, err := ParsePath(template)
		if err != nil {
			t.Fatalf("%q: %v", template, err)
		}
		if len(got) != len(want) {
			t.Fatalf("%q: %d segments, want %d", template, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q segment %d: %+v, want %+v", template, i, got[i], want[i])
			}
		}
	}
}

func TestParsePathRejectsBadTemplates(t *testing.T) {
	for _, template := range []string{
		"", "/users", "users/", "a//b", "{}", "in{id}voices", "{id}x",
		"invoices/{id}/{id}", "users/{a b}", "users/{a/b}", "a b", "users?x",
	} {
		if _, err := ParsePath(template); err == nil {
			t.Fatalf("%q was accepted", template)
		}
	}
}

func TestPathParamsReturnsNamesInOrder(t *testing.T) {
	names, err := PathParams("orders/{orderId}/lines/{lineId}")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "orderId" || names[1] != "lineId" {
		t.Fatalf("names %v", names)
	}
	if names, _ := PathParams("users"); len(names) != 0 {
		t.Fatalf("a template with no parameters returned %v", names)
	}
}
