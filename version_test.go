package bowline

import "testing"

func TestVersionIsSemver(t *testing.T) {
	if Version != "0.5.0" {
		t.Fatalf("unexpected version %q", Version)
	}
}
