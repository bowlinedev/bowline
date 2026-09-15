package rejectshapes

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Weird struct {
	PP    **int32          `json:"pp"`
	C     chan int         `json:"c"`
	F     func()           `json:"f"`
	Z     complex128       `json:"z"`
	U     uintptr          `json:"u"`
	K     map[struct{}]int `json:"k"`
	S     int32            `json:"s,string"`
	Bad   string           `json:"bad" validate:"gte=1"`
	Wrong int32            `json:"wrong" validate:"email"`
}

func Get(ctx context.Context, in struct{}) (Weird, error) { return Weird{}, nil }

func Anon(ctx context.Context, in struct{ X int32 }) (Weird, error) { return Weird{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get), bowline.Query("anon", Anon))
}
