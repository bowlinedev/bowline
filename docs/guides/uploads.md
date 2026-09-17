# Uploads

An upload is a procedure that receives a typed input and one file. The file is streamed to the procedure. The handler does not buffer it.

## Declaring one

The input is a normal struct and is validated like any other:

source: examples/ledger/api/attachments.go:12-14

```go
type AttachInput struct {
	InvoiceID int64 `json:"invoiceId" validate:"required"`
}
```

The procedure takes that input plus a `*bowline.File`:

source: examples/ledger/api/attachments.go:20-29

```go
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
```

It is registered with `bowline.Upload`:

source: examples/ledger/api/invoices.go:51-51

```go
		bowline.Upload("attach", a.attach, bowline.Description("Attach stores a file against an invoice.")),
```

`bowline.File` has the file name, the content type, and an `io.Reader`. Reading from it gives you the bytes as they arrive. The size is whatever the procedure counts. Input decoding and validation happen before the file part is read, so a bad input fails immediately without consuming the upload.

## The wire

Uploads are always `POST` with `multipart/form-data`. The first part is named `input` and contains the JSON input. The second part is named `file`. This order is required, because the input needs to be decoded before the file starts streaming. `bowline.MaxUploadSize(n)` on the handler limits the total body size. The default is 32 MiB. A larger body gets a 413 response with `INVALID_ARGUMENT`.

## On the client

source: examples/ledger/web/src/invoices.tsx:48-53

```tsx
  async function attach(id: number, file: File | undefined) {
    if (!file) {
      return;
    }
    setAttached(await client.invoices.attach({ invoiceId: id }, file));
  }
```

`file` can be a `Blob` or a `File`. The client builds the `FormData` in the required order and lets `fetch` set the multipart boundary. `client.invoices.attach.safe` returns a `Result`, the same as any other procedure.

The runtime tests in `upload_test.go` cover the part order, a missing file, validation before the file is read, the size limit, and the required method and content type.
