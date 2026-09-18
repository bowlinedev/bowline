package bowline

import "testing"

func TestVersionIsSemver(t *testing.T) {
	if Version != "1.3.0" {
		t.Fatalf("unexpected version %q", Version)
	}
}
