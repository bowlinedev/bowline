package pointers

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Tag struct {
	Label string `json:"label"`
}

type Doc struct {
	Title    *string         `json:"title"`
	Subtitle *string         `json:"subtitle,omitempty"`
	Primary  *Tag            `json:"primary"`
	Tags     []*Tag          `json:"tags"`
	ByName   map[string]*Tag `json:"byName"`
	Count    *int64          `json:"count,omitzero"`
}

func Get(ctx context.Context, in Tag) (Doc, error) { return Doc{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
