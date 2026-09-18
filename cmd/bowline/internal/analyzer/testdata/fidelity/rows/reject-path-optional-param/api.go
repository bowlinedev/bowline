package rejectpathoptionalparam

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type In struct {
	ID int64 `json:"id,omitempty"`
}

func get(ctx context.Context, in In) (In, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", get, bowline.Path("invoices/{id}")))
}
