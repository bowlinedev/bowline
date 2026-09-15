package elixir

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/goldens"
	"github.com/bowlinedev/bowline/cmd/bowline/internal/gen/naming"
	"github.com/bowlinedev/bowline/contract"
)

func TestGoldens(t *testing.T) {
	for row, doc := range goldens.Rows(t) {
		t.Run(row, func(t *testing.T) {
			got, err := Generator{Package: "Golden." + naming.UpperCamel(row)}.Generate(doc, "bowline.ex")
			if err != nil {
				t.Fatal(err)
			}
			goldens.Compare(t, "elixir", filepath.Join("testdata", row+".golden.ex"), got)
		})
	}
}

func TestGoldensAreFormatted(t *testing.T) {
	if _, err := exec.LookPath("mix"); err != nil {
		t.Skip("mix is not installed")
	}
	files, err := filepath.Glob(filepath.Join("testdata", "*.golden.ex"))
	if err != nil || len(files) == 0 {
		t.Fatal("no goldens")
	}
	abs := make([]string, len(files))
	for i, f := range files {
		abs[i], _ = filepath.Abs(f)
	}
	cmd := exec.Command("mix", append([]string{"format", "--check-formatted"}, abs...)...)
	cmd.Dir = t.TempDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mix format --check-formatted: %v\n%s", err, out)
	}
}

func TestEnumEmission(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.Status": {Kind: contract.Enum, Name: "Status", Base: "string", Values: []contract.EnumValue{{Name: "StatusDraft", Value: "draft"}, {Name: "StatusInProgress", Value: "in-progress"}}},
		"a.Level":  {Kind: contract.Enum, Name: "Level", Base: "int32", Values: []contract.EnumValue{{Name: "Low", Value: json.Number("1")}, {Name: "High", Value: json.Number("10")}}},
	}, Errors: map[string]*contract.ErrorDecl{}}
	out, err := Generator{}.Generate(doc, "bowline.ex")
	if err != nil {
		t.Fatal(err)
	}
	src := string(out)
	for _, want := range []string{
		"defmodule Bowline.Types.Status do",
		"@type t :: :draft | :in_progress",
		`def from_value("in-progress"), do: :in_progress`,
		`def to_value(:draft), do: "draft"`,
		"@type t :: 1 | 10",
		"def from_value(10), do: 10",
		`Read.mismatch("Status", "one of draft, in-progress", other)`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q in\n%s", want, src)
		}
	}
}

func TestGenericDecoderSynthesis(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.Page": {Kind: contract.Generic, Name: "Page", Params: []string{"T"}, Body: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{
			{Name: "items", Type: &contract.Type{Kind: contract.Array, Elem: &contract.Type{Kind: contract.Param, Name: "T"}}},
		}}},
		"a.User": {Kind: contract.Struct, Name: "User", Fields: []*contract.Field{{Name: "name", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}}}},
		"a.Report": {Kind: contract.Struct, Name: "Report", Fields: []*contract.Field{
			{Name: "users", Type: &contract.Type{Kind: contract.Ref, ID: "a.Page", Args: []*contract.Type{{Kind: contract.Ref, ID: "a.User"}}}},
		}},
	}, Errors: map[string]*contract.ErrorDecl{}}
	out, err := Generator{}.Generate(doc, "bowline.ex")
	if err != nil {
		t.Fatal(err)
	}
	src := string(out)
	for _, want := range []string{
		"@type t(param_t) :: %__MODULE__{",
		"def from_map(json, decode_t) do",
		`items: Read.list_value(Map.get(map, "items"), "items", decode_t)`,
		"def to_map(%__MODULE__{} = v, encode_t) do",
		"def validate(%__MODULE__{} = v, validate_t) do",
		`users: Types.Page.from_map(Map.get(map, "users"), &Types.User.from_map/1)`,
		"users: Types.Page.t(Types.User.t())",
		`Read.prefixed(["users"], Types.Page.validate(v.users, &Types.User.validate/1))`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q in\n%s", want, src)
		}
	}
}

func TestInlineModuleNamingAndReservedWords(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.Order": {Kind: contract.Struct, Name: "Order", Fields: []*contract.Field{
			{Name: "shipTo", Type: &contract.Type{Kind: contract.Struct, Fields: []*contract.Field{{Name: "city", Type: &contract.Type{Kind: contract.Primitive, Name: "string"}}}}},
			{Name: "end", Type: &contract.Type{Kind: contract.Primitive, Name: "bool"}},
			{Name: "createdAt", Type: &contract.Type{Kind: contract.Primitive, Name: "timestamp"}, Optional: true},
		}},
	}, Errors: map[string]*contract.ErrorDecl{}}
	out, err := Generator{Package: "shop.api"}.Generate(doc, "bowline.ex")
	if err != nil {
		t.Fatal(err)
	}
	src := string(out)
	for _, want := range []string{
		"defmodule Shop.Api.Types.Order.ShipTo do",
		"ship_to: Shop.Api.Types.Order.ShipTo.t()",
		`ship_to: Shop.Api.Types.Order.ShipTo.from_map(Map.get(map, "shipTo"))`,
		"end_: boolean()",
		`end_: Read.bool_value(Map.get(map, "end"), "end")`,
		`|> Encode.optional("createdAt", v.created_at, &Encode.timestamp/1)`,
		"created_at: DateTime.t() | nil",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q in\n%s", want, src)
		}
	}
}

func TestRootNameFromOutputPath(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{}, Errors: map[string]*contract.ErrorDecl{}}
	out, err := Generator{}.Generate(doc, "lib/ledger_api.ex")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "defmodule LedgerApi.Client do") {
		t.Fatalf("root module not derived from the file name:\n%s", out)
	}
	if err := os.WriteFile(filepath.Join(t.TempDir(), "x.ex"), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsAnUnrepresentableContract(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"example.com/app.Money": {Kind: contract.Kind("union"), Name: "Money"},
	}, Errors: map[string]*contract.ErrorDecl{}}
	_, err := Generator{}.Generate(doc, "bowline.out")
	if err == nil || !strings.Contains(err.Error(), "elixir") || !strings.Contains(err.Error(), "example.com/app.Money") {
		t.Fatalf("got %v", err)
	}
}
