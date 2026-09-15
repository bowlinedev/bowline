package collections

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Key string

type Cell struct {
	V int32 `json:"v"`
}

type Grid struct {
	Rows  [][]Cell         `json:"rows"`
	Fixed [3]int32         `json:"fixed"`
	ByKey map[Key]Cell     `json:"byKey"`
	ByInt map[int64]string `json:"byInt"`
	Names []string         `json:"names,omitempty" validate:"max=10"`
	Blob  []byte           `json:"blob"`
	IDs   IDList           `json:"ids"`
}

type IDList []int64

func Get(ctx context.Context, in Cell) (Grid, error) { return Grid{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
