package wireas

import (
	"context"
	"encoding/json"

	"github.com/bowlinedev/bowline"
)

type Money struct {
	Cents    int64
	Currency string
}

func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal("") }

var _ = bowline.WireAs[Money, string]()

type Point struct {
	X, Y float64
}

func (p Point) MarshalJSON() ([]byte, error) { return json.Marshal([]float64{p.X, p.Y}) }

var _ = bowline.WireAs[Point, [2]float64]()

type Envelope struct {
	Kind string `json:"kind"`
}

type Wrapped struct {
	Inner int32
}

func (w Wrapped) MarshalJSON() ([]byte, error) { return json.Marshal(Envelope{}) }

var _ = bowline.WireAs[Wrapped, Envelope]()

type Line struct {
	Price  Money   `json:"price"`
	Origin Point   `json:"origin"`
	Meta   Wrapped `json:"meta"`
	Prices []Money `json:"prices"`
}

func Get(ctx context.Context, in struct{}) (Line, error) { return Line{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
