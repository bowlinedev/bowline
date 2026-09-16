package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bowline.gateway.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeConfig(t, `{
  "listen": ":8090",
  "services": {
    "ledger": {"url": "http://ledger:8080/api", "contract": "contracts/ledger.contract.json", "version": "sha256:3f9"}
  }
}`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != "/api" {
		t.Fatalf("prefix %q", cfg.Prefix)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("timeout %s", cfg.Timeout)
	}
	if strings.Join(cfg.ForwardHeaders, ",") != "Authorization,Cookie,X-Request-Id,Accept-Language" {
		t.Fatalf("forward headers %v", cfg.ForwardHeaders)
	}
	if cfg.Services["ledger"].Retries != 2 {
		t.Fatalf("retries %d", cfg.Services["ledger"].Retries)
	}
}

func TestLoadConfigReadsEveryField(t *testing.T) {
	path := writeConfig(t, `{
  "listen": ":9000",
  "prefix": "/edge",
  "timeout": "5s",
  "forwardHeaders": ["Authorization"],
  "services": {
    "billing": {"url": "http://billing:8080/api", "registry": "https://registry.internal", "version": "main", "retries": 0}
  }
}`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":9000" || cfg.Prefix != "/edge" || cfg.Timeout != 5*time.Second {
		t.Fatalf("config %+v", cfg)
	}
	if len(cfg.ForwardHeaders) != 1 || cfg.ForwardHeaders[0] != "Authorization" {
		t.Fatalf("forward headers %v", cfg.ForwardHeaders)
	}
	up := cfg.Services["billing"]
	if up.Registry != "https://registry.internal" || up.Version != "main" || up.Retries != 0 {
		t.Fatalf("upstream %+v", up)
	}
	if names := cfg.ServiceNames(); len(names) != 1 || names[0] != "billing" {
		t.Fatalf("names %v", names)
	}
}

func TestLoadConfigRejections(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"unknown key":      {`{"listens": ":1", "services": {}}`, "unknown field"},
		"no services":      {`{"listen": ":1"}`, `"services" is required`},
		"no url":           {`{"services": {"a": {"contract": "c.json", "version": "sha256:1"}}}`, `needs a "url"`},
		"no source":        {`{"services": {"a": {"url": "http://a", "version": "sha256:1"}}}`, `needs either a "contract" file or a "registry"`},
		"two sources":      {`{"services": {"a": {"url": "http://a", "contract": "c.json", "registry": "http://r", "version": "sha256:1"}}}`, "pick one"},
		"no version":       {`{"services": {"a": {"url": "http://a", "contract": "c.json"}}}`, `needs a "version"`},
		"bad timeout":      {`{"timeout": "soon", "services": {"a": {"url": "http://a", "contract": "c.json", "version": "sha256:1"}}}`, "is not a duration"},
		"zero timeout":     {`{"timeout": "0s", "services": {"a": {"url": "http://a", "contract": "c.json", "version": "sha256:1"}}}`, "must be positive"},
		"negative retries": {`{"services": {"a": {"url": "http://a", "contract": "c.json", "version": "sha256:1", "retries": -1}}}`, "negative"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, c.body))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "nope.json")); err == nil || !strings.Contains(err.Error(), "reading") {
		t.Fatalf("got %v", err)
	}
}

func TestContractPathsResolveAgainstTheConfigFile(t *testing.T) {
	cfg, err := ParseConfig([]byte(`{"services":{"ledger":{"url":"http://x/api","contract":"contracts/ledger.contract.json","version":"sha256:a"}}}`), "deploy/edge/bowline.gateway.json")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("deploy", "edge", "contracts", "ledger.contract.json")
	if got := cfg.Services["ledger"].Contract; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	absolute := filepath.Join(string(filepath.Separator), "etc", "bowline", "ledger.json")
	cfg, err = ParseConfig([]byte(`{"services":{"ledger":{"url":"http://x/api","contract":"`+filepath.ToSlash(absolute)+`","version":"sha256:a"}}}`), "deploy/edge/bowline.gateway.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Services["ledger"].Contract; got != absolute {
		t.Fatalf("absolute path rewritten to %q", got)
	}
}

func TestRootedContractPathsAreTheSameOnEveryPlatform(t *testing.T) {
	for _, contract := range []string{"/etc/bowline/ledger.json", "/srv/contracts/a.json"} {
		cfg, err := ParseConfig([]byte(`{"services":{"ledger":{"url":"http://x/api","contract":"`+contract+`","version":"sha256:a"}}}`), "deploy/edge/bowline.gateway.json")
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.FromSlash(contract)
		if got := cfg.Services["ledger"].Contract; got != want {
			t.Errorf("%s resolved to %q, want %q", contract, got, want)
		}
	}
}
