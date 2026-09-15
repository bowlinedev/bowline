package api

import (
	"fmt"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

// InvoiceLocked is returned when an invoice can no longer change.
type InvoiceLocked struct {
	ID     int64         `json:"id"`
	Status ledger.Status `json:"status"`
}

func (e InvoiceLocked) Error() string { return fmt.Sprintf("invoice %d is %s", e.ID, e.Status) }

func (e InvoiceLocked) Code() bowline.Code { return bowline.FailedPrecondition }
