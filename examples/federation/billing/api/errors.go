package api

import (
	"fmt"

	"github.com/bowlinedev/bowline"
)

// UnknownInvoice is returned when the ledger has no invoice with the given ID.
type UnknownInvoice struct {
	InvoiceID int64 `json:"invoiceId"`
}

func (e UnknownInvoice) Error() string { return fmt.Sprintf("invoice %d is not in the ledger", e.InvoiceID) }

func (e UnknownInvoice) Code() bowline.Code { return bowline.FailedPrecondition }
