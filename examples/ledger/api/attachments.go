package api

import (
	"context"
	"errors"
	"io"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

type AttachInput struct {
	InvoiceID int64 `json:"invoiceId" validate:"required"`
}

type ListAttachmentsInput struct {
	InvoiceID int64 `json:"invoiceId" validate:"required"`
}

func (a *API) attach(ctx context.Context, in AttachInput, file *bowline.File) (ledger.Attachment, error) {
	if _, err := a.store.Invoice(in.InvoiceID); errors.Is(err, ledger.ErrNotFound) {
		return ledger.Attachment{}, bowline.Errorf(bowline.NotFound, "invoice %d not found", in.InvoiceID)
	}
	size, err := io.Copy(io.Discard, file)
	if err != nil {
		return ledger.Attachment{}, err
	}
	return a.store.Attach(in.InvoiceID, file.Name, file.ContentType, size), nil
}

func (a *API) listAttachments(ctx context.Context, in ListAttachmentsInput) (ledger.Page[ledger.Attachment], error) {
	return ledger.Page[ledger.Attachment]{Items: a.store.Attachments(in.InvoiceID)}, nil
}
