package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeMatchesCLIGoldens(t *testing.T) {
	list, err := Tools(ledger(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []Format{Anthropic, OpenAI, Schema} {
		got, err := Encode(list, f)
		if err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("..", "cmd", "bowline", "internal", "tools", "testdata", "ledger."+string(f)+".golden.json")
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s differs from %s", f, golden)
		}
	}
}

func TestEncodeUnknownFormat(t *testing.T) {
	_, err := Encode(nil, Format("gemini"))
	if err == nil || !strings.Contains(err.Error(), `"gemini"`) {
		t.Fatalf("got %v", err)
	}
}
