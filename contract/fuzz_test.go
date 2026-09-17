package contract

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func FuzzContractParse(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"version":"1.2"}`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"version":"1.2","types":{},"procedures":{}}`))

	matches, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		f.Fatal(err)
	}
	more, err := filepath.Glob(filepath.Join("..", "cmd", "bowline", "internal", "analyzer", "testdata", "*", "expected.contract.json"))
	if err != nil {
		f.Fatal(err)
	}
	for _, name := range append(matches, more...) {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Parse(data)
		if err != nil {
			return
		}
		if doc == nil {
			t.Fatal("Parse returned no document and no error")
		}
		out, err := doc.Marshal()
		if err != nil {
			t.Fatalf("a parsed document failed to marshal: %v", err)
		}
		again, err := Parse(out)
		if err != nil {
			t.Fatalf("a marshalled document failed to parse: %v", err)
		}
		if !reflect.DeepEqual(doc, again) {
			t.Fatal("parse, marshal, parse did not round-trip to an equal document")
		}
		third, err := again.Marshal()
		if err != nil {
			t.Fatalf("the second marshal failed: %v", err)
		}
		if !bytes.Equal(out, third) {
			t.Fatal("marshalling is not stable across two rounds")
		}
	})
}

func FuzzContractHash(f *testing.F) {
	f.Add([]byte(`{"version":"1.2","types":{},"procedures":{}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Parse(data)
		if err != nil {
			return
		}
		first, err := doc.ComputeHash()
		if err != nil {
			return
		}
		second, err := doc.ComputeHash()
		if err != nil {
			t.Fatalf("the hash failed on the second call: %v", err)
		}
		if first != second {
			t.Fatalf("hash is not stable: %q then %q", first, second)
		}
	})
}
