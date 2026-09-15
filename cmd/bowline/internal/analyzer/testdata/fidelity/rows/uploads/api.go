package uploads

import (
	"context"

	"github.com/bowlinedev/bowline"
)

type AttachInput struct {
	InvoiceID int64 `json:"invoiceId" validate:"required"`
}

type Attachment struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Attach stores a file against an invoice.
func Attach(ctx context.Context, in AttachInput, file *bowline.File) (Attachment, error) {
	return Attachment{}, nil
}

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Upload("attach", Attach, bowline.Description("Attach stores a file.")))
}
