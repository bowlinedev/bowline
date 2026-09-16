package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/analyzer"
	"github.com/bowlinedev/bowline/contract"
)

const certifyUsage = "usage: bowline certify --target <name> --generator <command> [--config certify.json] [--out docs/certified.md] [--report]"

const certifyConfigFile = "certify.json"

var defaultEscapeHatches = map[string][]string{
	"ts":     {"any"},
	"go":     {"interface{}"},
	"dart":   {"dynamic"},
	"python": {"Any"},
	"rust":   {"serde_json::Value"},
}

var defaultExtensions = map[string]string{
	"ts":     "ts",
	"go":     "go",
	"dart":   "dart",
	"python": "py",
	"rust":   "rs",
	"elixir": "ex",
}

type CertifyConfig struct {
	Target      string   `json:"target"`
	Generator   string   `json:"generator"`
	Repository  string   `json:"repository"`
	Version     string   `json:"version"`
	Extension   string   `json:"extension"`
	EscapeHatch []string `json:"escapeHatch"`
	Compile     []string `json:"compile"`
	Test        []string `json:"test"`
	Conformance string   `json:"conformance"`
}

type certifyCheck struct {
	Name    string
	Passed  bool
	Skipped bool
	Detail  string
}

type filesProducer interface {
	GenerateFiles(doc *contract.Document, out string) (map[string][]byte, []analyzer.Diagnostic, error)
}

type builtinProducer struct {
	name      string
	generator Generator
}

func (b builtinProducer) GenerateFiles(doc *contract.Document, out string) (map[string][]byte, []analyzer.Diagnostic, error) {
	content, err := b.generator.Generate(doc, out)
	if err != nil {
		return nil, nil, fmt.Errorf("target %s: %w", b.name, err)
	}
	return map[string][]byte{out: content}, nil, nil
}

func Certify(opts Options, args []string) int {
	flags := flag.NewFlagSet("certify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	target := flags.String("target", "", "the target name being certified")
	generator := flags.String("generator", "", "the generator command, or a built-in target name")
	configPath := flags.String("config", "", "certify.json describing the toolchain commands")
	out := flags.String("out", filepath.Join("docs", "certified.md"), "the certified list to update with --report")
	report := flags.Bool("report", false, "write the target's row into the certified list")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: certify: %v\n%s\n", err, certifyUsage)
		return 2
	}
	cfg, err := loadCertifyConfig(opts.Dir, *configPath, *target, *generator)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n%s\n", err, certifyUsage)
		return 2
	}
	checks, err := runCertification(opts, cfg)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
		return 1
	}
	passed, skipped := 0, 0
	for _, check := range checks {
		switch {
		case check.Skipped:
			skipped++
			fmt.Fprintf(opts.Stdout, "skip      %s: %s\n", check.Name, check.Detail)
		case check.Passed:
			passed++
			fmt.Fprintf(opts.Stdout, "ok        %s\n", check.Name)
		default:
			fmt.Fprintf(opts.Stderr, "failed    %s: %s\n", check.Name, check.Detail)
		}
	}
	failed := len(checks) - passed - skipped
	if failed > 0 {
		fmt.Fprintf(opts.Stderr, "bowline: %s is not certified; %d check(s) failed\n", cfg.Target, failed)
		return 1
	}
	if skipped > 0 {
		fmt.Fprintf(opts.Stderr, "bowline: %s is not certified; %d check(s) could not run\n", cfg.Target, skipped)
		return 1
	}
	fmt.Fprintf(opts.Stdout, "certified %s with %s\n", cfg.Target, cfg.Generator)
	if *report {
		if err := writeCertified(opts, cfg, *out); err != nil {
			fmt.Fprintf(opts.Stderr, "bowline: %v\n", err)
			return 1
		}
		fmt.Fprintf(opts.Stdout, "wrote %s\n", *out)
	}
	return 0
}

