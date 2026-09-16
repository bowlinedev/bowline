package bowline

import (
	"net/url"
	"testing"
)

func TestQueryInputMatchesParseQuery(t *testing.T) {
	for _, raw := range []string{
		"",
		"input=",
		"input=%7B%22id%22%3A7%7D",
		"a=1&input=x&b=2",
		"input=x&input=y",
		"INPUT=x",
		"inp%75t=x",
		"input",
		"&&input=x&&",
		"input=%ZZ",
		"a=%ZZ&input=x",
		"input=a+b",
		"input=x;y=z",
		"input=x&y=z;w=1",
		"=input",
		"input==x",
	} {
		want := ""
		if values, err := url.ParseQuery(raw); err == nil || len(values) > 0 {
			want = values.Get("input")
		}
		if got := queryInput(raw); got != want {
			t.Errorf("queryInput(%q) = %q, url.ParseQuery gives %q", raw, got, want)
		}
	}
}

func FuzzQueryInputMatchesParseQuery(f *testing.F) {
	for _, seed := range []string{"input=x", "a=1&input=%7B%7D", "input=x;y=z", "inp%75t=1", "input=a+b&input=c"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		values, _ := url.ParseQuery(raw)
		want := values.Get("input")
		if got := queryInput(raw); got != want {
			t.Fatalf("queryInput(%q) = %q, url.ParseQuery gives %q", raw, got, want)
		}
	})
}
