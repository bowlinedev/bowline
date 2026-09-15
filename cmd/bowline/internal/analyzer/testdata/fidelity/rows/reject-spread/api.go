package rejectspread

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type ID struct {
	ID int64 `json:"id"`
}

func get(ctx context.Context, in ID) (ID, error) { return in, nil }

var items = []bowline.Item{bowline.Query("get", get)}

func Routes() *bowline.Router {
	return bowline.NewRouter(items...)
}
