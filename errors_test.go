package bowline

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestEveryCodeHasAStatus(t *testing.T) {
	want := map[Code]int{
		Canceled: 408, Unknown: 500, InvalidArgument: 400, DeadlineExceeded: 408,
		NotFound: 404, AlreadyExists: 409, PermissionDenied: 403, ResourceExhausted: 429,
		FailedPrecondition: 412, Aborted: 409, OutOfRange: 400, Unimplemented: 404,
		Internal: 500, Unavailable: 503, DataLoss: 500, Unauthenticated: 401,
	}
	for code, status := range want {
		if got := code.HTTPStatus(); got != status {
			t.Errorf("%s: got %d, want %d", code, got, status)
		}
	}
	if got := Code("MADE_UP").HTTPStatus(); got != 500 {
		t.Errorf("unknown code: got %d, want 500", got)
	}
}

func TestErrorfWrapsCause(t *testing.T) {
	cause := errors.New("db down")
	err := Errorf(Unavailable, "fetching user: %w", cause)
	if !errors.Is(err, cause) {
		t.Fatal("cause not wrapped")
	}
	if err.Error() != "UNAVAILABLE: fetching user: db down" {
		t.Fatalf("unexpected message %q", err.Error())
	}
	var be *Error
	if !errors.As(fmt.Errorf("outer: %w", err), &be) || be.Code != Unavailable {
		t.Fatal("errors.As failed through wrapping")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		production bool
		status     int
		code       Code
		message    string
	}{
		{"bowline error", Errorf(NotFound, "no user 7"), true, 404, NotFound, "no user 7"},
		{"wrapped bowline error", fmt.Errorf("ctx: %w", Errorf(PermissionDenied, "nope")), true, 403, PermissionDenied, "nope"},
		{"canceled", context.Canceled, true, 408, Canceled, "request canceled"},
		{"deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), true, 408, DeadlineExceeded, "deadline exceeded"},
		{"plain error in production", errors.New("secret detail"), true, 500, Internal, "internal error"},
		{"plain error in development", errors.New("secret detail"), false, 500, Internal, "secret detail"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, env := classify(tc.err, tc.production)
			if status != tc.status || env.Error.Code != tc.code || env.Error.Message != tc.message {
				t.Fatalf("got %d %s %q, want %d %s %q", status, env.Error.Code, env.Error.Message, tc.status, tc.code, tc.message)
			}
		})
	}
}

func TestClassifyKeepsDetailsAndIssues(t *testing.T) {
	err := Errorf(InvalidArgument, "invalid input").WithDetails(map[string]int{"n": 1})
	err.Issues = []Issue{{Path: []string{"email"}, Rule: "email", Message: "must be a valid email"}}
	_, env := classify(err, true)
	if env.Error.Details == nil || len(env.Error.Issues) != 1 {
		t.Fatal("details or issues dropped")
	}
}
