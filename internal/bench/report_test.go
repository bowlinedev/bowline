package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleOutput = `goos: linux
goarch: amd64
pkg: github.com/bowlinedev/bowline
cpu: AMD EPYC 7763
BenchmarkRawNetHTTP-4   	  630266	      1900 ns/op	    1960 B/op	      25 allocs/op
BenchmarkRawNetHTTP-4   	  625116	      1881 ns/op	    1960 B/op	      25 allocs/op
BenchmarkBowline-4      	  604128	      1998 ns/op	    1992 B/op	      25 allocs/op
BenchmarkBowline-4      	  592932	      1950 ns/op	    1992 B/op	      25 allocs/op
BenchmarkDevLoopUpdate-4	       3	   8579903 ns/op	 7658658 B/op	   67784 allocs/op
BenchmarkGenerators/ts-4	   20000	      8927 ns/op	    8734 B/op	      94 allocs/op
BenchmarkGenerators/go-4	    3000	    387742 ns/op	  149604 B/op	    3197 allocs/op
PASS
ok  	github.com/bowlinedev/bowline	5.288s
`

const olderRun = `{
  "commit": "1111111111111111111111111111111111111111",
  "ranAt": "2026-09-01T00:00:00Z",
  "goos": "linux",
  "goarch": "amd64",
  "results": [
    {"name": "BenchmarkRawNetHTTP", "nsPerOp": 1900, "bytesPerOp": 1960, "allocsPerOp": 25},
    {"name": "BenchmarkBowline", "nsPerOp": 1960, "bytesPerOp": 1992, "allocsPerOp": 25},
    {"name": "BenchmarkDevLoopUpdate", "nsPerOp": 9000000, "bytesPerOp": 7658658, "allocsPerOp": 67784},
    {"name": "BenchmarkGenerators/ts", "nsPerOp": 9000, "bytesPerOp": 8734, "allocsPerOp": 94}
  ]
}`

const previousRun = `{
  "commit": "2222222222222222222222222222222222222222",
  "ranAt": "2026-09-08T00:00:00Z",
  "goos": "linux",
  "goarch": "amd64",
  "results": [
    {"name": "BenchmarkRawNetHTTP", "nsPerOp": 1880, "bytesPerOp": 1960, "allocsPerOp": 25},
    {"name": "BenchmarkBowline", "nsPerOp": 1940, "bytesPerOp": 1992, "allocsPerOp": 25},
    {"name": "BenchmarkDevLoopUpdate", "nsPerOp": 8000000, "bytesPerOp": 7658658, "allocsPerOp": 67784},
    {"name": "BenchmarkGenerators/ts", "nsPerOp": 8000, "bytesPerOp": 8734, "allocsPerOp": 94}
  ]
}`

func TestParseKeepsTheFastestRepeat(t *testing.T) {
	results, err := Parse(strings.NewReader(sampleOutput))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("got %d results: %+v", len(results), results)
	}
	run := Run{Results: results}
	raw, ok := run.Lookup(RawBenchmark)
	if !ok {
		t.Fatal("raw benchmark missing")
	}
	if raw.NsPerOp != 1881 {
		t.Fatalf("raw ns/op %v, want the fastest repeat 1881", raw.NsPerOp)
	}
	if raw.BytesPerOp != 1960 || raw.AllocsPerOp != 25 {
		t.Fatalf("raw memory stats %+v", raw)
	}
	if bowline, _ := run.Lookup(BowlineBenchmark); bowline.NsPerOp != 1950 {
		t.Fatalf("bowline ns/op %v, want 1950", bowline.NsPerOp)
	}
	generators := run.Generators()
	if len(generators) != 2 || generators[0].Name != GeneratorPrefix+"go" || generators[1].Name != GeneratorPrefix+"ts" {
		t.Fatalf("generators %+v", generators)
	}
}

func writeHistory(t *testing.T, dir string, documents ...string) {
	t.Helper()
	for i, document := range documents {
		name := filepath.Join(dir, string(rune('a'+i))+".json")
		if err := os.WriteFile(name, []byte(document), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadHistoryOrdersByRunTime(t *testing.T) {
	dir := t.TempDir()
	writeHistory(t, dir, previousRun, olderRun)
	history, err := LoadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("got %d runs", len(history))
	}
	if !strings.HasPrefix(history[0].Commit, "1111") || !strings.HasPrefix(history[1].Commit, "2222") {
		t.Fatalf("runs out of order: %s then %s", history[0].Commit, history[1].Commit)
	}
}

func TestLoadHistoryOnAnEmptyDirectory(t *testing.T) {
	history, err := LoadHistory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("got %d runs from an empty directory", len(history))
	}
}

func currentRun(t *testing.T) Run {
	t.Helper()
	results, err := Parse(strings.NewReader(sampleOutput))
	if err != nil {
		t.Fatal(err)
	}
	return Run{Commit: "3333333333333333333333333333333333333333", RanAt: mustTime(t, "2026-09-15T00:00:00Z"), GOOS: "linux", GOARCH: "amd64", Results: results}
}

func TestBenchReportRendersHistory(t *testing.T) {
	dir := t.TempDir()
	writeHistory(t, dir, olderRun, previousRun)
	history, err := LoadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	report := Render(currentRun(t), history)

	for _, want := range []string{
		"# Benchmarks",
		"## Handler overhead",
		"## Regeneration latency",
		"## Generator throughput",
		"## History",
		"## Regressions",
		"raw `net/http`",
		"| ts |",
		"| go |",
		"2026-09-01",
		"2026-09-08",
		"2026-09-15",
		"333333333333",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q\n%s", want, report)
		}
	}
	if strings.Contains(report, "Not measured in this run") {
		t.Errorf("every section should have data:\n%s", report)
	}
	if want := "Overhead is 3.7%"; !strings.Contains(report, want) {
		t.Errorf("report is missing %q\n%s", want, report)
	}
}

