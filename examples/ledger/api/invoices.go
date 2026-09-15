package api

import (
	"context"
	"errors"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

type GetInvoiceInput struct {
	ID int64 `json:"id" validate:"required"`
}

type ListInvoicesInput struct {
	Cursor string         `json:"cursor,omitempty"`
	Limit  int32          `json:"limit" validate:"min=1,max=100"`
	Status *ledger.Status `json:"status,omitempty"`
}

type CreateInvoiceInput struct {
	CustomerID int64         `json:"customerId" validate:"required"`
	Lines      []ledger.Line `json:"lines" validate:"required"`
	Note       *string       `json:"note,omitempty" validate:"max=500"`
}

type VoidInvoiceInput struct {
	ID int64 `json:"id" validate:"required"`
}

func (a *API) invoices() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("get", a.getInvoice, bowline.Description("Get returns one invoice by ID.")),
		bowline.Query("list", a.listInvoices),
		bowline.Mutation("create", a.createInvoice),
		bowline.Mutation("void", a.voidInvoice, bowline.Meta("auth", "admin")),
	)
}

func (a *API) getInvoice(ctx context.Context, in GetInvoiceInput) (ledger.Invoice, error) {
	inv, err := a.store.Invoice(in.ID)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.Invoice{}, bowline.Errorf(bowline.NotFound, "invoice %d not found", in.ID)
	}
	return inv, err
}

func (a *API) listInvoices(ctx context.Context, in ListInvoicesInput) (ledger.Page[ledger.Invoice], error) {
	items, next := a.store.ListInvoices(in.Cursor, int(in.Limit), in.Status)
	return ledger.Page[ledger.Invoice]{Items: items, NextCursor: next}, nil
}

func (a *API) createInvoice(ctx context.Context, in CreateInvoiceInput) (ledger.Invoice, error) {
	if _, err := a.store.Customer(in.CustomerID); errors.Is(err, ledger.ErrNotFound) {
		return ledger.Invoice{}, bowline.Errorf(bowline.FailedPrecondition, "customer %d does not exist", in.CustomerID)
	}
	return a.store.PutInvoice(ledger.Invoice{CustomerID: in.CustomerID, Status: ledger.StatusDraft, Lines: in.Lines, Note: in.Note}), nil
}

func (a *API) voidInvoice(ctx context.Context, in VoidInvoiceInput) (ledger.Invoice, error) {
	inv, err := a.store.Invoice(in.ID)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.Invoice{}, bowline.Errorf(bowline.NotFound, "invoice %d not found", in.ID)
	}
	if inv.Status == ledger.StatusPaid {
		return ledger.Invoice{}, bowline.Errorf(bowline.FailedPrecondition, "paid invoices cannot be voided")
	}
	inv.Status = ledger.StatusVoid
	return a.store.PutInvoice(inv), nil
}
