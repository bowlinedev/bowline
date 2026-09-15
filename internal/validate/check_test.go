package validate

import (
	"reflect"
	"strings"
	"testing"
)

type line struct {
	Description string `json:"description" validate:"required,max=10"`
	Quantity    int32  `json:"quantity" validate:"min=1"`
}

type form struct {
	Email  string          `json:"email" validate:"required,email"`
	Site   string          `json:"site" validate:"url"`
	ID     string          `json:"id" validate:"uuid"`
	Status string          `json:"status" validate:"oneof=draft sent"`
	Level  int             `json:"level" validate:"oneof=1 2 3"`
	Score  float64         `json:"score" validate:"min=0.5,max=1"`
	Note   *string         `json:"note" validate:"max=3"`
	Must   *string         `json:"must" validate:"required"`
	Lines  []line          `json:"lines" validate:"required,max=2"`
	ByKey  map[string]line `json:"byKey"`
	Nested struct {
		N int `json:"n" validate:"min=5"`
	} `json:"nested"`
	Untagged string `json:"untagged"`
}

func issueSet(issues []Issue) map[string]string {
	out := map[string]string{}
	for _, i := range issues {
		out[strings.Join(i.Path, ".")+":"+i.Rule] = i.Message
	}
	return out
}

func TestCheckReportsEveryIssue(t *testing.T) {
	c, err := Compile(reflect.TypeFor[form]())
	if err != nil {
		t.Fatal(err)
	}
	long := "toolong"
	v := form{
		Email: "nope", Site: "not a url", ID: "123", Status: "paid", Level: 9, Score: 2, Note: &long,
		Lines: []line{{Description: "", Quantity: 0}, {Description: "ok", Quantity: 1}, {Description: "x", Quantity: 1}},
		ByKey: map[string]line{"k": {Description: "this is far too long", Quantity: 1}},
	}
	got := issueSet(c.Check(v))
	for _, key := range []string{
		"email:email", "site:url", "id:uuid", "status:oneof", "level:oneof", "score:max", "note:max", "must:required",
		"lines:max", "lines.0.description:required", "lines.0.quantity:min", "byKey.k.description:max", "nested.n:min",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing issue %s in %v", key, got)
		}
	}
	if len(got) != 13 {
		t.Errorf("got %d issues, want 13: %v", len(got), got)
	}
}

func TestCheckPassesValidValue(t *testing.T) {
	c, _ := Compile(reflect.TypeFor[form]())
	must := "x"
	v := form{Email: "a@b.co", Site: "https://example.com/x", ID: "123e4567-e89b-12d3-a456-426614174000", Status: "sent", Level: 2, Score: 0.75, Must: &must, Lines: []line{{Description: "ok", Quantity: 1}}}
	v.Nested.N = 5
	if issues := c.Check(v); len(issues) != 0 {
		t.Fatalf("unexpected issues %v", issues)
	}
}

func TestCompileRejectsMisappliedRules(t *testing.T) {
	type bad struct {
		N int `validate:"email"`
	}
	if _, err := Compile(reflect.TypeFor[bad]()); err == nil || !strings.Contains(err.Error(), "email") {
		t.Fatalf("got %v", err)
	}
	type unsupported struct {
		S string `validate:"gte=1"`
	}
	if _, err := Compile(reflect.TypeFor[unsupported]()); err == nil {
		t.Fatal("expected error")
	}
}

func TestInactiveChecker(t *testing.T) {
	type none struct {
		A string `json:"a"`
	}
	c, err := Compile(reflect.TypeFor[none]())
	if err != nil || c.Active() {
		t.Fatalf("got active=%v err=%v", c.Active(), err)
	}
	if c.Check(none{}) != nil {
		t.Fatal("inactive checker must return nil")
	}
}

func TestRecursiveInput(t *testing.T) {
	type tree struct {
		Name     string  `json:"name" validate:"required"`
		Children []*tree `json:"children"`
	}
	c, err := Compile(reflect.TypeFor[tree]())
	if err != nil {
		t.Fatal(err)
	}
	issues := c.Check(tree{Name: "root", Children: []*tree{{Name: ""}, nil}})
	if len(issues) != 1 || strings.Join(issues[0].Path, ".") != "children.0.name" {
		t.Fatalf("got %v", issues)
	}
}
