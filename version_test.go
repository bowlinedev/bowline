package bowline

import "testing"

func TestVersionIsSemver(t *testing.T) {
	if Version != "0.6.0" {
		t.Fatalf("unexpected version %q", Version)
	}
}
