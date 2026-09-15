package naming

import (
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func TestAssignRenamesCollisionsAndEscapesReservedWords(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"example.com/app/users.Event": {Kind: contract.Struct, Name: "Event"},
		"example.com/app/audit.Event": {Kind: contract.Struct, Name: "Event"},
		"example.com/app/audit.Log":   {Kind: contract.Struct, Name: "Log"},
		"example.com/app/audit.class": {Kind: contract.Struct, Name: "class"},
	}}
	names := Assign(doc, map[string]bool{"class": true})
	want := map[string]string{
		"example.com/app/users.Event": "Users_Event",
		"example.com/app/audit.Event": "Audit_Event",
		"example.com/app/audit.Log":   "Log",
		"example.com/app/audit.class": "class_",
	}
	for id, name := range want {
		if names[id] != name {
			t.Errorf("%s: got %q, want %q", id, names[id], name)
		}
	}
}

func TestCaseConverters(t *testing.T) {
	cases := []struct{ in, lower, upper, snake string }{
		{"createdAt", "createdAt", "CreatedAt", "created_at"},
		{"ID", "id", "Id", "id"},
		{"byKey", "byKey", "ByKey", "by_key"},
		{"x-y", "xY", "XY", "x_y"},
		{"HTTPStatus", "httpStatus", "HttpStatus", "http_status"},
		{"user2Name", "user2Name", "User2Name", "user2_name"},
		{"1st", "_1st", "_1st", "_1st"},
	}
	for _, c := range cases {
		if got := LowerCamel(c.in); got != c.lower {
			t.Errorf("LowerCamel(%q) = %q, want %q", c.in, got, c.lower)
		}
		if got := UpperCamel(c.in); got != c.upper {
			t.Errorf("UpperCamel(%q) = %q, want %q", c.in, got, c.upper)
		}
		if got := Snake(c.in); got != c.snake {
			t.Errorf("Snake(%q) = %q, want %q", c.in, got, c.snake)
		}
	}
	if Identifier("a-b.c") != "a_b_c" || Identifier("9x") != "_x" {
		t.Fatal("identifier sanitizing changed")
	}
}
