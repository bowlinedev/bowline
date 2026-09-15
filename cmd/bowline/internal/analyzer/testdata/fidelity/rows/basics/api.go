package basics

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type Input struct {
	Name  string  `json:"name"`
	Count int     `json:"count"`
	Small int32   `json:"small"`
	Big   uint64  `json:"big"`
	Ratio float64 `json:"ratio"`
	On    bool    `json:"on"`
}

type Output struct {
	Echo string `json:"echo"`
}

func Echo(ctx context.Context, in Input) (Output, error) {
	return Output{Echo: in.Name}, nil
}

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("echo", Echo))
}
