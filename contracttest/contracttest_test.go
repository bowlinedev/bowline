package contracttest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
)

type invoice struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Total  string `json:"total"`
}

type renamed struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	Amount string `json:"amount"`
}

type getInput struct {
	ID int64 `json:"id" validate:"required"`
}

type InvoiceLocked struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

func (e InvoiceLocked) Error() string      { return fmt.Sprintf("invoice %d is %s", e.ID, e.Status) }
func (e InvoiceLocked) Code() bowline.Code { return bowline.FailedPrecondition }

func router(get func(ctx context.Context, in getInput) (invoice, error)) *bowline.Router {
	return bowline.NewRouter(bowline.Mount("invoices", bowline.NewRouter(
		bowline.Query("get", get),
		bowline.Mutation("void", func(ctx context.Context, in getInput) (invoice, error) {
			if in.ID == 4 {
				return invoice{}, InvoiceLocked{ID: 4, Status: "paid"}
			}
			return invoice{ID: in.ID, Status: "void", Total: "USD 0.00"}, nil
		}, bowline.Errors(InvoiceLocked{})),
	)))
}

func healthy(ctx context.Context, in getInput) (invoice, error) {
	if in.ID == 999 {
		return invoice{}, bowline.Errorf(bowline.NotFound, "invoice 999 not found")
	}
	return invoice{ID: in.ID, Status: "sent", Total: "USD 1500.00"}, nil
}

const contractJSON = `{
  "bowline": "1.2",
  "types": {
    "example.com/api.Invoice": {"kind": "struct", "name": "Invoice", "fields": [
      {"name": "id", "type": {"kind": "primitive", "name": "int64"}},
      {"name": "status", "type": {"kind": "primitive", "name": "string"}},
      {"name": "total", "type": {"kind": "primitive", "name": "string"}}
    ]},
    "example.com/api.GetInput": {"kind": "struct", "name": "GetInput", "fields": [
      {"name": "id", "type": {"kind": "primitive", "name": "int64"}, "rules": [{"rule": "required"}]}
    ]}
  },
  "errors": {
    "example.com/api.InvoiceLocked": {"name": "InvoiceLocked", "code": "FAILED_PRECONDITION", "fields": [
      {"name": "id", "type": {"kind": "primitive", "name": "int64"}},
      {"name": "status", "type": {"kind": "primitive", "name": "string"}}
    ]}
  },
  "procedures": [
    {"path": "invoices.get", "kind": "query", "method": "GET",
     "input": {"kind": "ref", "id": "example.com/api.GetInput"}, "output": {"kind": "ref", "id": "example.com/api.Invoice"}},
    {"path": "invoices.void", "kind": "mutation", "method": "POST",
     "input": {"kind": "ref", "id": "example.com/api.GetInput"}, "output": {"kind": "ref", "id": "example.com/api.Invoice"},
     "errors": ["example.com/api.InvoiceLocked"]}
  ]
}`

const consumerJSON = `{
  "bowline": "1.2",
  "consumer": "ledger-web",
  "provider": "ledger",
  "interactions": [
    {"procedure": "invoices.get", "method": "GET", "input": {"id": 3},
     "response": {"status": 200, "body": {"id": 3, "status": "sent", "total": "USD 1500.00"}}},
    {"procedure": "invoices.get", "method": "GET", "input": {"id": 999},
     "response": {"status": 404, "body": {"error": {"code": "NOT_FOUND", "message": "invoice 999 not found"}}}},
    {"procedure": "invoices.void", "method": "POST", "input": {"id": 4},
     "response": {"status": 412, "body": {"error": {"code": "FAILED_PRECONDITION", "message": "invoice 4 is paid", "type": "InvoiceLocked", "details": {"id": 4, "status": "paid"}}}}}
  ]
}`

func consumerDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ledger-web.json"), []byte(consumerJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type fakeT struct {
	testing.TB
	errors []string
	fatals []string
	logs   []string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeT) Fatalf(format string, args ...any) {
	f.fatals = append(f.fatals, fmt.Sprintf(format, args...))
}

func (f *fakeT) Logf(format string, args ...any) {
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}

func (f *fakeT) joined() string {
	return strings.Join(append(append([]string{}, f.errors...), f.fatals...), "\n")
}

func TestPassingRouterReportsNothing(t *testing.T) {
	dir := consumerDir(t)
	ft := &fakeT{TB: t}
	VerifyConsumers(ft, router(healthy), dir, WithContract([]byte(contractJSON)))
	if len(ft.errors) != 0 || len(ft.fatals) != 0 {
		t.Fatalf("unexpected failures:\n%s", ft.joined())
	}
	if !strings.Contains(strings.Join(ft.logs, "\n"), "verified 3 interactions from 1 consumers") {
		t.Fatalf("logs %v", ft.logs)
	}
	VerifyConsumers(t, router(healthy), dir, WithContract([]byte(contractJSON)))
}

