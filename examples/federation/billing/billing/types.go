package billing

import "time"

type Status string

const (
	StatusOpen     Status = "open"
	StatusSettled  Status = "settled"
	StatusRefunded Status = "refunded"
)

type Charge struct {
	ID        int64     `json:"id"`
	InvoiceID int64     `json:"invoiceId"`
	Amount    string    `json:"amount" example:"USD 1500.00"`
	Status    Status    `json:"status" example:"open"`
	CreatedAt time.Time `json:"createdAt"`
}

type Page[T any] struct {
	Items []T `json:"items"`
}
