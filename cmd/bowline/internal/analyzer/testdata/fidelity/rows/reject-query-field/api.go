package rejectqueryfield

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Nested struct {
	Deep string `json:"deep"`
}

type In struct {
	ID     int64  `json:"id" validate:"required"`
	Filter Nested `json:"filter"`
}

func get(ctx context.Context, in In) (In, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", get, bowline.Path("invoices/{id}")))
}
