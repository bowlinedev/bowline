package aliases

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Real struct {
	N int32 `json:"n"`
}

type Alias = Real

type Holder struct {
	A Alias   `json:"a"`
	B []Alias `json:"b"`
}

func Get(ctx context.Context, in Alias) (Holder, error) { return Holder{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
