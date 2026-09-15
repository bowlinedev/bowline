package mock

import "context"

func withInput(ctx context.Context, input []byte) context.Context {
	return context.WithValue(ctx, inputKey{}, input)
}

func inputFrom(ctx context.Context) ([]byte, bool) {
	v, ok := ctx.Value(inputKey{}).([]byte)
	return v, ok
}
