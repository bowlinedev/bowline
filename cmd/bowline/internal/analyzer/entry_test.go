package analyzer

import (
	"strings"
	"testing"
)

func TestEvaluateRoutingFixture(t *testing.T) {
	prog, err := Load(fixtureDir(t), testEnv(), "./rows/...")
	if err != nil {
		t.Fatal(err)
	}
	fn, _, diag := prog.resolveEntry("./rows/routing.Routes")
	if diag != nil {
		t.Fatal(diag)
	}
	ev := newEvaluator(prog)
	specs := ev.routerFunc(fn, "", fn.Pos())
	if len(ev.diags) != 0 {
		t.Fatalf("diagnostics: %v", ev.diags)
	}
	byPath := map[string]procedureSpec{}
	for _, s := range specs {
		byPath[s.Path] = s
	}
	if len(byPath) != 4 {
		t.Fatalf("got %d procedures: %v", len(byPath), byPath)
	}
	if s := byPath["get"]; s.Kind != "query" || s.Method != "GET" || s.Description != "Get fetches an item." {
		t.Errorf("get: %+v", s)
	}
	if s := byPath["search"]; s.Method != "POST" || !s.Sensitive {
		t.Errorf("search: %+v", s)
	}
	if s := byPath["admin.purge"]; s.Kind != "mutation" || s.Meta["auth"] != "admin" || s.Deprecated != "use sub.remove" {
		t.Errorf("admin.purge: %+v", s)
	}
	if s := byPath["admin.purge"]; s.Meta["maxBody"] != "1048576" {
		t.Errorf("admin.purge maxBody: %q", s.Meta["maxBody"])
	}
	if s := byPath["sub.remove"]; s.In.String() != "fidelity.test/rows/routing/sub.ID" {
		t.Errorf("sub.remove: %+v", s)
	}
	if s := byPath["sub.remove"]; prog.Doc(s.Pkg, s.FnPos) != "Remove deletes an item by ID." {
		t.Errorf("doc lookup through method value failed: %q", prog.Doc(s.Pkg, s.FnPos))
	}
}

func TestEntryResolutionErrors(t *testing.T) {
	prog, err := Load(fixtureDir(t), testEnv(), "./rows/...")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"./rows/basics.Nope", "./rows/nowhere.Routes", "rows/basics.Echo", "Routes"} {
		if _, _, diag := prog.resolveEntry(entry); diag == nil {
			t.Errorf("%q: expected a diagnostic", entry)
		}
	}
}

func TestRejectedRouterShapes(t *testing.T) {
	prog, err := Load(fixtureDir(t), testEnv(), "./rows/...")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"./rows/reject-dynamic-router.Routes": "router structure must be static",
		"./rows/reject-computed-name.Routes":  "must be a string constant",
		"./rows/reject-spread.Routes":         "spread arguments",
	}
	for entry, want := range cases {
		fn, _, diag := prog.resolveEntry(entry)
		if diag != nil {
			t.Fatal(diag)
		}
		ev := newEvaluator(prog)
		ev.routerFunc(fn, "", fn.Pos())
		if len(ev.diags) == 0 || !strings.Contains(ev.diags[0].Message, want) {
			t.Errorf("%s: got %v, want message containing %q", entry, ev.diags, want)
		}
	}
}
