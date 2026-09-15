package recursive

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Node struct {
	Name     string `json:"name"`
	Children []Node `json:"children"`
	Parent   *Node  `json:"parent,omitempty"`
}

func Get(ctx context.Context, in Node) (Node, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
