package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type ImpactReport struct {
	Baseline            string            `json:"baseline"`
	Changes             []contract.Change `json:"changes"`
	Affected            []Affected        `json:"affected"`
	Unattributed        []contract.Change `json:"unattributed"`
	OK                  bool              `json:"ok"`
	SkippedCompositions bool              `json:"skippedCompositions,omitempty"`
}

type Affected struct {
	Consumer string          `json:"consumer"`
	Via      string          `json:"via,omitempty"`
	Change   contract.Change `json:"change"`
	Reason   string          `json:"reason"`
}

type Composer func(map[string]*contract.Document) (*contract.Document, error)

type ImpactOptions struct {
	Strict   bool
	Composer Composer
}

func Impact(ctx context.Context, store Store, service string, candidate *contract.Document, strict bool) (*ImpactReport, error) {
	return ImpactWith(ctx, store, service, candidate, ImpactOptions{Strict: strict})
}

func ImpactWith(ctx context.Context, store Store, service string, candidate *contract.Document, opts ImpactOptions) (*ImpactReport, error) {
	report := &ImpactReport{Changes: []contract.Change{}, Affected: []Affected{}, Unattributed: []contract.Change{}, OK: true}
	baseline, err := store.Tagged(ctx, service, "main")
	if errors.Is(err, ErrNotFound) {
		return report, nil
	}
	if err != nil {
		return nil, err
	}
	previous, err := contract.Parse(baseline.Contract)
	if err != nil {
		return nil, fmt.Errorf("registry: the stored contract for %s@%s does not parse: %w", service, baseline.Hash, err)
	}
	report.Baseline = baseline.Hash
	report.Changes = contract.Diff(previous, candidate)

	hits := map[int][]Affected{}
	direct, err := store.Consumers(ctx, service)
	if err != nil {
		return nil, err
	}
	attribute(report.Changes, direct, "", hits)

	compositions, err := store.Compositions(ctx, service)
	if err != nil {
		return nil, err
	}
	if len(compositions) > 0 && opts.Composer == nil {
		report.SkippedCompositions = true
	}
	if opts.Composer != nil {
		if err := attributeThroughGateways(ctx, store, service, candidate, opts.Composer, compositions, report.Changes, hits); err != nil {
			return nil, err
		}
	}

	for i, change := range report.Changes {
		if !attributable(change) {
			continue
		}
		found := hits[i]
		if len(found) == 0 {
			report.Unattributed = append(report.Unattributed, change)
			continue
		}
		report.Affected = append(report.Affected, found...)
	}
	sort.SliceStable(report.Affected, func(i, j int) bool {
		a, b := report.Affected[i], report.Affected[j]
		if a.Consumer != b.Consumer {
			return a.Consumer < b.Consumer
		}
		if a.Via != b.Via {
			return a.Via < b.Via
		}
		if a.Change.Path != b.Change.Path {
			return a.Change.Path < b.Change.Path
		}
		return a.Change.Message < b.Change.Message
	})
	report.OK = len(report.Affected) == 0 && (!opts.Strict || len(report.Unattributed) == 0)
	return report, nil
}

func attributable(c contract.Change) bool {
	return c.Category == contract.Breaking || c.Category == contract.Narrowed
}

func attribute(changes []contract.Change, consumers []Consumer, via string, hits map[int][]Affected) {
	for _, consumer := range consumers {
		used := parseUsage(consumer.Usage)
		for i, change := range changes {
			if !attributable(change) {
				continue
			}
			reason, ok := match(change, used)
			if !ok {
				continue
			}
			hits[i] = append(hits[i], Affected{Consumer: consumer.Consumer, Via: via, Change: change, Reason: reason})
		}
	}
}

func attributeThroughGateways(ctx context.Context, store Store, service string, candidate *contract.Document, compose Composer, compositions []Composition, direct []contract.Change, hits map[int][]Affected) error {
	index := map[contract.Change]int{}
	for i, c := range direct {
		if _, ok := index[c]; !ok {
			index[c] = i
		}
	}
	seen := map[string]bool{}
	for _, composition := range compositions {
		if seen[composition.Gateway] {
			continue
		}
		seen[composition.Gateway] = true
		consumers, err := store.Consumers(ctx, composition.Gateway)
		if err != nil {
			return err
		}
		if len(consumers) == 0 {
			continue
		}
		before := map[string]*contract.Document{}
		complete := true
		for name, hash := range composition.Services {
			version, err := store.Version(ctx, name, hash)
			if errors.Is(err, ErrNotFound) {
				complete = false
				break
			}
			if err != nil {
				return err
			}
			doc, err := contract.Parse(version.Contract)
			if err != nil {
				complete = false
				break
			}
			before[name] = doc
		}
		if !complete {
			continue
		}
		after := map[string]*contract.Document{}
		maps.Copy(after, before)
		after[service] = candidate
		oldComposed, err := compose(before)
		if err != nil {
			return err
		}
		newComposed, err := compose(after)
		if err != nil {
			return err
		}
		composed := contract.Diff(oldComposed, newComposed)
		gatewayHits := map[int][]Affected{}
		attribute(composed, consumers, composition.Gateway, gatewayHits)
		positions := make([]int, 0, len(gatewayHits))
		for i := range gatewayHits {
			positions = append(positions, i)
		}
		sort.Ints(positions)
		for _, i := range positions {
			target, ok := index[unprefix(composed[i], service)]
			if !ok {
				continue
			}
			for _, hit := range gatewayHits[i] {
				hit.Change = direct[target]
				hits[target] = append(hits[target], hit)
			}
		}
	}
	return nil
}

