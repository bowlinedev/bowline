package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionIncrementalUpdate(t *testing.T) {
	dir := t.TempDir()
	writeSynth(t, dir, 4, 5, 3)
	s, err := NewSession(dir, testEnv(), "./...")
	if err != nil {
		t.Fatal(err)
	}
	doc, diags := Analyze(s.Program(), "./api.Routes")
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	before := doc.Hash
	if len(doc.Procedures) != 12 {
		t.Fatalf("got %d procedures", len(doc.Procedures))
	}
	file := filepath.Join(dir, "p01", "p01.go")
	src, _ := os.ReadFile(file)
	edited := strings.Replace(string(src), "Name string `json:\"name\" validate:\"required\"`", "Name string `json:\"title\" validate:\"required\"`", 1)
	os.WriteFile(file, []byte(edited), 0o644)
	stats, typeErrs, err := s.Update([]string{file})
	if err != nil || len(typeErrs) > 0 {
		t.Fatalf("update: %v %v", err, typeErrs)
	}
	if stats.Full {
		t.Fatal("expected an incremental update")
	}
	want := []string{"synth.test/p01", "synth.test/p02", "synth.test/p03", "synth.test/api"}
	if strings.Join(stats.Rechecked, ",") != strings.Join(want, ",") {
		t.Fatalf("rechecked %v, want %v", stats.Rechecked, want)
	}
	doc, diags = Analyze(s.Program(), "./api.Routes")
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	if doc.Hash == before {
		t.Fatal("hash did not change after an edit")
	}
	if _, ok := doc.Types["synth.test/p01.T0"]; !ok {
		t.Fatal("type missing after update")
	}
	if doc.Types["synth.test/p01.T0"].Fields[1].Name != "title" {
		t.Fatalf("edit not reflected: %+v", doc.Types["synth.test/p01.T0"].Fields[1])
	}
}

func TestSessionReportsTypeErrors(t *testing.T) {
	dir := t.TempDir()
	writeSynth(t, dir, 2, 2, 1)
	s, err := NewSession(dir, testEnv(), "./...")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "p00", "p00.go")
	src, _ := os.ReadFile(file)
	os.WriteFile(file, []byte(strings.Replace(string(src), "{}, nil }", "{} }", 1)), 0o644)
	_, typeErrs, err := s.Update([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	if len(typeErrs) == 0 || !strings.Contains(typeErrs[0].Pos.Filename, filepath.Join("p00", "p00.go")) {
		t.Fatalf("expected a type error in p00, got %v", typeErrs)
	}
	os.WriteFile(file, src, 0o644)
	if _, typeErrs, err := s.Update([]string{file}); err != nil || len(typeErrs) > 0 {
		t.Fatalf("recovery failed: %v %v", err, typeErrs)
	}
	if _, diags := Analyze(s.Program(), "./api.Routes"); len(diags) > 0 {
		t.Fatal(diags)
	}
}

func TestSessionFallsBackToFullReload(t *testing.T) {
	dir := t.TempDir()
	writeSynth(t, dir, 2, 2, 1)
	s, err := NewSession(dir, testEnv(), "./...")
	if err != nil {
		t.Fatal(err)
	}
	newFile := filepath.Join(dir, "p00", "extra.go")
	os.WriteFile(newFile, []byte("package p00\n\ntype Extra struct{ N int32 `json:\"n\"` }\n"), 0o644)
	stats, typeErrs, err := s.Update([]string{newFile})
	if err != nil || len(typeErrs) > 0 || !stats.Full {
		t.Fatalf("expected full reload: %+v %v %v", stats, typeErrs, err)
	}
	if s.Program().Package("synth.test/p00").Types.Scope().Lookup("Extra") == nil {
		t.Fatal("new file not loaded")
	}
}

func TestDevLoopBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	dir := t.TempDir()
	writeSynth(t, dir, 20, 25, 10)
	s, err := NewSession(dir, testEnv(), "./...")
	if err != nil {
		t.Fatal(err)
	}
	if _, diags := Analyze(s.Program(), "./api.Routes"); len(diags) > 0 {
		t.Fatal(diags)
	}
	file := filepath.Join(dir, "p10", "p10.go")
	src, _ := os.ReadFile(file)
	var durations []time.Duration
	for i := 0; i < 5; i++ {
		tag := "name"
		if i%2 == 0 {
			tag = "label"
		}
		os.WriteFile(file, []byte(strings.Replace(string(src), "`json:\"name\" validate:\"required\"`", "`json:\""+tag+"\" validate:\"required\"`", 1)), 0o644)
		start := time.Now()
		if _, typeErrs, err := s.Update([]string{file}); err != nil || len(typeErrs) > 0 {
			t.Fatal(err, typeErrs)
		}
		if _, diags := Analyze(s.Program(), "./api.Routes"); len(diags) > 0 {
			t.Fatal(diags)
		}
		durations = append(durations, time.Since(start))
	}
	median := durations[len(durations)/2]
	t.Logf("update+analyze durations: %v", durations)
	if median > 500*time.Millisecond {
		t.Fatalf("median %v exceeds the 500ms budget", median)
	}
}
