package goclient

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/analyzer"
	"github.com/bowlinedev/bowline/contract"
)

var update = flag.Bool("update", false, "rewrite golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func moduleEnv() []string {
	return append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
}

func writeModule(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	goMod := "module example.com/gen\n\ngo 1.24\n\nrequire github.com/bowlinedev/bowline v0.0.0\n\nreplace github.com/bowlinedev/bowline => " + repoRoot(t) + "\n"
	files["go.mod"] = goMod
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = moduleEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestGoldens(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join("..", "..", "analyzer", "testdata", "fidelity", "rows", "*", "expected.contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no contracts found; run the analyzer fidelity suite first")
	}
	for _, input := range inputs {
		row := filepath.Base(filepath.Dir(input))
		t.Run(row, func(t *testing.T) {
			data, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := contract.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Generator{}.Generate(doc, "internal/apiclient/client.go")
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", row+".golden.go")
			if *update {
				os.MkdirAll("testdata", 0o755)
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v\n%s", err, got)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch; run go test ./internal/gen/goclient -update after review:\n%s", got)
			}
			if testing.Short() {
				return
			}
			dir := t.TempDir()
			writeModule(t, dir, map[string]string{"apiclient/client.go": string(got)})
			run(t, dir, "vet", "./...")
		})
	}
}

func TestPackageName(t *testing.T) {
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{}, Errors: map[string]*contract.ErrorDecl{}}
	cases := map[string]string{
		"client.go":                 "apiclient",
		"internal/ledger-api/c.go":  "ledgerapi",
		"pkg/Users/users.go":        "users",
		"internal/apiclient/gen.go": "apiclient",
	}
	for out, want := range cases {
		got, err := Generator{}.Generate(doc, out)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(got, []byte("package "+want+"\n")) {
			t.Fatalf("%s: package %q not found in\n%s", out, want, got)
		}
	}
	got, _ := Generator{}.WithPackage("custom").Generate(doc, "x/y.go")
	if !bytes.Contains(got, []byte("package custom\n")) {
		t.Fatalf("explicit package ignored:\n%s", got)
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{"id": "ID", "createdAt": "CreatedAt", "user_id": "UserID", "1st": "F1st", "html-url": "HTMLURL", "ok": "Ok"} {
		if got := fieldName(in); got != want {
			t.Errorf("fieldName(%q) = %q, want %q", in, got, want)
		}
	}
	doc := &contract.Document{Bowline: contract.Version, Types: map[string]*contract.TypeDecl{
		"a.Client": {Kind: contract.Struct, Name: "Client"},
		"a.item":   {Kind: contract.Struct, Name: "item"},
		"b.Item":   {Kind: contract.Struct, Name: "Item"},
	}, Errors: map[string]*contract.ErrorDecl{}}
	names := assignNames(doc)
	if names["a.Client"] != "ClientType" || names["a.item"] != "A_item" || names["b.Item"] != "B_Item" {
		t.Fatalf("%v", names)
	}
}

const liveAPI = `package api

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/bowlinedev/bowline"
)

type GetInput struct {
	ID int64 ` + "`json:\"id\"`" + `
}

// Item is one stored thing.
type Item struct {
	ID        int64     ` + "`json:\"id\"`" + `
	Name      string    ` + "`json:\"name\"`" + `
	CreatedAt time.Time ` + "`json:\"createdAt\"`" + `
	Big       int64     ` + "`json:\"big,string\"`" + `
	Tags      []string  ` + "`json:\"tags\"`" + `
	Note      *string   ` + "`json:\"note,omitempty\"`" + `
}

type Page[T any] struct {
	Items []T    ` + "`json:\"items\"`" + `
	Next  string ` + "`json:\"next,omitempty\"`" + `
}

type ListInput struct {
	Limit int32 ` + "`json:\"limit\" validate:\"min=1\"`" + `
}

type Locked struct {
	ID     int64  ` + "`json:\"id\"`" + `
	Reason string ` + "`json:\"reason\"`" + `
}

func (e Locked) Error() string       { return fmt.Sprintf("item %d is locked", e.ID) }
func (e Locked) Code() bowline.Code  { return bowline.FailedPrecondition }

type TicksInput struct {
	Count int32 ` + "`json:\"count\"`" + `
}

type Tick struct {
	N int32 ` + "`json:\"n\"`" + `
}

type AttachInput struct {
	Label string ` + "`json:\"label\"`" + `
}

type Attachment struct {
	Label string ` + "`json:\"label\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Size  int64  ` + "`json:\"size\"`" + `
}

var epoch = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func get(ctx context.Context, in GetInput) (Item, error) {
	if in.ID == 404 {
		return Item{}, bowline.Errorf(bowline.NotFound, "item %d not found", in.ID)
	}
	return Item{ID: in.ID, Name: "thing", CreatedAt: epoch, Big: 9007199254740993}, nil
}

func list(ctx context.Context, in ListInput) (Page[Item], error) {
	return Page[Item]{Items: []Item{{ID: 1, Name: "a", CreatedAt: epoch}, {ID: 2, Name: "b", CreatedAt: epoch}}, Next: "3"}, nil
}

