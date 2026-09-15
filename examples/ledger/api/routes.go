package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

type API struct {
	store *ledger.Store
	log   *slog.Logger
}

func New(store *ledger.Store, log *slog.Logger) *API {
	return &API{store: store, log: log}
}

type HealthOutput struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

func Routes() *bowline.Router {
	a := New(ledger.NewStore(time.Now), slog.Default())
	return a.Router()
}

func (a *API) Router() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("health", a.health),
		bowline.Mount("invoices", a.invoices()),
		bowline.Mount("customers", a.customers()),
	).Use(a.logCalls)
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
