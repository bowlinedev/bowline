package idempotent

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type CreateInput struct {
	Name string `json:"name" validate:"required"`
}

type Created struct {
	ID int64 `json:"id"`
}

func Create(ctx context.Context, in CreateInput) (Created, error) { return Created{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Mutation("create", Create, bowline.Idempotent()))
}
