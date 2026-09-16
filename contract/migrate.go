package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

func Migrate(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("contract: migrate: %w", err)
	}
	if doc.Bowline == "" {
		return nil, errors.New("contract: migrate: document has no version")
	}
	if checkVersion(doc.Bowline) == nil {
		return data, nil
	}
	if doc.Bowline[0] != '0' {
		return nil, fmt.Errorf("contract: migrate: cannot migrate version %q to %q", doc.Bowline, Version)
	}
	doc.Bowline = Version
	if doc.Errors == nil {
		doc.Errors = map[string]*ErrorDecl{}
	}
	if err := doc.SetHash(); err != nil {
		return nil, err
	}
	return doc.Marshal()
}
