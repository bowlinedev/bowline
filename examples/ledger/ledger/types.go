package ledger

import "time"

type Status string

const (
	StatusDraft Status = "draft"
	StatusSent  Status = "sent"
	StatusPaid  Status = "paid"
	StatusVoid  Status = "void"
)

type Audit struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Line struct {
	Description string `json:"description" validate:"required,max=200" example:"Consulting"`
	Quantity    int32  `json:"quantity" validate:"min=1" example:"10"`
	UnitPrice   Money  `json:"unitPrice"`
}

type Invoice struct {
	ID         int64   `json:"id"`
	CustomerID int64   `json:"customerId"`
	Status     Status  `json:"status"`
	Total      Money   `json:"total"`
	Lines      []Line  `json:"lines"`
	Note       *string `json:"note,omitempty" example:"net 30"`
	Audit
}

type Customer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name" validate:"required" example:"Ada Lovelace"`
	Email string `json:"email" validate:"required,email" example:"ada@example.com"`
	Audit
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type Attachment struct {
	ID          int64  `json:"id"`
	InvoiceID   int64  `json:"invoiceId"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Audit
}
