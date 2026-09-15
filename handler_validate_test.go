package bowline

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

type signupInput struct {
	Email string   `json:"email" validate:"required,email"`
	Tags  []string `json:"tags" validate:"max=2"`
}

type signupOutput struct {
	ID    int64    `json:"id"`
	Tags  []string `json:"tags"`
	Roles []string `json:"roles,omitempty"`
}

func signup(ctx context.Context, in signupInput) (signupOutput, error) {
	if in.Email == "huge@example.com" {
		return signupOutput{ID: 1 << 60}, nil
	}
	return signupOutput{ID: 1}, nil
}

func TestValidationIssuesAreReturned(t *testing.T) {
	h := NewRouter(Mutation("signup", signup)).Handler()
	rec := do(h, http.MethodPost, "/signup", `{"email":"bad","tags":["a","b","c"]}`, nil)
	if rec.Code != 400 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env wireEnvelope
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Code != InvalidArgument || len(env.Error.Issues) != 2 {
		t.Fatalf("body %s", rec.Body.String())
	}
	if env.Error.Issues[0].Path[0] != "email" || env.Error.Issues[0].Rule != "email" {
		t.Fatalf("issues %+v", env.Error.Issues)
	}
}

func TestNilSlicesAreEmptyOnTheWire(t *testing.T) {
	h := NewRouter(Mutation("signup", signup)).Handler()
	rec := do(h, http.MethodPost, "/signup", `{"email":"a@b.co"}`, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tags":[]`) || strings.Contains(rec.Body.String(), `"roles"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestOutOfRangeIntegerIsInternal(t *testing.T) {
	h := NewRouter(Mutation("signup", signup)).Handler(Production(true), Logger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	rec := do(h, http.MethodPost, "/signup", `{"email":"huge@example.com"}`, nil)
	if rec.Code != 500 || errorCode(t, rec) != Internal {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUnsupportedValidateTagPanicsAtConstruction(t *testing.T) {
	type bad struct {
		N int `json:"n" validate:"gte=1"`
	}
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), "bad.N") {
			t.Fatalf("expected panic naming the field, got %v", r)
		}
	}()
	NewRouter(Query("x", func(ctx context.Context, in bad) (user, error) { return user{}, nil }))
}
