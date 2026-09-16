package analyzer

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

var updateCorpus = flag.Bool("update-corpus", false, "rewrite corpus.json from the fidelity testdata")

func buildCorpus(t *testing.T) corpusFile {
	t.Helper()
	rows, err := filepath.Glob(filepath.Join("testdata", "fidelity", "rows", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no fidelity rows found")
	}
	file := corpusFile{Accepted: map[string]json.RawMessage{}}
	for _, dir := range rows {
		row := filepath.Base(dir)
		if data, err := os.ReadFile(filepath.Join(dir, "expected.contract.json")); err == nil {
			file.Accepted[row] = json.RawMessage(data)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "expected.diag.txt")); err == nil {
			file.Rejected = append(file.Rejected, row)
		}
	}
	sort.Strings(file.Rejected)
	return file
}

func TestCorpusMatchesTheFidelityTestdata(t *testing.T) {
	want, err := json.MarshalIndent(buildCorpus(t), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if *updateCorpus {
		if err := os.WriteFile("corpus.json", want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile("corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("corpus.json is stale; run go test ./internal/analyzer -update-corpus")
	}
}

func TestFidelityCorpusIsUsable(t *testing.T) {
	accepted := FidelityAccepted()
	if len(accepted) == 0 {
		t.Fatal("no accepted rows")
	}
	for row, data := range accepted {
		if _, err := parseCorpusRow(data); err != nil {
			t.Errorf("row %s: %v", row, err)
		}
	}
	rejected := FidelityRejected()
	if len(rejected) == 0 {
		t.Fatal("no rejected rows")
	}
	for _, row := range rejected {
		if _, both := accepted[row]; both {
			t.Errorf("row %s is both accepted and rejected", row)
		}
	}
}

func parseCorpusRow(data []byte) (map[string]any, error) {
	var doc map[string]any
	err := json.Unmarshal(data, &doc)
	return doc, err
}
