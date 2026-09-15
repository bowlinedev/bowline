package rejectinterface

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Bag struct {
	Any  any            `json:"any"`
	Meta map[string]any `json:"meta"`
	Err  error          `json:"err"`
}

func Get(ctx context.Context, in struct{}) (Bag, error) { return Bag{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
