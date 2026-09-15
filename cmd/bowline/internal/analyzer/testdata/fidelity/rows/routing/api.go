package routing

import (
	"context"

	"fidelity.test/rows/routing/sub"
	"github.com/bowlinedev/bowline"
)

const searchName = "search"

type ID struct {
	ID int64 `json:"id"`
}

type Item struct {
	Name string `json:"name"`
}

var admin = bowline.NewRouter(
	bowline.Mutation("purge", purge, bowline.Meta("auth", "admin"), bowline.Deprecated("use sub.remove")),
)

func get(ctx context.Context, in ID) (Item, error) { return Item{}, nil }

func search(ctx context.Context, in Item) (Item, error) { return Item{}, nil }

func purge(ctx context.Context, in ID) (struct{}, error) { return struct{}{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("get", get, bowline.Description("Get fetches an item.")),
		bowline.Query(searchName, search, bowline.Sensitive()),
		bowline.Mount("admin", admin),
		bowline.Mount("sub", sub.Router()),
	).Use(nil)
}
