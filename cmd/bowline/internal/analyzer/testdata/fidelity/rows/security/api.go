package security

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type In struct {
	ID int64 `json:"id"`
}

func get(ctx context.Context, in In) (In, error) { return in, nil }

func admin(ctx context.Context, in In) (In, error) { return in, nil }

func ping(ctx context.Context, in In) (In, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("ping", ping, bowline.Public()),
		bowline.Query("get", get),
		bowline.Mutation("admin", admin, bowline.Requires("adminKey")),
	).Scheme("bearer", bowline.BearerAuth("JWT")).Scheme("adminKey", bowline.APIKeyAuth("header", "X-Admin-Key")).Secure("bearer")
}
