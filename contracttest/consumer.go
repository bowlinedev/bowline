package contracttest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type consumer struct {
	Bowline      string        `json:"bowline"`
	Consumer     string        `json:"consumer"`
	Provider     string        `json:"provider,omitempty"`
	Interactions []interaction `json:"interactions"`
}

type interaction struct {
	Procedure string          `json:"procedure"`
	Method    string          `json:"method,omitempty"`
	Input     json.RawMessage `json:"input"`
	Response  response        `json:"response"`
}

type response struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func loadConsumers(dir string) ([]consumer, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []consumer
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		c, err := parseConsumer(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Consumer < out[j].Consumer })
	return out, nil
}

func parseConsumer(data []byte) (consumer, error) {
	var c consumer
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Consumer == "" {
		return c, errors.New(`"consumer" is required`)
	}
	major, _, _ := strings.Cut(c.Bowline, ".")
	if major != "1" {
		return c, fmt.Errorf("consumer file format %q is not supported; this bowline reads 1.x", c.Bowline)
	}
	for i, in := range c.Interactions {
		if in.Procedure == "" {
			return c, fmt.Errorf("interaction %d has no procedure", i)
		}
	}
	return c, nil
}

func decode(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}
