package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

const (
	DefaultPrefix  = "/api"
	DefaultTimeout = 30 * time.Second
	DefaultRetries = 2
)

var DefaultForwardHeaders = []string{"Authorization", "Cookie", "X-Request-Id", "Accept-Language"}

type Config struct {
	Listen         string
	Prefix         string
	Timeout        time.Duration
	ForwardHeaders []string
	Services       map[string]Upstream
}

type Upstream struct {
	URL      string
	Contract string
	Registry string
	Version  string
	Retries  int
}

type configFile struct {
	Listen         string                  `json:"listen"`
	Prefix         string                  `json:"prefix"`
	Timeout        string                  `json:"timeout"`
	ForwardHeaders []string                `json:"forwardHeaders"`
	Services       map[string]upstreamFile `json:"services"`
}

type upstreamFile struct {
	URL      string `json:"url"`
	Contract string `json:"contract"`
	Registry string `json:"registry"`
	Version  string `json:"version"`
	Retries  *int   `json:"retries"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return ParseConfig(data, path)
}

func ParseConfig(data []byte, path string) (*Config, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var file configFile
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg := &Config{
		Listen:         file.Listen,
		Prefix:         file.Prefix,
		Timeout:        DefaultTimeout,
		ForwardHeaders: file.ForwardHeaders,
		Services:       map[string]Upstream{},
	}
	if cfg.Prefix == "" {
		cfg.Prefix = DefaultPrefix
	}
	if cfg.ForwardHeaders == nil {
		cfg.ForwardHeaders = append([]string(nil), DefaultForwardHeaders...)
	}
	if file.Timeout != "" {
		d, err := time.ParseDuration(file.Timeout)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a duration such as \"30s\": %w", path, file.Timeout, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("%s: timeout must be positive, got %q", path, file.Timeout)
		}
		cfg.Timeout = d
	}
	if len(file.Services) == 0 {
		return nil, fmt.Errorf("%s: %q is required with at least one service", path, "services")
	}
	names := make([]string, 0, len(file.Services))
	for name := range file.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		up := file.Services[name]
		if up.URL == "" {
			return nil, fmt.Errorf("%s: service %q needs a \"url\"", path, name)
		}
		if up.Contract == "" && up.Registry == "" {
			return nil, fmt.Errorf("%s: service %q needs either a \"contract\" file or a \"registry\" to fetch from", path, name)
		}
		if up.Contract != "" && up.Registry != "" {
			return nil, fmt.Errorf("%s: service %q sets both \"contract\" and \"registry\"; pick one", path, name)
		}
		if up.Version == "" {
			return nil, fmt.Errorf("%s: service %q needs a \"version\" pinning the contract hash", path, name)
		}
		retries := DefaultRetries
		if up.Retries != nil {
			if *up.Retries < 0 {
				return nil, fmt.Errorf("%s: service %q has negative \"retries\"", path, name)
			}
			retries = *up.Retries
		}
		cfg.Services[name] = Upstream{URL: up.URL, Contract: up.Contract, Registry: up.Registry, Version: up.Version, Retries: retries}
	}
	return cfg, nil
}

func (c *Config) ServiceNames() []string {
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
