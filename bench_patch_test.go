package bowline

import "testing"

var benchDoc = []byte(`{"id":12345,"name":"ada","tags":["a","b","c"],"nested":{"x":1,"y":2},"note":"lorem ipsum dolor sit amet"}`)

var benchWideDoc = []byte(`{"id":9007199254740993,"name":"ada","tags":["a","b","c"],"nested":{"x":1,"y":2}}`)

func BenchmarkApplyMergePatch(b *testing.B) {
	patch := []byte(`{"name":"grace","nested":{"x":9}}`)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := applyMergePatch(benchDoc, patch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyMergePatchWideIntegers(b *testing.B) {
	patch := []byte(`{"name":"grace"}`)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := applyMergePatch(benchWideDoc, patch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyJSONPatch(b *testing.B) {
	patch := []byte(`[{"op":"replace","path":"/name","value":"grace"},{"op":"add","path":"/tags/-","value":"d"}]`)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := applyJSONPatch(benchDoc, patch); err != nil {
			b.Fatal(err)
		}
	}
}
