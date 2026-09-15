package main

import (
	"bytes"
	"testing"

	"github.com/bowlinedev/bowline"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"version"}, &out, &out)
	if code != 0 {
		t.Fatalf("exit code %d, output %q", code, out.String())
	}
	if got, want := out.String(), "bowline "+bowline.Version+"\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"nope"}, &out, &out)
	if code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
}

func TestNoArgsPrintsUsage(t *testing.T) {
	var out bytes.Buffer
	code := run(nil, &out, &out)
	if code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("usage: bowline <command>")) {
		t.Fatalf("usage missing from %q", out.String())
	}
}
