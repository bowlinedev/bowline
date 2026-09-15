package ts

import (
	"path/filepath"
	"testing"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/goldens"
	"github.com/bowlinedev/bowline/contract"
)

func TestGoldens(t *testing.T) {
	goldens.Run(t, "ts", "ts", Generator{})
	for row, doc := range goldens.Rows(t) {
		t.Run(row+"/zod", func(t *testing.T) {
			zod, err := Generator{}.GenerateZodFor(doc, row+".golden.ts")
			if err != nil {
				t.Fatal(err)
			}
			goldens.Compare(t, "ts", filepath.Join("testdata", row+".zod.golden.ts"), zod)
		})
	}
}

func TestNameCollisions(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"example.com/app/users.Event": {Kind: contract.Struct, Name: "Event"},
		"example.com/app/audit.Event": {Kind: contract.Struct, Name: "Event"},
		"example.com/app/audit.Log":   {Kind: contract.Struct, Name: "Log"},
	}}
	names := assignNames(doc)
	if names["example.com/app/users.Event"] != "Users_Event" || names["example.com/app/audit.Event"] != "Audit_Event" || names["example.com/app/audit.Log"] != "Log" {
		t.Fatalf("%v", names)
	}
}
