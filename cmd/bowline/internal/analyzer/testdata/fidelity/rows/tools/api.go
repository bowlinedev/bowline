package tools

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type ID struct {
	ID int64 `json:"id"`
}

type Item struct {
	Name string `json:"name"`
}

const billing = "billing"

// Get returns one item.
func Get(ctx context.Context, in ID) (Item, error) { return Item{}, nil }

func Search(ctx context.Context, in Item) (Item, error) { return Item{}, nil }

func Remove(ctx context.Context, in ID) (struct{}, error) { return struct{}{}, nil }

func Hidden(ctx context.Context, in ID) (Item, error) { return Item{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("get", Get, bowline.Tool(bowline.Scope(billing))),
		bowline.Query("search", Search, bowline.Sensitive(), bowline.Tool(bowline.Scope("crm", billing))),
		bowline.Mutation("remove", Remove, bowline.Tool(bowline.Scope(billing), bowline.Destructive())),
		bowline.Query("hidden", Hidden),
	)
}
