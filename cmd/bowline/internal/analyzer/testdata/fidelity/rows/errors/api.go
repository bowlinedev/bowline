package errors

import (
	"context"
	"fmt"
	"time"

	"github.com/bowlinedev/bowline"
)

type Status string

const (
	StatusDraft Status = "draft"
	StatusPaid  Status = "paid"
)

// InvoiceLocked is returned when an invoice cannot change.
type InvoiceLocked struct {
	ID     int64     `json:"id"`
	Status Status    `json:"status"`
	Since  time.Time `json:"since"`
}

func (e InvoiceLocked) Error() string { return fmt.Sprintf("invoice %d is %s", e.ID, e.Status) }

func (e InvoiceLocked) Code() bowline.Code { return bowline.FailedPrecondition }

type QuotaExceeded struct {
	Limit int32 `json:"limit"`
}

func (e *QuotaExceeded) Error() string { return "quota exceeded" }

func (e *QuotaExceeded) Code() bowline.Code { return bowline.ResourceExhausted }

type ID struct {
	ID int64 `json:"id"`
}

type Invoice struct {
	ID int64 `json:"id"`
}

func Void(ctx context.Context, in ID) (Invoice, error) { return Invoice{}, nil }

func Send(ctx context.Context, in ID) (Invoice, error) { return Invoice{}, nil }

func Routes() *bowline.Router {
	return bowline.NewRouter(
		bowline.Mutation("void", Void, bowline.Errors(InvoiceLocked{}, &QuotaExceeded{})),
		bowline.Mutation("send", Send, bowline.Errors(InvoiceLocked{})),
	)
}
