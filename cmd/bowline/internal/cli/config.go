package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configFile = "bowline.json"

type Config struct {
	Entry    string            `json:"entry"`
	Contract string            `json:"contract"`
	Schemas  bool              `json:"schemas"`
	Targets  map[string]Target `json:"targets"`
	OpenAPI  *OpenAPIConfig    `json:"openapi"`
	Dev      *DevConfig        `json:"dev"`
}

type DevConfig struct {
	App string `json:"app"`
}

type OpenAPIConfig struct {
	Out       string `json:"out,omitempty"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	ServerURL string `json:"serverUrl"`
	AutoPatch bool   `json:"autoPatch,omitempty"`
	ETags     bool   `json:"etags,omitempty"`
	Problem   bool   `json:"problemDetails,omitempty"`
}

type Target struct {
	Out     string `json:"out"`
	Zod     bool   `json:"zod,omitempty"`
	Format  string `json:"format,omitempty"`
	Package string `json:"package,omitempty"`
	Command string `json:"command,omitempty"`
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
		if target.Command == "" {
			continue
		}
		if _, builtin := Generators[name]; builtin || name == "tools" {
			return nil, fmt.Errorf("%s: target %q sets a \"command\" but %q is built in; rename the target or drop the command", configFile, name, name)
		}
		if strings.ContainsAny(target.Command, `/\`) {
			return nil, fmt.Errorf("%s: target %q: %q must be a command name resolved on PATH, not a path", configFile, name, target.Command)
		}
	}
	return &cfg, nil
}