func TestRenderWithoutHistoryReportsNoRegressions(t *testing.T) {
	report := Render(currentRun(t), nil)
	if !strings.Contains(report, "None over 5%") {
		t.Errorf("expected no regressions without a baseline:\n%s", report)
	}
	if !strings.Contains(report, "2026-09-15") {
		t.Errorf("the current run should still appear in the history table:\n%s", report)
	}
}

func TestRegressionsFlagOnlySlowdownsOverTheBudget(t *testing.T) {
	dir := t.TempDir()
	writeHistory(t, dir, previousRun)
	history, err := LoadHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	regressions := Regressions(currentRun(t), history)
	if len(regressions) != 2 {
		t.Fatalf("got %d regressions: %+v", len(regressions), regressions)
	}
	if regressions[0].Name != GeneratorPrefix+"ts" {
		t.Fatalf("expected the worst regression first, got %+v", regressions)
	}
	if got := regressions[0].Percent; got < 11.5 || got > 11.7 {
		t.Fatalf("ts regression %.2f%%, want about 11.6%%", got)
	}
	if regressions[1].Name != DevLoopBenchmark {
		t.Fatalf("expected the dev loop regression second, got %+v", regressions)
	}
	report := Render(currentRun(t), history)
	if !strings.Contains(report, "Over 5% against the previous run") {
		t.Errorf("regressions missing from the report:\n%s", report)
	}
}

func TestRegressionsIgnoreImprovements(t *testing.T) {
	previous := Run{Results: []Result{{Name: BowlineBenchmark, NsPerOp: 5000}}}
	current := Run{Results: []Result{{Name: BowlineBenchmark, NsPerOp: 1000}}}
	if got := Regressions(current, []Run{previous}); len(got) != 0 {
		t.Fatalf("an improvement was flagged: %+v", got)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestRegressionsIgnoreAUniformlySlowerRunner(t *testing.T) {
	previous := Run{Results: []Result{
		{Name: baselineName, NsPerOp: 1622, AllocsPerOp: 25},
		{Name: "BenchmarkBowline", NsPerOp: 1571, AllocsPerOp: 23},
	}}
	slower := Run{Results: []Result{
		{Name: baselineName, NsPerOp: 2216, AllocsPerOp: 25},
		{Name: "BenchmarkBowline", NsPerOp: 2220, AllocsPerOp: 23},
	}}
	if got := Regressions(slower, []Run{previous}); len(got) != 0 {
		t.Fatalf("a 37%% slower runner reported %d regressions: %v", len(got), got)
	}
}

func TestRegressionsIgnoreAllocationJitterInLargeBenchmarks(t *testing.T) {
	previous := Run{Results: []Result{
		{Name: baselineName, NsPerOp: 1622, AllocsPerOp: 25},
		{Name: "BenchmarkDevLoopUpdate", NsPerOp: 6400000, AllocsPerOp: 68976},
	}}
	jittery := Run{Results: []Result{
		{Name: baselineName, NsPerOp: 1622, AllocsPerOp: 25},
		{Name: "BenchmarkDevLoopUpdate", NsPerOp: 6400000, AllocsPerOp: 68980},
	}}
	if got := Regressions(jittery, []Run{previous}); len(got) != 0 {
		t.Fatalf("four allocations out of 68976 reported %v", got)
	}
}

func TestRegressionsCatchAnExtraAllocation(t *testing.T) {
	previous := Run{Results: []Result{
		{Name: baselineName, NsPerOp: 1622, AllocsPerOp: 25},
		{Name: "BenchmarkBowline", NsPerOp: 1571, AllocsPerOp: 23},
	}}
	leaky := Run{Results: []Result{
		{Name: baselineName, NsPerOp: 1622, AllocsPerOp: 25},
		{Name: "BenchmarkBowline", NsPerOp: 1580, AllocsPerOp: 24},
	}}
	got := Regressions(leaky, []Run{previous})
	if len(got) != 1 || got[0].Metric != "allocs" {
		t.Fatalf("one extra allocation reported %v", got)
	}
}

func TestBlockingKeepsAllocationsAndDropsTimings(t *testing.T) {
	regressions := []Regression{
		{Name: "BenchmarkA", Metric: "time", Percent: 31},
		{Name: "BenchmarkB", Metric: "allocs", Percent: 4},
	}
	blocking := Blocking(regressions)
	if len(blocking) != 1 || blocking[0].Name != "BenchmarkB" {
		t.Fatalf("blocking = %v, want only the allocation regression", blocking)
	}
}
