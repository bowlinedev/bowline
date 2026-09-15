package embedded

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Audit struct {
	CreatedBy string `json:"createdBy"`
	Version   int32  `json:"version"`
}

type base struct {
	ID int64 `json:"id"`
}

type Named struct {
	Name string `json:"name"`
}

type Record struct {
	base
	Audit
	*Named  `json:"named"`
	Version int32  `json:"version"`
	Extra   string `json:"extra"`
}

func Get(ctx context.Context, in Audit) (Record, error) { return Record{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
