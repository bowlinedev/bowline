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
	Description string `json:"description" validate:"required,max=200"`
	Quantity    int32  `json:"quantity" validate:"min=1"`
	UnitPrice   Money  `json:"unitPrice"`
}

type Invoice struct {
	ID         int64   `json:"id"`
	CustomerID int64   `json:"customerId"`
	Status     Status  `json:"status"`
	Total      Money   `json:"total"`
	Lines      []Line  `json:"lines"`
	Note       *string `json:"note,omitempty"`
	Audit
}

type Customer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Audit
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}
