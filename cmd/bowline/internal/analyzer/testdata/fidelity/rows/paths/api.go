package paths

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type GetInput struct {
	ID int64 `json:"id" validate:"required"`
}

type ListInput struct {
	Limit int32 `json:"limit"`
}

type CreateInput struct {
	Total string `json:"total" validate:"required"`
}

type LineInput struct {
	InvoiceID int64  `json:"invoiceId" validate:"required"`
	LineID    string `json:"lineId" validate:"required"`
}

type Invoice struct {
	ID    int64  `json:"id"`
	Total string `json:"total"`
}

func get(ctx context.Context, in GetInput) (Invoice, error)   { return Invoice{ID: in.ID}, nil }
func list(ctx context.Context, in ListInput) (Invoice, error) { return Invoice{}, nil }
func create(ctx context.Context, in CreateInput) (Invoice, error) {
	return Invoice{Total: in.Total}, nil
}
func line(ctx context.Context, in LineInput) (Invoice, error) { return Invoice{ID: in.InvoiceID}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Mount("invoices", bowline.NewRouter(
			bowline.Query("get", get, bowline.Path("invoices/{id}")),
			bowline.Query("list", list, bowline.Path("invoices")),
			bowline.Mutation("create", create, bowline.Path("invoices")),
			bowline.Query("line", line, bowline.Path("invoices/{invoiceId}/lines/{lineId}")),
		)),
	)
}
