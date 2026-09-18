package rejectpathbadtype

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type In struct {
	When float64 `json:"when"`
}

func get(ctx context.Context, in In) (In, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", get, bowline.Path("at/{when}")))
}
