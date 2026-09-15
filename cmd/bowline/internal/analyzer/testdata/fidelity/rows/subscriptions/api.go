package subscriptions

import (
	"context"
	"time"

	"github.com/bowlinedev/bowline"
)

type WatchInput struct {
	Limit int32 `json:"limit" validate:"min=1,max=100"`
}

type Change struct {
	ID   int64     `json:"id"`
	At   time.Time `json:"at"`
	Tags []string  `json:"tags"`
}

type Gone struct {
	ID int64 `json:"id"`
}

func (e Gone) Error() string { return "gone" }

func (e Gone) Code() bowline.Code { return bowline.NotFound }

// Watch streams every change.
func Watch(ctx context.Context, in WatchInput, stream *bowline.Stream[Change]) error { return nil }

func Secret(ctx context.Context, in WatchInput, stream *bowline.Stream[Change]) error { return nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Subscription("watch", Watch, bowline.Errors(Gone{})),
		bowline.Subscription("secret", Secret, bowline.Sensitive()),
	)
}
