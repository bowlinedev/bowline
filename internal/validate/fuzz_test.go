package validate

import (
	"reflect"
	"testing"
)

type fuzzInput struct {
	Name  string            `json:"name"`
	Count int32             `json:"count"`
	Score float64           `json:"score"`
	Tags  []string          `json:"tags"`
	Attrs map[string]string `json:"attrs"`
	Inner *fuzzInner        `json:"inner"`
}

type fuzzInner struct {
	Email string   `json:"email"`
	Items []string `json:"items"`
}

func FuzzValidateParseTag(f *testing.F) {
	seeds := []string{
		"",
		"required",
		"required,min=1,max=200",
		"oneof=draft sent paid void",
		"email",
		"url",
		"uuid",
		"len=36",
		"min=",
		"=1",
		",,,",
		"min=1,min=2",
		"required,unknown=3",
		"min=99999999999999999999",
		"min=-1",
		"oneof=",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, tag string) {
		rules, err := ParseTag(tag)
		if err != nil {
			if rules != nil {
				t.Fatalf("ParseTag(%q) returned rules alongside %v", tag, err)
			}
			return
		}
		for _, rule := range rules {
			if rule.Name == "" {
				t.Fatalf("ParseTag(%q) produced a rule with no name", tag)
			}
			if _, ok := vocabulary[rule.Name]; !ok {
				t.Fatalf("ParseTag(%q) produced %q, which is not in the vocabulary", tag, rule.Name)
			}
		}
	})
}

func FuzzValidateCheck(f *testing.F) {
	f.Add("required", "a", int32(1))
	f.Add("min=1,max=3", "", int32(0))
	f.Add("oneof=a b", "b", int32(-1))
	f.Add("email", "not an address", int32(1<<30))
	f.Fuzz(func(t *testing.T, tag, name string, count int32) {
		if _, err := ParseTag(tag); err != nil {
			return
		}
		checker, err := Compile(reflect.TypeOf(fuzzInput{}))
		if err != nil {
			t.Fatalf("the fixture type failed to compile: %v", err)
		}
		value := fuzzInput{
			Name:  name,
			Count: count,
			Tags:  []string{name},
			Attrs: map[string]string{name: name},
			Inner: &fuzzInner{Email: name, Items: []string{name}},
		}
		for _, issue := range checker.Check(value) {
			if len(issue.Path) == 0 {
				t.Fatalf("an issue for %+v carries no path", value)
			}
			if issue.Rule == "" {
				t.Fatalf("an issue at %q carries no rule", issue.Path)
			}
			if issue.Message == "" {
				t.Fatalf("an issue at %q carries no message", issue.Path)
			}
		}
	})
}
