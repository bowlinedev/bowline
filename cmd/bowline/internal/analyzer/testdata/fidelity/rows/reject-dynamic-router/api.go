package rejectdynamicrouter

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type ID struct {
	ID int64 `json:"id"`
}

func get(ctx context.Context, in ID) (ID, error) { return in, nil }

func build(names []string) *bowline.Router {
	var items []bowline.Item
	for _, n := range names {
		items = append(items, bowline.Query(n, get))
	}
	return bowline.NewRouter(items...)
}

func Routes() *bowline.Router {
	return build([]string{"a", "b"})
}
