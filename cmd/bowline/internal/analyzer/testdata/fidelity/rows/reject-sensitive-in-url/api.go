package rejectsensitiveinurl

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type In struct {
	Token string `json:"token"`
}

func lookup(ctx context.Context, in In) (In, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("lookup", lookup, bowline.Sensitive(), bowline.Path("lookup/{token}"), bowline.Method("GET")))
}