func unprefix(c contract.Change, service string) contract.Change {
	tokens := strings.Fields(c.Path)
	if len(tokens) >= 2 && tokens[0] == "procedure" {
		tokens[1] = strings.TrimPrefix(tokens[1], service+".")
		c.Path = strings.Join(tokens, " ")
	}
	return c
}

type usage struct {
	procedures map[string]*procedureUsage
}

type procedureUsage struct {
	input  map[string]bool
	output map[string]bool
}

type consumerDocument struct {
	Interactions []struct {
		Procedure string          `json:"procedure"`
		Input     json.RawMessage `json:"input"`
		Response  struct {
			Status int             `json:"status"`
			Body   json.RawMessage `json:"body"`
		} `json:"response"`
	} `json:"interactions"`
	Procedures map[string]struct {
		Input  []string `json:"input"`
		Output []string `json:"output"`
	} `json:"procedures"`
}

func parseUsage(raw json.RawMessage) *usage {
	u := &usage{procedures: map[string]*procedureUsage{}}
	if len(raw) == 0 {
		return u
	}
	var doc consumerDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return u
	}
	for name, summary := range doc.Procedures {
		p := u.procedure(name)
		for _, path := range summary.Input {
			p.input[path] = true
		}
		for _, path := range summary.Output {
			p.output[path] = true
		}
	}
	for _, interaction := range doc.Interactions {
		if interaction.Procedure == "" {
			continue
		}
		p := u.procedure(interaction.Procedure)
		collectPaths(interaction.Input, "", p.input)
		if interaction.Response.Status < 400 {
			collectPaths(interaction.Response.Body, "", p.output)
		}
	}
	return u
}

func (u *usage) procedure(name string) *procedureUsage {
	p, ok := u.procedures[name]
	if !ok {
		p = &procedureUsage{input: map[string]bool{}, output: map[string]bool{}}
		u.procedures[name] = p
	}
	return p
}

func collectPaths(raw json.RawMessage, prefix string, into map[string]bool) {
	if len(raw) == 0 {
		return
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return
	}
	walkValue(value, prefix, into)
}

func walkValue(value any, prefix string, into map[string]bool) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			path := join(prefix, key)
			into[path] = true
			walkValue(child, path, into)
		}
	case []any:
		path := join(prefix, "*")
		if prefix != "" {
			into[path] = true
		}
		for _, child := range v {
			walkValue(child, path, into)
		}
	}
}

func join(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return prefix + "." + segment
}

func match(change contract.Change, used *usage) (string, bool) {
	procedure, side, path := parseChangePath(change.Path)
	if procedure == "" {
		return "", false
	}
	p, ok := used.procedures[procedure]
	if !ok {
		return "", false
	}
	joined := strings.Join(path, ".")
	switch side {
	case "output":
		if joined == "" {
			return "calls " + procedure, true
		}
		if p.output[joined] {
			return "reads " + joined, true
		}
		return "", false
	case "input":
		if joined != "" && p.input[joined] {
			return "sends " + joined, true
		}
		return "calls " + procedure, true
	default:
		return "calls " + procedure, true
	}
}

func parseChangePath(changePath string) (procedure, side string, path []string) {
	tokens := strings.Fields(changePath)
	if len(tokens) < 2 || tokens[0] != "procedure" {
		return "", "", nil
	}
	procedure = tokens[1]
	rest := tokens[2:]
	if len(rest) == 0 {
		return procedure, "", nil
	}
	switch rest[0] {
	case "input", "output":
		side = rest[0]
		rest = rest[1:]
	case "error":
		if len(rest) > 1 {
			return procedure, "error", []string{rest[1]}
		}
		return procedure, "error", nil
	default:
		return procedure, "", nil
	}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "field":
			if i+1 < len(rest) {
				path = append(path, rest[i+1])
				i++
			}
		case "elem", "value":
			path = append(path, "*")
		}
	}
	return procedure, side, path
}

func FormatText(report *ImpactReport) string {
	var b strings.Builder
	if report.Baseline == "" {
		b.WriteString("no baseline tagged main; nothing to compare\n")
		return b.String()
	}
	fmt.Fprintf(&b, "baseline  %s\n", report.Baseline)
	if len(report.Changes) == 0 {
		b.WriteString("no contract changes\n")
		return b.String()
	}
	for _, c := range report.Changes {
		fmt.Fprintf(&b, "%-9s %s: %s\n", c.Category, c.Path, c.Message)
	}
	for _, a := range report.Affected {
		via := ""
		if a.Via != "" {
			via = " through " + a.Via
		}
		fmt.Fprintf(&b, "breaks    %s%s: %s (%s)\n", a.Consumer, via, a.Change.Path, a.Reason)
	}
	for _, c := range report.Unattributed {
		fmt.Fprintf(&b, "unused    %s: %s\n", c.Path, c.Message)
	}
	if report.SkippedCompositions {
		b.WriteString("note      compositions were not recomposed; no composer was supplied\n")
	}
	if report.OK {
		b.WriteString("ok        no known consumer breaks\n")
	} else {
		fmt.Fprintf(&b, "bowline: %d consumer break(s)\n", len(report.Affected))
	}
	return b.String()
}