func TestErroringProcedureNamesTheInteraction(t *testing.T) {
	dir := consumerDir(t)
	ft := &fakeT{TB: t}
	failing := router(func(ctx context.Context, in getInput) (invoice, error) {
		return invoice{}, bowline.Errorf(bowline.Internal, "database down")
	})
	VerifyConsumers(ft, failing, dir, WithContract([]byte(contractJSON)))
	joined := ft.joined()
	if !strings.Contains(joined, "consumer ledger-web: invoices.get: status 500, recorded 200") {
		t.Fatalf("missing status failure:\n%s", joined)
	}
	if !strings.Contains(joined, "consumer ledger-web: invoices.get: status 500, recorded 404") {
		t.Fatalf("missing second status failure:\n%s", joined)
	}
	if !strings.Contains(joined, "2 of 3 interactions broke") {
		t.Fatalf("missing summary:\n%s", joined)
	}
}

func TestShapeRuleFlagsFieldsMissingFromTheContract(t *testing.T) {
	dir := consumerDir(t)
	ft := &fakeT{TB: t}
	drifted := bowline.NewRouter(bowline.Mount("invoices", bowline.NewRouter(
		bowline.Query("get", func(ctx context.Context, in getInput) (renamed, error) {
			if in.ID == 999 {
				return renamed{}, bowline.Errorf(bowline.NotFound, "invoice 999 not found")
			}
			return renamed{ID: in.ID, Status: "sent", Amount: "USD 1500.00"}, nil
		}),
		bowline.Mutation("void", func(ctx context.Context, in getInput) (invoice, error) {
			return invoice{}, InvoiceLocked{ID: 4, Status: "paid"}
		}, bowline.Errors(InvoiceLocked{})),
	)))
	VerifyConsumers(ft, drifted, dir, WithContract([]byte(contractJSON)))
	joined := ft.joined()
	if !strings.Contains(joined, "consumer ledger-web: invoices.get → amount: field removed") {
		t.Fatalf("missing shape failure:\n%s", joined)
	}
	if !strings.Contains(joined, "1 of 3 interactions broke") {
		t.Fatalf("missing summary:\n%s", joined)
	}
}

func TestHeadersAndSetupReachEveryInteraction(t *testing.T) {
	dir := consumerDir(t)
	guarded := router(healthy).Use(func(next bowline.Next) bowline.Next {
		return func(ctx context.Context, in any) (any, error) {
			if bowline.CallFrom(ctx).Request.Header.Get("Authorization") != "Bearer dev" {
				return nil, bowline.Errorf(bowline.Unauthenticated, "token required")
			}
			return next(ctx, in)
		}
	})
	ft := &fakeT{TB: t}
	VerifyConsumers(ft, guarded, dir, WithContract([]byte(contractJSON)))
	if !strings.Contains(ft.joined(), "status 401, recorded 200") {
		t.Fatalf("expected the guard to reject without headers:\n%s", ft.joined())
	}
	setups := 0
	ft = &fakeT{TB: t}
	VerifyConsumers(ft, guarded, dir,
		WithContract([]byte(contractJSON)),
		WithHeaders(http.Header{"Authorization": {"Bearer dev"}}),
		WithSetup(func(testing.TB) { setups++ }),
	)
	if len(ft.errors) != 0 {
		t.Fatalf("unexpected failures with headers:\n%s", ft.joined())
	}
	if setups != 3 {
		t.Fatalf("setup ran %d times", setups)
	}
}

func TestRecordedInputMustStillBeAccepted(t *testing.T) {
	dir := consumerDir(t)
	stricter := strings.Replace(contractJSON, `{"name": "id", "type": {"kind": "primitive", "name": "int64"}, "rules": [{"rule": "required"}]}`, `{"name": "id", "type": {"kind": "primitive", "name": "int64"}, "rules": [{"rule": "required"}]}, {"name": "tenant", "type": {"kind": "primitive", "name": "string"}, "rules": [{"rule": "required"}]}`, 1)
	ft := &fakeT{TB: t}
	VerifyConsumers(ft, router(healthy), dir, WithContract([]byte(stricter)))
	joined := ft.joined()
	if !strings.Contains(joined, "consumer ledger-web: invoices.get → input.tenant: input no longer accepted: is required") {
		t.Fatalf("missing input failure:\n%s", joined)
	}
	if !strings.Contains(joined, "3 of 3 interactions broke") {
		t.Fatalf("missing summary:\n%s", joined)
	}
}

func TestHandlerOptionAndMissingContract(t *testing.T) {
	dir := consumerDir(t)
	wrapped := http.StripPrefix("/api", router(healthy).Handler())
	rewrite := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/api" + r.URL.Path
		wrapped.ServeHTTP(w, r)
	})
	ft := &fakeT{TB: t}
	VerifyConsumers(ft, nil, dir, WithHandler(rewrite))
	if len(ft.errors) != 0 || len(ft.fatals) != 0 {
		t.Fatalf("unexpected failures:\n%s", ft.joined())
	}
	if !strings.Contains(strings.Join(ft.logs, "\n"), "no contract given") {
		t.Fatalf("logs %v", ft.logs)
	}
	ft = &fakeT{TB: t}
	VerifyConsumers(ft, router(healthy), filepath.Join(dir, "missing"))
	if len(ft.fatals) != 1 || !strings.Contains(ft.fatals[0], "loading consumer contracts") {
		t.Fatalf("fatals %v", ft.fatals)
	}
	empty := t.TempDir()
	ft = &fakeT{TB: t}
	VerifyConsumers(ft, router(healthy), empty)
	if len(ft.fatals) != 1 || !strings.Contains(ft.fatals[0], "no consumer contracts") {
		t.Fatalf("fatals %v", ft.fatals)
	}
}
