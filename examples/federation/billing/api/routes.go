package api

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/federation/billing/billing"
	"github.com/bowlinedev/bowline/examples/ledger/ledgerclient"
)

type API struct {
	store  *billing.Store
	ledger *ledgerclient.Client
	log    *slog.Logger
}

func New(store *billing.Store, ledger *ledgerclient.Client, log *slog.Logger) *API {
	return &API{store: store, ledger: ledger, log: log}
}

type CreateChargeInput struct {
	InvoiceID int64 `json:"invoiceId" validate:"required" example:"3"`
}

type ListChargesInput struct {
	Limit int32 `json:"limit" validate:"min=1,max=100" example:"20"`
}

type SettleChargeInput struct {
	ID int64 `json:"id" validate:"required"`
}

type HealthOutput struct {
	OK      bool   `json:"ok"`
	Version string `json:"version" example:"0.6.0"`
}

func Routes() *bowline.Router {
	url := os.Getenv("LEDGER_URL")
	if url == "" {
		url = "http://localhost:8080/api"
	}
	return New(billing.NewStore(Clock()), LedgerClient(url, os.Getenv("BILLING_SIGNING_KEY"), []byte(os.Getenv("BILLING_SIGNING_SECRET"))), slog.Default()).Router()
}

func Clock() func() time.Time {
	fixed := os.Getenv("BILLING_FIXED_TIME")
	if fixed == "" {
		return time.Now
	}
	at, err := time.Parse(time.RFC3339, fixed)
	if err != nil {
		panic("BILLING_FIXED_TIME: " + err.Error())
	}
	return func() time.Time { return at }
}

func (a *API) Router() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("health", a.health),
		bowline.Mount("charges", bowline.NewRouter(
			bowline.Query("list", a.list, bowline.Description("List returns every charge, newest last.")),
			bowline.Mutation("create", a.create, bowline.Description("Create charges an invoice read from the ledger."), bowline.Errors(UnknownInvoice{}), bowline.Idempotent()),
			bowline.Mutation("settle", a.settle),
		)),
	)
}

func (a *API) health(ctx context.Context, _ struct{}) (HealthOutput, error) {
	return HealthOutput{OK: true, Version: bowline.Version}, nil
}

func (a *API) list(ctx context.Context, in ListChargesInput) (billing.Page[billing.Charge], error) {
	charges := a.store.List()
	if int(in.Limit) < len(charges) {
		charges = charges[:in.Limit]
	}
	return billing.Page[billing.Charge]{Items: charges}, nil
}

func (a *API) create(ctx context.Context, in CreateChargeInput) (billing.Charge, error) {
	invoice, err := a.ledger.Invoices.Get(ctx, ledgerclient.GetInvoiceInput{ID: in.InvoiceID})
	if err != nil {
		var remote *bowline.Error
		if errors.As(err, &remote) && remote.Code == bowline.NotFound {
			return billing.Charge{}, UnknownInvoice(in)
		}
		return billing.Charge{}, err
	}
	return a.store.Create(invoice.ID, invoice.Total), nil
}

func (a *API) settle(ctx context.Context, in SettleChargeInput) (billing.Charge, error) {
	charge, ok := a.store.Settle(in.ID)
	if !ok {
		return billing.Charge{}, bowline.Errorf(bowline.NotFound, "charge %d not found", in.ID)
	}
	return charge, nil
}
