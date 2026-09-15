package rejectwireasconflict

import (
	"context"
	"encoding/json"

	"github.com/bowlinedev/bowline"
)

type Money struct{ Cents int64 }

func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal("") }

var _ = bowline.WireAs[Money, string]()
var _ = bowline.WireAs[Money, int64]()

type Line struct {
	Price Money `json:"price"`
}

func Get(ctx context.Context, in struct{}) (Line, error) { return Line{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
