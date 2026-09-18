package rest

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type ListInput struct {
	Limit  int32    `json:"limit" validate:"min=1,max=100"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
	Deep   bool     `json:"deep"`
}

type IDInput struct {
	ID int64 `json:"id" validate:"required"`
}

type ReplaceInput struct {
	ID    int64  `json:"id" validate:"required"`
	Title string `json:"title" validate:"required"`
}

type RemoveInput struct {
	ID    int64 `json:"id" validate:"required"`
	Force bool  `json:"force"`
}

type Invoice struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

func list(ctx context.Context, in ListInput) (Invoice, error)       { return Invoice{}, nil }
func get(ctx context.Context, in IDInput) (Invoice, error)          { return Invoice{ID: in.ID}, nil }
func replace(ctx context.Context, in ReplaceInput) (Invoice, error) { return Invoice{ID: in.ID}, nil }
func remove(ctx context.Context, in RemoveInput) (Invoice, error)   { return Invoice{ID: in.ID}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Mount("invoices", bowline.NewRouter(
			bowline.Query("list", list, bowline.Path("invoices")),
			bowline.Query("get", get, bowline.Path("invoices/{id}")),
			bowline.Mutation("replace", replace, bowline.Path("invoices/{id}"), bowline.Method("PUT")),
			bowline.Mutation("remove", remove, bowline.Path("invoices/{id}"), bowline.Method("DELETE")),
		)),
	)
}
