package contract

import (
	"crypto/sha256"
	"encoding/hex"
)

func (d *Document) ComputeHash() (string, error) {
	stripped := *d
	stripped.Hash = ""
	stripped.Positions = nil
	data, err := stripped.Marshal()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (d *Document) SetHash() error {
	h, err := d.ComputeHash()
	if err != nil {
		return err
	}
	d.Hash = h
	return nil
}
