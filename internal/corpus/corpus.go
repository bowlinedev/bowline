package corpus

import "time"

type User struct {
	ID int64
}

type Page[T any] struct {
	Items []T
}

type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

type Alias = User

var Instances struct {
	A User
	B Page[User]
	C Page[int]
	D struct{}
	E time.Time
	F Pair[string, Page[User]]
	G Alias
	H int64
}
