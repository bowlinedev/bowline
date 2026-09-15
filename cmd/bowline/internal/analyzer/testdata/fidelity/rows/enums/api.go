package enums

import (
	"context"

	"github.com/bowlinedev/bowline"
)

// Status is the lifecycle state of an order.
type Status string

const (
	StatusDraft Status = "draft"
	StatusSent  Status = "sent"
	statusVoid  Status = "void"
)

type Level int32

const (
	LevelLow  Level = 1
	LevelHigh Level = 10
)

type Slug string

type Order struct {
	Status  Status           `json:"status" validate:"oneof=draft sent"`
	Level   Level            `json:"level"`
	Slug    Slug             `json:"slug"`
	History map[Status]int32 `json:"history"`
	Levels  []Level          `json:"levels"`
}

func Get(ctx context.Context, in struct{}) (Order, error) { return Order{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("get", Get))
}
