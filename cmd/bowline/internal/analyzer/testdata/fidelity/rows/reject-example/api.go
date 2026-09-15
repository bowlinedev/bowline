package rejectexample

import (
	"context"
	"time"

	"github.com/bowlinedev/bowline"
)

type Status string

const (
	StatusDraft Status = "draft"
	StatusSent  Status = "sent"
)

type Address struct {
	City string `json:"city"`
}

type Sample struct {
	Since  time.Time `json:"since" example:"yesterday"`
	Status Status    `json:"status" example:"void"`
	Home   Address   `json:"home" example:"{city: Rome}"`
	Age    int32     `json:"age" example:"many"`
}

func Get(ctx context.Context, in struct{}) (Sample, error) { return Sample{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