func remove(ctx context.Context, in GetInput) (struct{}, error) {
	if in.ID == 7 {
		return struct{}{}, Locked{ID: in.ID, Reason: "paid"}
	}
	return struct{}{}, nil
}

func ticks(ctx context.Context, in TicksInput, stream *bowline.Stream[Tick]) error {
	for i := int32(1); i <= in.Count; i++ {
		if err := stream.Send(Tick{N: i}); err != nil {
			return err
		}
	}
	return nil
}

func attach(ctx context.Context, in AttachInput, file *bowline.File) (Attachment, error) {
	n, err := io.Copy(io.Discard, file)
	if err != nil {
		return Attachment{}, err
	}
	return Attachment{Label: in.Label, Name: file.Name, Size: n}, nil
}

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Mount("items", bowline.NewRouter(
			bowline.Query("get", get),
			bowline.Query("list", list),
			bowline.Mutation("remove", remove, bowline.Errors(Locked{})),
		)),
		bowline.Subscription("ticks", ticks),
		bowline.Upload("attach", attach),
	)
}
`

const liveTest = `package run

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/gen/api"
	"example.com/gen/client"
	"github.com/bowlinedev/bowline"
)

func TestLive(t *testing.T) {
	seen := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case seen <- r.Header.Get("Authorization"):
		default:
		}
		api.Routes().Handler().ServeHTTP(w, r)
	}))
	defer srv.Close()
	c := client.New(srv.URL+"/", client.WithHeaders(func(ctx context.Context) (http.Header, error) {
		return http.Header{"Authorization": {"Bearer t"}}, nil
	}))
	ctx := context.Background()

	item, err := c.Items.Get(ctx, client.GetInput{ID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 3 || item.Big != 9007199254740993 || !item.CreatedAt.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) || item.Tags == nil || item.Note != nil {
		t.Fatalf("item %+v", item)
	}
	if got := <-seen; got != "Bearer t" {
		t.Fatalf("authorization header %q", got)
	}

	_, err = c.Items.Get(ctx, client.GetInput{ID: 404})
	var be *bowline.Error
	if !errors.As(err, &be) || be.Code != bowline.NotFound || !strings.Contains(be.Message, "404") {
		t.Fatalf("error %v", err)
	}

	page, err := c.Items.List(ctx, client.ListInput{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.Items[1].Name != "b" || page.Next == nil || *page.Next != "3" {
		t.Fatalf("page %+v %v", page, err)
	}
	_, err = c.Items.List(ctx, client.ListInput{Limit: 0})
	if !errors.As(err, &be) || be.Code != bowline.InvalidArgument || len(be.Issues) != 1 || be.Issues[0].Rule != "min" {
		t.Fatalf("validation %v", err)
	}

	if err := c.Items.Remove(ctx, client.GetInput{ID: 1}); err != nil {
		t.Fatal(err)
	}
	err = c.Items.Remove(ctx, client.GetInput{ID: 7})
	if !errors.As(err, &be) || be.Code != bowline.FailedPrecondition || client.VariantOf(err) != "Locked" {
		t.Fatalf("variant %v", err)
	}
	details, ok := client.DetailsAs[client.Locked](err)
	if !ok || details.ID != 7 || details.Reason != "paid" {
		t.Fatalf("details %+v %v", details, ok)
	}

	seq, err := c.Ticks(ctx, client.TicksInput{Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	var ns []int32
	for tick, err := range seq {
		if err != nil {
			t.Fatal(err)
		}
		ns = append(ns, tick.N)
	}
	if len(ns) != 3 || ns[2] != 3 {
		t.Fatalf("ticks %v", ns)
	}

	att, err := c.Attach(ctx, client.AttachInput{Label: "receipt"}, "r.bin", strings.NewReader("hello"))
	if err != nil || att.Size != 5 || att.Name != "r.bin" || att.Label != "receipt" {
		t.Fatalf("attach %+v %v", att, err)
	}

	srv.Close()
	_, err = c.Items.Get(ctx, client.GetInput{ID: 1})
	if !errors.As(err, &be) || be.Code != bowline.Unavailable {
		t.Fatalf("down %v", err)
	}
}
`

func TestLiveClient(t *testing.T) {
	if testing.Short() {
		t.Skip("live client test builds a module")
	}
	dir := t.TempDir()
	writeModule(t, dir, map[string]string{"api/api.go": liveAPI})
	prog, err := analyzer.Load(dir, moduleEnv(), "./api/...")
	if err != nil {
		t.Fatal(err)
	}
	doc, diags := analyzer.Analyze(prog, "./api.Routes")
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	generated, err := Generator{Package: "client"}.Generate(doc, "client/client.go")
	if err != nil {
		t.Fatal(err)
	}
	writeModule(t, dir, map[string]string{"client/client.go": string(generated), "run/run_test.go": liveTest})
	run(t, dir, "test", "./run/")
}
