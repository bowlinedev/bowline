package rejecttoolcollision

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type ID struct {
	ID int64 `json:"id"`
}

func Get(ctx context.Context, in ID) (ID, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Mount("a", bowline.NewRouter(bowline.Query("b_c", Get, bowline.Tool()))),
		bowline.Mount("a_b", bowline.NewRouter(bowline.Query("c", Get, bowline.Tool()))),
	)
}