func loadCertifyConfig(dir, explicit, target, generator string) (*CertifyConfig, error) {
	cfg := &CertifyConfig{}
	path := explicit
	if path == "" {
		path = certifyConfigFile
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(dir, filepath.FromSlash(path))
	}
	data, err := os.ReadFile(full)
	switch {
	case err == nil:
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	case explicit != "":
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if target != "" {
		cfg.Target = target
	}
	if generator != "" {
		cfg.Generator = generator
	}
	if cfg.Target == "" {
		return nil, errors.New("certify: --target is required")
	}
	if cfg.Generator == "" {
		return nil, errors.New("certify: --generator is required")
	}
	if cfg.Extension == "" {
		cfg.Extension = defaultExtensions[cfg.Target]
	}
	if cfg.Extension == "" {
		cfg.Extension = "txt"
	}
	if len(cfg.EscapeHatch) == 0 {
		cfg.EscapeHatch = defaultEscapeHatches[cfg.Target]
	}
	return cfg, nil
}

func producerFor(cfg *CertifyConfig, env []string) filesProducer {
	if builtin, ok := Generators[cfg.Generator]; ok {
		return builtinProducer{name: cfg.Generator, generator: builtin}
	}
	return externalGenerator{name: cfg.Target, command: cfg.Generator, env: env}
}

func runCertification(opts Options, cfg *CertifyConfig) ([]certifyCheck, error) {
	env := opts.Env
	if env == nil {
		env = os.Environ()
	}
	producer := producerFor(cfg, env)

	accepted := analyzer.FidelityAccepted()
	if len(accepted) == 0 {
		return nil, errors.New("certify: the fidelity corpus is empty")
	}
	rows := slices.Sorted(maps.Keys(accepted))

	work, err := os.MkdirTemp("", "bowline-certify-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)

	out := "bowline." + cfg.Extension
	first := map[string]map[string][]byte{}
	var fidelityFailures, hatchFailures, determinismFailures []string

	for _, row := range rows {
		doc, err := contract.Parse(accepted[row])
		if err != nil {
			return nil, fmt.Errorf("certify: fidelity row %s: %w", row, err)
		}
		files, diags, err := producer.GenerateFiles(doc, out)
		if err != nil {
			fidelityFailures = append(fidelityFailures, fmt.Sprintf("%s: %v", row, err))
			continue
		}
		if len(diags) > 0 {
			fidelityFailures = append(fidelityFailures, fmt.Sprintf("%s: the generator rejected an accepted row: %s", row, diags[0].String()))
			continue
		}
		first[row] = files
	}

	baseline, baselineErr := escapeHatchBaseline(producer, cfg, out)
	if baselineErr != nil {
		return nil, baselineErr
	}
	for _, row := range rows {
		files, ok := first[row]
		if !ok || usesRaw(accepted[row]) {
			continue
		}
		if token, where, count := findEscapeHatch(files, cfg.EscapeHatch, baseline); token != "" {
			hatchFailures = append(hatchFailures, fmt.Sprintf("%s: %s uses %q %d time(s) beyond the generator's own runtime code and the row has no raw primitive", row, where, token, count))
		}
	}

	for _, row := range rows {
		if _, ok := first[row]; !ok {
			continue
		}
		doc, err := contract.Parse(accepted[row])
		if err != nil {
			return nil, err
		}
		second, _, err := producer.GenerateFiles(doc, out)
		if err != nil {
			determinismFailures = append(determinismFailures, fmt.Sprintf("%s: the second run failed: %v", row, err))
			continue
		}
		if diff := compareFiles(first[row], second); diff != "" {
			determinismFailures = append(determinismFailures, fmt.Sprintf("%s: %s", row, diff))
		}
	}

	if err := writeCorpusOutput(work, first); err != nil {
		return nil, err
	}

	checks := []certifyCheck{
		result("fidelity rows generate", fidelityFailures, fmt.Sprintf("%d rows", len(rows))),
		result("no escape-hatch types", hatchFailures, describeHatches(cfg.EscapeHatch)),
		result("generator is deterministic", determinismFailures, "two runs per row"),
		rejectRowCheck(),
	}
	checks = append(checks, toolchainCheck("goldens compile", cfg.Compile, opts.Dir, work, env))
	checks = append(checks, toolchainCheck("runtime tests", cfg.Test, opts.Dir, work, env))
	return checks, nil
}

func result(name string, failures []string, detail string) certifyCheck {
	if len(failures) == 0 {
		return certifyCheck{Name: name, Passed: true, Detail: detail}
	}
	return certifyCheck{Name: name, Detail: strings.Join(failures, "; ")}
}

func describeHatches(hatches []string) string {
	if len(hatches) == 0 {
		return "no escape-hatch type declared for this target"
	}
	return strings.Join(hatches, ", ")
}

func rejectRowCheck() certifyCheck {
	rejected := analyzer.FidelityRejected()
	if len(rejected) == 0 {
		return certifyCheck{Name: "reject rows stop at the analyzer", Detail: "the corpus has no reject rows"}
	}
	accepted := analyzer.FidelityAccepted()
	var reachable []string
	for _, row := range rejected {
		if _, ok := accepted[row]; ok {
			reachable = append(reachable, row)
		}
	}
	return result("reject rows stop at the analyzer", reachable, fmt.Sprintf("%d rows never reach a generator", len(rejected)))
}

func toolchainCheck(name string, command []string, projectDir, workDir string, env []string) certifyCheck {
	if len(command) == 0 {
		return certifyCheck{Name: name, Skipped: true, Detail: "certify.json declares no command"}
	}
	program := command[0]
	if !filepath.IsAbs(program) && strings.ContainsAny(program, `/\`) {
		program = filepath.Join(projectDir, filepath.FromSlash(program))
	}
	cmd := exec.Command(program, command[1:]...)
	cmd.Dir = projectDir
	cmd.Env = append(append([]string{}, env...), "BOWLINE_CERTIFY_DIR="+workDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return certifyCheck{Name: name, Detail: detail}
	}
	return certifyCheck{Name: name, Passed: true, Detail: strings.Join(command, " ")}
}

func usesRaw(document []byte) bool {
	return bytes.Contains(document, []byte(`"raw"`))
}

const emptyDocument = `{"bowline":"1.2","types":{},"errors":{},"procedures":[]}`

func escapeHatchBaseline(producer filesProducer, cfg *CertifyConfig, out string) (map[string]int, error) {
	baseline := map[string]int{}
	if len(cfg.EscapeHatch) == 0 {
		return baseline, nil
	}
	doc, err := contract.Parse([]byte(emptyDocument))
	if err != nil {
		return nil, fmt.Errorf("certify: building the escape-hatch baseline: %w", err)
	}
	files, _, err := producer.GenerateFiles(doc, out)
	if err != nil {
		return baseline, nil
	}
	for _, content := range files {
		for _, hatch := range cfg.EscapeHatch {
			baseline[hatch] += bytes.Count(content, []byte(hatch))
		}
	}
	return baseline, nil
}

func findEscapeHatch(files map[string][]byte, hatches []string, baseline map[string]int) (string, string, int) {
	names := slices.Sorted(maps.Keys(files))
	counts := map[string]int{}
	for _, name := range names {
		for _, hatch := range hatches {
			counts[hatch] += bytes.Count(files[name], []byte(hatch))
		}
	}
	for _, name := range names {
		for _, hatch := range hatches {
			if !bytes.Contains(files[name], []byte(hatch)) {
				continue
			}
			if over := counts[hatch] - baseline[hatch]; over > 0 {
				return hatch, name, over
			}
		}
	}
	return "", "", 0
}

func compareFiles(first, second map[string][]byte) string {
	if len(first) != len(second) {
		return fmt.Sprintf("the first run wrote %d file(s), the second %d", len(first), len(second))
	}
	names := slices.Sorted(maps.Keys(first))
	for _, name := range names {
		other, ok := second[name]
		if !ok {
			return fmt.Sprintf("the second run did not write %s", name)
		}
		if !bytes.Equal(first[name], other) {
			return fmt.Sprintf("%s differs between two runs", name)
		}
	}
	return ""
}

func writeCorpusOutput(dir string, rows map[string]map[string][]byte) error {
	for row, files := range rows {
		for name, content := range files {
			path := filepath.Join(dir, row, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeCertified(opts Options, cfg *CertifyConfig, out string) error {
	path := out
	if !filepath.IsAbs(path) {
		path = filepath.Join(opts.Dir, filepath.FromSlash(out))
	}
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	row := certifiedRow(cfg)
	body := string(existing)
	if body == "" {
		body = certifiedHeader
	}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	prefix := "| `" + cfg.Target + "` |"
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = row
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, row)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func certifiedRow(cfg *CertifyConfig) string {
	repository := cfg.Repository
	if repository == "" {
		repository = "built in"
	}
	version := cfg.Version
	if version == "" {
		version = bowline.Version
	}
	return fmt.Sprintf("| `%s` | %s | %s | %s | %s |", cfg.Target, repository, version, bowline.Version, time.Now().UTC().Format("2006-01-02"))
}

const certifiedHeader = `# Certified generators

Every row here passed ` + "`bowline certify`" + ` against the fidelity corpus: it generates every accepted row, emits no escape-hatch type where the contract has no raw primitive, is deterministic across two runs, compiles with the target's own toolchain, and leaves the target's runtime package tests passing. ` + "`docs/certification.md`" + ` describes the checks and how to submit a generator.

| Target | Generator | Version certified | Bowline version | Date |
|---|---|---|---|---|
`
