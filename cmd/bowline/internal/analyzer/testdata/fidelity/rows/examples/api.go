package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bowlinedev/bowline"
)

type Status string

const (
	StatusDraft Status = "draft"
	StatusSent  Status = "sent"
)

type Level int32

const (
	LevelLow  Level = 1
	LevelHigh Level = 10
)

type Slug string

type Address struct {
	City string `json:"city" example:"Paris"`
}

type Sample struct {
	Name    string           `json:"name" validate:"required" example:"Ada Lovelace"`
	Email   string           `json:"email" validate:"email" example:"ada@example.com"`
	Age     int32            `json:"age" validate:"min=0" example:"36"`
	Big     int64            `json:"big,string" example:"9007199254740993"`
	Ratio   float64          `json:"ratio" example:"0.75"`
	Active  bool             `json:"active" example:"true"`
	Status  Status           `json:"status" example:"sent"`
	Level   Level            `json:"level" example:"10"`
	Slug    Slug             `json:"slug" example:"ada-lovelace"`
	Since   time.Time        `json:"since" example:"2026-01-01T00:00:00Z"`
	Timeout time.Duration    `json:"timeout" example:"1500000000"`
	Blob    []byte           `json:"blob" example:"aGVsbG8="`
	Raw     json.RawMessage  `json:"raw" example:"{\"any\":1}"`
	Tags    []string         `json:"tags" example:"[\"a\",\"b\"]"`
	Counts  map[string]int32 `json:"counts" example:"{\"x\":1}"`
	Home    Address          `json:"home" example:"{\"city\":\"Rome\"}"`
	Nick    *string          `json:"nick,omitempty" example:"ada"`
	Ratings []Level          `json:"ratings"`
}

func Get(ctx context.Context, in struct{}) (Sample, error) { return Sample{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
