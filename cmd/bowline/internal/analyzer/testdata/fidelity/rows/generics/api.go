package generics

import (
	"context"

	"github.com/bowlinedev/bowline"
)

// Page is one page of results.
type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next,omitempty"`
}

type Pair[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

type Tree[T any] struct {
	Value    T         `json:"value"`
	Children []Tree[T] `json:"children"`
}

type Number interface {
	~int32 | ~int64 | ~float64
}

type Range[N Number] struct {
	Low  N `json:"low"`
	High N `json:"high"`
}

type User struct {
	Name string `json:"name"`
}

type Report struct {
	Users  Page[User]           `json:"users"`
	Nested Page[Page[User]]     `json:"nested"`
	Pairs  []Pair[string, User] `json:"pairs"`
	Tree   Tree[User]           `json:"tree"`
	Ints   Range[int64]         `json:"ints"`
	Floats Range[float64]       `json:"floats"`
	Scores Page[Range[int32]]   `json:"scores"`
}

func Get(ctx context.Context, in Page[User]) (Report, error) { return Report{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
