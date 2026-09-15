package sub

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type ID struct {
	ID int64 `json:"id"`
}

// Remove deletes an item by ID.
func Remove(ctx context.Context, in ID) (struct{}, error) { return struct{}{}, nil }

func Router() *bowline.Router {
	r := bowline.NewRouter(bowline.Mutation("remove", Remove, bowline.Use(nil)))
	return r
}
