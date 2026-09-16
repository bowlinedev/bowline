package reserved

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

const (
	ContractPath = ".bowline/contract"
	HealthPath   = ".bowline/health"
)

type Set struct {
	Document []byte
	Health   []byte
}

func New(document []byte) (*Set, error) {
	doc, err := contract.Parse(document)
	if err != nil {
		return nil, err
	}
	hash := doc.Hash
	if hash == "" {
		computed, err := doc.ComputeHash()
		if err != nil {
			return nil, err
		}
		hash = computed
	}
	health, err := json.Marshal(struct {
		OK   bool   `json:"ok"`
		Hash string `json:"hash"`
	}{OK: true, Hash: hash})
	if err != nil {
		return nil, err
	}
	return &Set{Document: document, Health: health}, nil
}

func MustNew(document []byte) *Set {
	set, err := New(document)
	if err != nil {
		panic(fmt.Sprintf("bowline: WithContract: %v", err))
	}
	return set
}

func Path(urlPath string) string {
	trimmed := strings.TrimSuffix(urlPath, "/")
	for _, name := range []string{ContractPath, HealthPath} {
		if trimmed == name || trimmed == "/"+name || strings.HasSuffix(trimmed, "/"+name) {
			return name
		}
	}
	return ""
}
