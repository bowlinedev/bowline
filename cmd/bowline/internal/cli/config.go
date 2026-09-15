package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const configFile = "bowline.json"

type Config struct {
	Entry    string            `json:"entry"`
	Contract string            `json:"contract"`
	Targets  map[string]Target `json:"targets"`
	OpenAPI  *OpenAPIConfig    `json:"openapi"`
}

type OpenAPIConfig struct {
	Title     string `json:"title"`
	Version   string `json:"version"`
	ServerURL string `json:"serverUrl"`
}

type Target struct {
	Out string `json:"out"`
}

func LoadConfig(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", configFile, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", configFile, err)
	}
	if cfg.Entry == "" {
		return nil, errors.New(configFile + `: "entry" is required, for example "./api.Routes"`)
	}
	if cfg.Contract == "" {
		cfg.Contract = "bowline.contract.json"
	}
	for name, target := range cfg.Targets {
		if target.Out == "" {
			return nil, fmt.Errorf("%s: target %q needs an \"out\" path", configFile, name)
		}
	}
	return &cfg, nil
}
