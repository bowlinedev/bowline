package codec

import (
	"encoding/json"
	"reflect"
	"testing"
)

type fuzzTarget struct {
	ID    int64            `json:"id"`
	Name  string           `json:"name"`
	Tags  []string         `json:"tags"`
	Attrs map[string]int32 `json:"attrs"`
	Inner *fuzzTarget      `json:"inner"`
	List  []fuzzTarget     `json:"list"`
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{"id":1}`))
	f.Add([]byte(`{"id":1,"inner":{"tags":null}}`))
	f.Add([]byte(``))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var v fuzzTarget
		strict := len(data)%2 == 0
		if err := Decode(data, &v, strict); err != nil {
			return
		}
		out, err := Compile(reflect.TypeOf(v)).Normalize(v)
		if err != nil {
			return
		}
		if _, err := json.Marshal(out); err != nil {
			t.Fatalf("normalized value failed to marshal: %v", err)
		}
	})
}

func FuzzNormalizeEquivalence(f *testing.F) {
	f.Add(int64(1), "a", true)
	f.Fuzz(func(t *testing.T, id int64, name string, withCollections bool) {
		if id > 1<<53-1 || id < -(1<<53-1) {
			return
		}
		v := fuzzTarget{ID: id, Name: name}
		if withCollections {
			v.Tags = []string{name}
			v.Attrs = map[string]int32{name: 1}
			v.List = []fuzzTarget{{ID: id, Tags: []string{}, Attrs: map[string]int32{}, List: []fuzzTarget{}}}
			v.Inner = &fuzzTarget{Tags: []string{}, Attrs: map[string]int32{}, List: []fuzzTarget{}}
		} else {
			v.Tags, v.Attrs, v.List = []string{}, map[string]int32{}, []fuzzTarget{}
		}
		out, err := Compile(reflect.TypeOf(v)).Normalize(v)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := json.Marshal(v)
		b, _ := json.Marshal(out)
		if string(a) != string(b) {
			t.Fatalf("mismatch\n%s\n%s", a, b)
		}
	})
}
