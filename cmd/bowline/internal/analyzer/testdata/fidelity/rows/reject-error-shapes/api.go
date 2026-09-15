package rejecterrorshapes

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Computed struct {
	N int32 `json:"n"`
}

func (e Computed) Error() string { return "computed" }

func (e Computed) Code() bowline.Code {
	if e.N > 0 {
		return bowline.NotFound
	}
	return bowline.Internal
}

type Hidden struct {
	secret string
}

func (e Hidden) Error() string { return e.secret }

func (e Hidden) Code() bowline.Code { return bowline.Internal }

type Plain string

func (e Plain) Error() string { return string(e) }

func (e Plain) Code() bowline.Code { return bowline.Unknown }

type ID struct {
	ID int64 `json:"id"`
}

func Do(ctx context.Context, in ID) (ID, error) { return in, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Mutation("do", Do, bowline.Errors(Computed{}, Hidden{}, Plain("x"))))
}
