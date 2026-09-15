# Uploads

An upload is a procedure that receives a typed input and one file. The file is streamed to the procedure; the handler never buffers it.

## Declaring one

```go
type AttachInput struct {
	InvoiceID int64 `json:"invoiceId" validate:"required"`
}

func (a *API) attach(ctx context.Context, in AttachInput, file *bowline.File) (ledger.Attachment, error) {
	size, err := io.Copy(io.Discard, file)
	if err != nil {
		return ledger.Attachment{}, err
	}
	return a.store.Attach(in.InvoiceID, file.Name, file.ContentType, size), nil
}

bowline.Upload("attach", a.attach)
```

`bowline.File` carries the file name, the content type, and an `io.Reader`. Reading it yields the bytes as they arrive; the size is whatever the procedure counts. Input decoding and validation happen before the file part is touched, so a bad input fails fast without consuming the upload.

## The wire

Uploads are always `POST` with `multipart/form-data`. The first part is named `input` and contains the JSON input; the second part is named `file`. The order is required because the input must be decoded before the file streams. `bowline.MaxUploadSize(n)` on the handler caps the whole body, default 32 MiB; a larger body gets 413 with `INVALID_ARGUMENT`.

## On the client

```ts
const attachment = await client.invoices.attach({ invoiceId: 3 }, file);
```

`file` is a `Blob` or `File`. The client builds the `FormData` in the required order and lets `fetch` set the multipart boundary. `client.invoices.attach.safe` returns a `Result` like any other procedure.

The runtime tests in `upload_test.go` cover the part order, a missing file, validation before the file is read, the size limit, and the required method and content type.
