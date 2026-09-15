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

type WatchInput struct {
	Status *ledger.Status `json:"status,omitempty"`
}

func (a *API) invoices() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("get", a.getInvoice, bowline.Description("Get returns one invoice by ID.")),
		bowline.Query("list", a.listInvoices),
		bowline.Mutation("create", a.createInvoice, bowline.Idempotent()),
		bowline.Mutation("void", a.voidInvoice, bowline.Meta("auth", "admin"), bowline.Errors(InvoiceLocked{})),
		bowline.Subscription("watch", a.watchInvoices, bowline.Description("Watch streams every invoice change.")),
		bowline.Upload("attach", a.attach, bowline.Description("Attach stores a file against an invoice.")),
		bowline.Query("attachments", a.listAttachments),
	)
}

func (a *API) watchInvoices(ctx context.Context, in WatchInput, stream *bowline.Stream[ledger.Invoice]) error {
	changes, stop := a.store.Watch()
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case inv, ok := <-changes:
			if !ok {
				return nil
			}
			if in.Status != nil && inv.Status != *in.Status {
				continue
			}
			if err := stream.Send(inv); err != nil {
				return err
			}
		}
	}
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
	if inv.Status == ledger.StatusPaid || inv.Status == ledger.StatusVoid {
		return ledger.Invoice{}, InvoiceLocked{ID: inv.ID, Status: inv.Status}
	}
	inv.Status = ledger.StatusVoid
	return a.store.PutInvoice(inv), nil
}
