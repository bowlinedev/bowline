package bowline

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type getInput struct {
	ID int64 `json:"id"`
}

type user struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func getUser(ctx context.Context, in getInput) (user, error) {
	return user{ID: in.ID, Name: "ada"}, nil
}

func createUser(ctx context.Context, in user) (user, error) {
	return in, nil
}

func health(ctx context.Context, _ struct{}) (struct{}, error) {
	return struct{}{}, nil
}

func TestProceduresAreFlattenedWithDottedPaths(t *testing.T) {
	users := NewRouter(
		Query("get", getUser, Description("Get returns one user.")),
		Mutation("create", createUser, Sensitive(), Meta("auth", "required")),
	)
	root := NewRouter(
		Mount("users", users),
		Query("health", health),
	)
	procs := root.Procedures()
	got := map[string]Procedure{}
	for _, p := range procs {
		got[p.Path] = p
	}
	if len(got) != 3 {
		t.Fatalf("got %d procedures, want 3", len(got))
	}
	if got["users.get"].Kind != KindQuery || got["users.get"].Method() != "GET" || got["users.get"].Description != "Get returns one user." {
		t.Fatalf("users.get wrong: %+v", got["users.get"])
	}
	if got["users.create"].Kind != KindMutation || got["users.create"].Method() != "POST" || got["users.create"].Meta["auth"] != "required" {
		t.Fatalf("users.create wrong: %+v", got["users.create"])
	}
	if got["users.get"].In != reflect.TypeFor[getInput]() || got["users.get"].Out != reflect.TypeFor[user]() {
		t.Fatal("reflect types not recorded")
	}
	if got["health"].In != reflect.TypeFor[struct{}]() {
		t.Fatal("struct{} input not accepted")
	}
}

func TestSensitiveQueryUsesPost(t *testing.T) {
	r := NewRouter(Query("search", getUser, Sensitive()))
	if m := r.Procedures()[0].Method(); m != "POST" {
		t.Fatalf("got %s, want POST", m)
	}
}

func TestMiddlewareOrderIsParentChildProcedure(t *testing.T) {
	var order []string
	tag := func(name string) Middleware {
		return func(next Next) Next {
			return func(ctx context.Context, in any) (any, error) {
				order = append(order, name)
				return next(ctx, in)
			}
		}
	}
	child := NewRouter(Query("get", getUser, Use(tag("proc")))).Use(tag("child"))
	root := NewRouter(Mount("users", child)).Use(tag("parent"))
	rt := root.routes()[0]
	if _, err := rt.next(context.Background(), getInput{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "parent,child,proc" {
		t.Fatalf("order %v", order)
	}
}

func TestMiddlewareSeesInputByValue(t *testing.T) {
	var seen any
	capture := func(next Next) Next {
		return func(ctx context.Context, in any) (any, error) {
			seen = in
			return next(ctx, in)
		}
	}
	r := NewRouter(Query("get", getUser, Use(capture)))
	if _, err := r.routes()[0].next(context.Background(), getInput{ID: 9}); err != nil {
		t.Fatal(err)
	}
	if _, ok := seen.(getInput); !ok {
		t.Fatalf("middleware saw %T, want getInput", seen)
	}
}

func TestNewRouterPanicsOnBadNames(t *testing.T) {
	cases := map[string]func(){
		"duplicate":   func() { NewRouter(Query("a", getUser), Query("a", getUser)) },
		"mount clash": func() { NewRouter(Query("a", getUser), Mount("a", NewRouter())) },
		"empty":       func() { NewRouter(Query("", getUser)) },
		"dot":         func() { NewRouter(Query("a.b", getUser)) },
		"slash":       func() { NewRouter(Query("a/b", getUser)) },
		"nil router":  func() { NewRouter(Mount("x", nil)) },
		"nil fn":      func() { NewRouter(Query[getInput, user]("a", nil)) },
		"anonymous input": func() {
			NewRouter(Query("a", func(ctx context.Context, in struct{ X int }) (user, error) { return user{}, nil }))
		},
		"anonymous output": func() {
			NewRouter(Query("a", func(ctx context.Context, in getInput) (struct{ X int }, error) { return struct{ X int }{}, nil }))
		},
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			fn()
		})
	}
}

func TestCallFromContext(t *testing.T) {
	if CallFrom(context.Background()) != nil {
		t.Fatal("expected nil without a call")
	}
	ctx := withCall(context.Background(), &Call{Procedure: Procedure{Path: "x"}})
	if CallFrom(ctx).Procedure.Path != "x" {
		t.Fatal("call not stored")
	}
}
