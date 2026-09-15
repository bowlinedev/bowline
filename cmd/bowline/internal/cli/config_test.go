package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./api.Routes","targets":{"ts":{"out":"web/src/bowline.ts"}}}`), 0o644)
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Contract != "bowline.contract.json" || cfg.Entry != "./api.Routes" || cfg.Targets["ts"].Out != "web/src/bowline.ts" {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("missing file must error")
	}
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"contract":"x.json"}`), 0o644)
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("missing entry must error")
	}
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./api.Routes","unknown":1}`), 0o644)
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("unknown keys must error")
	}
}
