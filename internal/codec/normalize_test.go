package codec

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type inner struct {
	Tags []string          `json:"tags"`
	Attr map[string]string `json:"attr"`
}

type outer struct {
	ID      int64            `json:"id"`
	Big     int64            `json:"big,string"`
	Skip    []string         `json:"skip,omitempty"`
	Zero    []string         `json:"zero,omitzero"`
	Inner   inner            `json:"inner"`
	Ptr     *inner           `json:"ptr"`
	List    []inner          `json:"list"`
	ByName  map[string]inner `json:"byName"`
	When    time.Time        `json:"when"`
	Raw     json.RawMessage  `json:"raw"`
	Bytes   []byte           `json:"bytes"`
	Arr     [2][]int         `json:"arr"`
	private []string
	Embedded
}

type Embedded struct {
	Nested []int `json:"nested"`
}

type plain struct {
	Name string `json:"name"`
	N    int32  `json:"n"`
}

type chain struct {
	Next *chain `json:"next"`
	Name string `json:"name"`
}

type tree struct {
	Children []*tree `json:"children"`
}

func TestNilCollectionsBecomeEmpty(t *testing.T) {
	in := outer{ID: 1, Ptr: &inner{}, List: []inner{{}}, ByName: map[string]inner{"a": {}}}
	out, err := Compile(reflect.TypeOf(in)).Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(out)
	for _, want := range []string{
		`"tags":[]`, `"attr":{}`, `"ptr":{"tags":[],"attr":{}}`, `"list":[{"tags":[],"attr":{}}]`,
		`"byName":{"a":{"tags":[],"attr":{}}}`, `"bytes":""`, `"arr":[[],[]]`, `"nested":[]`,
	} {
		if !contains(data, want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
	if contains(data, `"skip"`) || contains(data, `"zero"`) {
		t.Errorf("omitempty and omitzero fields must be left alone: %s", data)
	}
	if !contains(data, `"raw":null`) {
		t.Errorf("json.RawMessage is a leaf and stays null: %s", data)
	}
}

func TestNormalizeIsPure(t *testing.T) {
	in := outer{List: []inner{{Tags: nil}}}
	if _, err := Compile(reflect.TypeOf(in)).Normalize(in); err != nil {
		t.Fatal(err)
	}
	if in.Inner.Tags != nil || in.List[0].Tags != nil {
		t.Fatal("input was mutated")
	}
}

func TestNormalizeSharesUnchangedSubtrees(t *testing.T) {
	shared := &inner{Tags: []string{"x"}, Attr: map[string]string{}}
	in := outer{Ptr: shared, Inner: inner{Tags: []string{}, Attr: map[string]string{}}, List: []inner{}, ByName: map[string]inner{}, Bytes: []byte{}, Arr: [2][]int{{}, {}}, Embedded: Embedded{Nested: []int{}}}
	out, err := Compile(reflect.TypeOf(in)).Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.(outer).Ptr != shared {
		t.Fatal("unchanged pointer subtree was copied")
	}
}

func TestInactivePlanReturnsSameValue(t *testing.T) {
	p := Compile(reflect.TypeOf(plain{}))
	if p.Active() {
		t.Fatal("plain has nothing to normalize")
	}
	in := plain{Name: "a", N: 1}
	out, err := p.Normalize(in)
	if err != nil || out != any(in) {
		t.Fatalf("got %v %v", out, err)
	}
}

func TestSafeRange(t *testing.T) {
	p := Compile(reflect.TypeOf(outer{}))
	_, err := p.Normalize(outer{ID: 1 << 53})
	var re *RangeError
	if !errors.As(err, &re) || re.Path != "id" {
		t.Fatalf("got %v", err)
	}
	if _, err := p.Normalize(outer{ID: 1<<53 - 1, Big: 1 << 62}); err != nil {
		t.Fatalf("string-encoded int64 must not be range checked: %v", err)
	}
	type nested struct {
		Items []struct {
			U uint64 `json:"u"`
		} `json:"items"`
	}
	_, err = Compile(reflect.TypeOf(nested{})).Normalize(nested{Items: []struct {
		U uint64 `json:"u"`
	}{{U: 1 << 60}}})
	if !errors.As(err, &re) || re.Path != "items[0].u" {
		t.Fatalf("got %v", err)
	}
}

func TestRecursiveTypes(t *testing.T) {
	p := Compile(reflect.TypeOf(chain{}))
	if p.Active() {
		t.Fatal("chain has no collections or 64-bit ints")
	}
	tp := Compile(reflect.TypeOf(tree{}))
	out, err := tp.Normalize(tree{Children: []*tree{{}}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(out)
	if string(data) != `{"children":[{"children":[]}]}` {
		t.Fatalf("got %s", data)
	}
}

func TestMarshalersAreLeaves(t *testing.T) {
	type withTime struct {
		When time.Time `json:"when"`
	}
	if Compile(reflect.TypeOf(withTime{})).Active() {
		t.Fatal("time.Time implements json.Marshaler and must be a leaf")
	}
}

func TestEquivalenceWithStdlibOnCleanValues(t *testing.T) {
	in := outer{
		ID: 42, Big: 7, Inner: inner{Tags: []string{"a"}, Attr: map[string]string{"k": "v"}},
		Ptr: &inner{Tags: []string{}, Attr: map[string]string{}}, List: []inner{}, ByName: map[string]inner{},
		When: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), Raw: json.RawMessage(`{"x":1}`), Bytes: []byte("hi"),
		Arr: [2][]int{{1}, {2}}, Embedded: Embedded{Nested: []int{3}},
	}
	out, err := Compile(reflect.TypeOf(in)).Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(in)
	b, _ := json.Marshal(out)
	if string(a) != string(b) {
		t.Fatalf("normalized output differs from stdlib on a clean value:\n%s\n%s", a, b)
	}
}

func contains(data []byte, s string) bool {
	return strings.Contains(string(data), s)
}
