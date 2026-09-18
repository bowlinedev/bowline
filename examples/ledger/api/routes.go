package api

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

type API struct {
	store *ledger.Store
	log   *slog.Logger
	token string
}

func New(store *ledger.Store, log *slog.Logger, token string) *API {
	return &API{store: store, log: log, token: token}
}

type HealthOutput struct {
	OK      bool   `json:"ok" example:"true"`
	Version string `json:"version" example:"1.2.0"`
}

func Routes() *bowline.Router {
	a := New(ledger.NewStore(Clock()), slog.Default(), os.Getenv("LEDGER_TOKEN"))
	return a.Router()
}

func Clock() func() time.Time {
	fixed := os.Getenv("LEDGER_FIXED_TIME")
	if fixed == "" {
		return time.Now
	}
	at, err := time.Parse(time.RFC3339, fixed)
	if err != nil {
		panic("LEDGER_FIXED_TIME: " + err.Error())
	}
	return func() time.Time { return at }
}

func (a *API) Router() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("health", a.health, bowline.Public()),
		bowline.Mount("invoices", a.invoices()),
		bowline.Mount("customers", a.customers()),
	).Use(a.logCalls).Scheme("bearer", bowline.BearerAuth("opaque")).Secure("bearer")
}

func (a *API) health(ctx context.Context, _ struct{}) (HealthOutput, error) {
	return HealthOutput{OK: true, Version: bowline.Version}, nil
}

func (a *API) logCalls(next bowline.Next) bowline.Next {
	return func(ctx context.Context, in any) (any, error) {
		start := time.Now()
		out, err := next(ctx, in)
		call := bowline.CallFrom(ctx)
		a.log.InfoContext(ctx, "call", "procedure", call.Procedure.Path, "duration", time.Since(start), "error", err)
		return out, err
	}
}
