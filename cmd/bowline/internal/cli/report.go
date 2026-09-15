package cli

import (
	"encoding/json"
	"io"

	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/registry"
)

type CheckReport struct {
	Mode     string                 `json:"mode"`
	OK       bool                   `json:"ok"`
	Files    []CheckFile            `json:"files,omitempty"`
	Ref      string                 `json:"ref,omitempty"`
	Changes  []contract.Change      `json:"changes,omitempty"`
	Breaking int                    `json:"breaking"`
	Impact   *registry.ImpactReport `json:"impact,omitempty"`
}

type CheckFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

func writeReport(w io.Writer, report *CheckReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
