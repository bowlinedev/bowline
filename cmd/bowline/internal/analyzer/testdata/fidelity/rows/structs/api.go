package structs

import (
	"context"

	"github.com/bowlinedev/bowline"
)

// Address is where mail goes.
type Address struct {
	// Street line.
	Street string `json:"street"`
	City   string `json:"city" validate:"required"`
}

type Person struct {
	Name    string  `json:"name" validate:"required,min=1,max=80"`
	Age     int32   `json:"age" validate:"min=0,max=150"`
	Email   string  `json:"email" validate:"email"`
	Home    Address `json:"home"`
	Ignored string  `json:"-"`
	hidden  string
	NoTag   string
	Inline  struct {
		X int32 `json:"x"`
	} `json:"inline"`
}

func Get(ctx context.Context, in Address) (Person, error) { return Person{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
