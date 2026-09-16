package analyzer

import (
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed corpus.json
var corpusJSON []byte

type corpusFile struct {
	Accepted map[string]json.RawMessage `json:"accepted"`
	Rejected []string                   `json:"rejected"`
}

func loadCorpus() corpusFile {
	var file corpusFile
	if err := json.Unmarshal(corpusJSON, &file); err != nil {
		panic("analyzer: corpus.json is malformed: " + err.Error())
	}
	return file
}

func FidelityAccepted() map[string][]byte {
	file := loadCorpus()
	rows := make(map[string][]byte, len(file.Accepted))
	for row, data := range file.Accepted {
		rows[row] = []byte(data)
	}
	return rows
}

func FidelityRejected() []string {
	rows := append([]string(nil), loadCorpus().Rejected...)
	sort.Strings(rows)
	return rows
}
