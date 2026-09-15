package rejectmarshaler

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Money struct {
	Cents int64
}

func (m Money) MarshalJSON() ([]byte, error) { return []byte(`"0"`), nil }

type Price struct {
	Amount Money `json:"amount"`
}

func Get(ctx context.Context, in struct{}) (Price, error) { return Price{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
