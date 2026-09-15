package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bowlinedev/bowline/contract"
)

func TestCheckJSONOutput(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes"}`), 0o644)

	opts, out, _ := testOptions(dir)
	if code := Check(opts, []string{"--json"}); code != 1 {
		t.Fatalf("exit %d, want 1 before generating", code)
	}
	var missing CheckReport
	if err := json.Unmarshal(out.Bytes(), &missing); err != nil {
		t.Fatalf("%v in %q", err, out.String())
	}
	if missing.Mode != "drift" || missing.OK {
		t.Fatalf("report %+v", missing)
	}
	if len(missing.Files) != 1 || missing.Files[0].Path != "bowline.contract.json" || missing.Files[0].Status != "missing" {
		t.Fatalf("files %+v", missing.Files)
	}

	Gen(testOptionsOnly(dir), nil)
	opts, out, errOut := testOptions(dir)
	if code := Check(opts, []string{"--json"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var clean CheckReport
	if err := json.Unmarshal(out.Bytes(), &clean); err != nil {
		t.Fatalf("%v in %q", err, out.String())
	}
	if !clean.OK || clean.Files[0].Status != "ok" {
		t.Fatalf("report %+v", clean)
	}

	os.WriteFile(filepath.Join(dir, "bowline.contract.json"), []byte("{}"), 0o644)
	opts, out, _ = testOptions(dir)
	if code := Check(opts, []string{"--json"}); code != 1 {
		t.Fatalf("exit %d, want 1 after drift", code)
	}
	var stale CheckReport
	if err := json.Unmarshal(out.Bytes(), &stale); err != nil {
		t.Fatalf("%v in %q", err, out.String())
	}
	if stale.OK || stale.Files[0].Status != "outdated" {
		t.Fatalf("report %+v", stale)
	}
}

func TestCheckJSONWritesNothingHumanReadable(t *testing.T) {
	dir := fixtureCopy(t)
	os.WriteFile(filepath.Join(dir, "bowline.json"), []byte(`{"entry":"./rows/routing.Routes"}`), 0o644)
	Gen(testOptionsOnly(dir), nil)
	opts, out, errOut := testOptions(dir)
	Check(opts, []string{"--json"})
	if errOut.Len() != 0 {
		t.Fatalf("stderr %q", errOut.String())
	}
	var report CheckReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not a single JSON document: %v in %q", err, out.String())
	}
}

func TestDiffJSONOutput(t *testing.T) {
	dir := writeContracts(t)
	opts, out, _ := testOptions(dir)
	if code := DiffCommand(opts, []string{"--json", "old.json", "new.json"}); code != 1 {
		t.Fatalf("exit %d, want 1 on a breaking change", code)
	}
	var changes []contract.Change
	if err := json.Unmarshal(out.Bytes(), &changes); err != nil {
		t.Fatalf("%v in %q", err, out.String())
	}
	found := false
	for _, c := range changes {
		if c.Category == contract.Breaking && c.Path == "procedure get output field total" && c.Message == "field removed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("changes %+v", changes)
	}

	opts, formatted, _ := testOptions(dir)
	DiffCommand(opts, []string{"--format", "json", "old.json", "new.json"})
	if formatted.String() != out.String() {
		t.Fatalf("--json and --format json differ:\n%s\n%s", out.String(), formatted.String())
	}
}

func TestDiffJSONIsEmptyArrayWhenNothingChanged(t *testing.T) {
	dir := writeContracts(t)
	os.WriteFile(filepath.Join(dir, "same.json"), []byte(oldContract), 0o644)
	opts, out, _ := testOptions(dir)
	if code := DiffCommand(opts, []string{"--json", "old.json", "same.json"}); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	var changes []contract.Change
	if err := json.Unmarshal(out.Bytes(), &changes); err != nil {
		t.Fatalf("%v in %q", err, out.String())
	}
	if changes == nil || len(changes) != 0 {
		t.Fatalf("changes %+v, want an empty array", changes)
	}
}
