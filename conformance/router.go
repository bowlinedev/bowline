package conformance

import (
	"context"
	"io"
	"time"

	"github.com/bowlinedev/bowline"
)

type EchoInput struct {
	Message string `json:"message"`
	Count   int32  `json:"count,omitempty"`
}

type EchoOutput struct {
	Message string   `json:"message"`
	Count   int32    `json:"count"`
	Tags    []string `json:"tags"`
	Attrs   map[string]string
	At      time.Time `json:"at"`
	Big     int64     `json:"big,string"`
}

type CreateInput struct {
	Name string `json:"name" validate:"required"`
}

type Created struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type FailInput struct {
	Code string `json:"code"`
}

type Empty struct{}

type ValidateInput struct {
	Name    string   `json:"name" validate:"required"`
	Age     int32    `json:"age" validate:"min=18,max=130"`
	Tags    []string `json:"tags" validate:"len=2"`
	Kind    string   `json:"kind" validate:"oneof=draft final"`
	Email   string   `json:"email" validate:"email"`
	Website string   `json:"website" validate:"url"`
	Token   string   `json:"token" validate:"uuid"`
}

type Deep struct {
	Depth int32 `json:"depth"`
}

type TicksInput struct {
	Count int32 `json:"count"`
}

type Tick struct {
	N int32 `json:"n"`
}

type UploadInput struct {
	Label string `json:"label" validate:"required"`
}

type Uploaded struct {
	Label string `json:"label"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
}

func Router() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("echo", echo),
		bowline.Query("sensitive", echo, bowline.Sensitive()),
		bowline.Mutation("create", create),
		bowline.Query("fail", fail),
		bowline.Query("panic", boom),
		bowline.Mutation("validate", validate),
		bowline.Query("old", echo, bowline.Deprecated("use echo")),
		bowline.Mount("nested", bowline.NewRouter(
			bowline.Mount("deep", bowline.NewRouter(bowline.Query("get", deep))),
		)),
		bowline.Subscription("ticks", ticks),
		bowline.Upload("upload", upload),
	)
}

func echo(ctx context.Context, in EchoInput) (EchoOutput, error) {
	return EchoOutput{Message: in.Message, Count: in.Count, At: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Big: 9007199254740993}, nil
}

func create(ctx context.Context, in CreateInput) (Created, error) {
	return Created{ID: 1, Name: in.Name}, nil
}

func fail(ctx context.Context, in FailInput) (Empty, error) {
	return Empty{}, bowline.Errorf(bowline.Code(in.Code), "failed with %s", in.Code)
}

func boom(ctx context.Context, in Empty) (Empty, error) {
	panic("conformance panic")
}

func validate(ctx context.Context, in ValidateInput) (Empty, error) {
	return Empty{}, nil
}

func deep(ctx context.Context, in Empty) (Deep, error) {
	return Deep{Depth: 2}, nil
}

func ticks(ctx context.Context, in TicksInput, stream *bowline.Stream[Tick]) error {
	count := in.Count
	if count == 0 {
		count = 3
	}
	for i := int32(1); i <= count; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := stream.Send(Tick{N: i}); err != nil {
			return err
		}
	}
	return nil
}

func upload(ctx context.Context, in UploadInput, file *bowline.File) (Uploaded, error) {
	size, err := io.Copy(io.Discard, file)
	if err != nil {
		return Uploaded{}, err
	}
	return Uploaded{Label: in.Label, Name: file.Name, Size: size}, nil
}
