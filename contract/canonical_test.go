package contract

import (
	"reflect"
	"testing"
	"time"
)

type page[T any] struct {
	Items []T
}

type user struct{}

func TestGoTypeName(t *testing.T) {
	cases := []struct {
		typ  reflect.Type
		want string
	}{
		{reflect.TypeFor[user](), "github.com/bowlinedev/bowline/contract.user"},
		{reflect.TypeFor[page[user]](), "github.com/bowlinedev/bowline/contract.page[github.com/bowlinedev/bowline/contract.user]"},
		{reflect.TypeFor[page[int]](), "github.com/bowlinedev/bowline/contract.page[int]"},
		{reflect.TypeFor[time.Time](), "time.Time"},
		{reflect.TypeFor[struct{}](), "struct{}"},
	}
	for _, tc := range cases {
		if got := GoTypeName(tc.typ); got != tc.want {
			t.Errorf("%v: got %q, want %q", tc.typ, got, tc.want)
		}
	}
}
