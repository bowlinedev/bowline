package rejecttoolonstream

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type ID struct {
	ID int64 `json:"id"`
}

func Watch(ctx context.Context, in ID, stream *bowline.Stream[ID]) error { return nil }

func Attach(ctx context.Context, in ID, file *bowline.File) (ID, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Subscription("watch", Watch, bowline.Tool()),
		bowline.Upload("attach", Attach, bowline.Tool()),
	)
}
