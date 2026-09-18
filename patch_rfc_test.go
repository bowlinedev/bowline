package bowline

import (
	"encoding/json"
	"testing"
)

func patched(t *testing.T, doc, patch string) (string, error) {
	t.Helper()
	out, err := applyJSONPatch([]byte(doc), []byte(patch))
	if err != nil {
		return "", err
	}
	var normalised any
	json.Unmarshal(out, &normalised)
	canonical, _ := json.Marshal(normalised)
	return string(canonical), nil
}

func TestJSONPatchArrays(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		patch string
		want  string
	}{
		{"replace an element", `{"t":["a","b"]}`, `[{"op":"replace","path":"/t/1","value":"z"}]`, `{"t":["a","z"]}`},
		{"append with dash", `{"t":["a","b"]}`, `[{"op":"add","path":"/t/-","value":"z"}]`, `{"t":["a","b","z"]}`},
		{"insert shifts right", `{"t":["a","b"]}`, `[{"op":"add","path":"/t/0","value":"z"}]`, `{"t":["z","a","b"]}`},
		{"add at length appends", `{"t":["a"]}`, `[{"op":"add","path":"/t/1","value":"z"}]`, `{"t":["a","z"]}`},
		{"remove shifts left", `{"t":["a","b","c"]}`, `[{"op":"remove","path":"/t/1"}]`, `{"t":["a","c"]}`},
		{"nested array", `{"a":{"b":[1,2]}}`, `[{"op":"replace","path":"/a/b/0","value":9}]`, `{"a":{"b":[9,2]}}`},
		{"array of objects", `{"t":[{"n":1}]}`, `[{"op":"replace","path":"/t/0/n","value":5}]`, `{"t":[{"n":5}]}`},
		{"copy into array", `{"t":["a"],"s":"z"}`, `[{"op":"copy","from":"/s","path":"/t/-"}]`, `{"s":"z","t":["a","z"]}`},
		{"move within array", `{"t":["a","b"]}`, `[{"op":"move","from":"/t/0","path":"/t/1"}]`, `{"t":["b","a"]}`},
		{"test an element", `{"t":["a","b"]}`, `[{"op":"test","path":"/t/1","value":"b"},{"op":"remove","path":"/t/0"}]`, `{"t":["b"]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := patched(t, c.doc, c.patch)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestJSONPatchRejectsWhatTheRFCRequires(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		patch string
	}{
		{"add index past the end", `{"t":["a"]}`, `[{"op":"add","path":"/t/5","value":"z"}]`},
		{"replace a missing member", `{"a":1}`, `[{"op":"replace","path":"/nope","value":1}]`},
		{"replace a missing element", `{"t":["a"]}`, `[{"op":"replace","path":"/t/3","value":"z"}]`},
		{"remove a missing member", `{"a":1}`, `[{"op":"remove","path":"/nope"}]`},
		{"add under a missing parent", `{"a":1}`, `[{"op":"add","path":"/nope/deep","value":1}]`},
		{"index with a leading zero", `{"t":["a","b"]}`, `[{"op":"replace","path":"/t/01","value":"z"}]`},
		{"negative index", `{"t":["a"]}`, `[{"op":"replace","path":"/t/-1","value":"z"}]`},
		{"dash when not adding", `{"t":["a"]}`, `[{"op":"remove","path":"/t/-"}]`},
		{"move into its own child", `{"a":{"b":{}}}`, `[{"op":"move","from":"/a","path":"/a/b/c"}]`},
		{"move from a missing location", `{"a":1}`, `[{"op":"move","from":"/nope","path":"/b"}]`},
		{"copy from a missing location", `{"a":1}`, `[{"op":"copy","from":"/nope","path":"/b"}]`},
		{"pointer without a slash", `{"a":1}`, `[{"op":"replace","path":"a","value":1}]`},
		{"unknown operation", `{"a":1}`, `[{"op":"frobnicate","path":"/a","value":1}]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := patched(t, c.doc, c.patch); err == nil {
				t.Fatalf("accepted, producing %s", got)
			}
		})
	}
}

func TestJSONPatchAddReplacesAnExistingMember(t *testing.T) {
	got, err := patched(t, `{"a":1}`, `[{"op":"add","path":"/a","value":2}]`)
	if err != nil || got != `{"a":2}` {
		t.Fatalf("got %s err %v; add onto an existing object member replaces it", got, err)
	}
}

func TestJSONPatchIsAtomicOnFailure(t *testing.T) {
	doc := `{"a":1,"b":2}`
	if _, err := patched(t, doc, `[{"op":"remove","path":"/a"},{"op":"remove","path":"/nope"}]`); err == nil {
		t.Fatal("the second operation should have failed")
	}
}
