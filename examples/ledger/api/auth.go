package api

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/bowlinedev/bowline"
)

func RequireToken(token string) bowline.Middleware {
	return func(next bowline.Next) bowline.Next {
		if token == "" {
			return next
		}
		return func(ctx context.Context, in any) (any, error) {
			call := bowline.CallFrom(ctx)
			header := ""
			if call.Request != nil {
				header = call.Request.Header.Get("Authorization")
			}
			got, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				return nil, bowline.Errorf(bowline.Unauthenticated, "a bearer token is required")
			}
			return next(ctx, in)
		}
	}
}
