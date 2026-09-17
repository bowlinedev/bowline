package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/bowlinedev/bowline/internal/bench"
)

func main() {
	input := flag.String("in", "", "file holding go test -bench output; defaults to stdin")
	history := flag.String("history", "", "directory of previous run files")
	runOut := flag.String("run-out", "", "path to write this run as JSON")
	reportOut := flag.String("report-out", "", "path to write the rendered report; defaults to stdout")
	commit := flag.String("commit", os.Getenv("GITHUB_SHA"), "commit this run measured")
	failOnRegression := flag.Bool("fail-on-regression", false, "exit 1 when a benchmark regressed beyond the budget")
	flag.Parse()

	if err := run(*input, *history, *runOut, *reportOut, *commit, *failOnRegression); err != nil {
		fmt.Fprintf(os.Stderr, "bench-report: %v\n", err)
		os.Exit(1)
	}
}

func run(input, history, runOut, reportOut, commit string, failOnRegression bool) error {
	source := os.Stdin
	if input != "" {
		file, err := os.Open(input)
		if err != nil {
			return err
		}
		defer file.Close()
		source = file
	}
	results, err := bench.Parse(source)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return errors.New("no benchmark results found in the input")
	}
	current := bench.Run{
		Commit:  commit,
		RanAt:   time.Now().UTC().Truncate(time.Second),
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		Results: results,
	}

	var past []bench.Run
	if history != "" {
		if _, err := os.Stat(history); err == nil {
			past, err = bench.LoadHistory(history)
			if err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	if runOut != "" {
		data, err := json.MarshalIndent(current, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(runOut), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(runOut, append(data, '\n'), 0o644); err != nil {
			return err
		}
	}

	report := bench.Render(current, past)
	if reportOut == "" {
		fmt.Print(report)
	} else {
		if err := os.MkdirAll(filepath.Dir(reportOut), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(reportOut, []byte(report), 0o644); err != nil {
			return err
		}
	}

	regressions := bench.Regressions(current, past)
	for _, regression := range regressions {
		fmt.Fprintln(os.Stderr, regression)
	}
	if blocking := bench.Blocking(regressions); failOnRegression && len(blocking) > 0 {
		return fmt.Errorf("%d benchmark(s) allocate more than the previous run", len(blocking))
	}
	return nil
}
