package rejectcomputedname

import (
	"context"
	"strings"

	"github.com/bowlinedev/bowline"
)

type ID struct {
	ID int64 `json:"id"`
}

func get(ctx context.Context, in ID) (ID, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query(strings.ToLower("GET"), get))
}
