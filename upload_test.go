package bowline

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type attachInput struct {
	InvoiceID int64 `json:"invoiceId" validate:"required"`
}

type attachment struct {
	InvoiceID   int64  `json:"invoiceId"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Head        string `json:"head"`
}

func attach(ctx context.Context, in attachInput, file *File) (attachment, error) {
	head := make([]byte, 4)
	n, _ := io.ReadFull(file, head)
	rest, err := io.Copy(io.Discard, file)
	if err != nil {
		return attachment{}, err
	}
	return attachment{InvoiceID: in.InvoiceID, Name: file.Name, ContentType: file.ContentType, Size: int64(n) + rest, Head: string(head[:n])}, nil
}

func uploadHandler(opts ...HandlerOption) http.Handler {
	return NewRouter(Mount("invoices", NewRouter(Upload("attach", attach)))).Handler(opts...)
}

func multipartBody(t *testing.T, parts [][3]string) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		var w io.Writer
		var err error
		if p[0] == "file" {
			w, err = mw.CreateFormFile("file", p[1])
		} else {
			w, err = mw.CreateFormField(p[0])
		}
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, p[2])
	}
	mw.Close()
	return mw.FormDataContentType(), &buf
}

func upload(h http.Handler, contentType string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/invoices.attach", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadStreamsFileToProcedure(t *testing.T) {
	ct, body := multipartBody(t, [][3]string{{"input", "", `{"invoiceId":3}`}, {"file", "receipt.pdf", "%PDF-1.7 hello"}})
	rec := upload(uploadHandler(), ct, body)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"invoiceId":3`, `"name":"receipt.pdf"`, `"contentType":"application/octet-stream"`, `"size":14`, `"head":"%PDF"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %s in %s", want, rec.Body.String())
		}
	}
}

func TestUploadRejectsWrongPartOrder(t *testing.T) {
	ct, body := multipartBody(t, [][3]string{{"file", "a.txt", "x"}, {"input", "", `{"invoiceId":3}`}})
	rec := upload(uploadHandler(), ct, body)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "first multipart part") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUploadRejectsMissingFile(t *testing.T) {
	ct, body := multipartBody(t, [][3]string{{"input", "", `{"invoiceId":3}`}})
	rec := upload(uploadHandler(), ct, body)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "second multipart part") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUploadValidatesInputBeforeReadingFile(t *testing.T) {
	ct, body := multipartBody(t, [][3]string{{"input", "", `{"invoiceId":0}`}, {"file", "a.txt", "x"}})
	rec := upload(uploadHandler(), ct, body)
	if rec.Code != 400 || errorCode(t, rec) != InvalidArgument || !strings.Contains(rec.Body.String(), `"invoiceId"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUploadSizeLimit(t *testing.T) {
	ct, body := multipartBody(t, [][3]string{{"input", "", `{"invoiceId":3}`}, {"file", "big.bin", strings.Repeat("x", 5000)}})
	rec := upload(uploadHandler(MaxUploadSize(1024)), ct, body)
	if rec.Code != 413 || errorCode(t, rec) != InvalidArgument {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUploadRequiresMultipartPost(t *testing.T) {
	rec := upload(uploadHandler(), "application/json", strings.NewReader(`{}`))
	if rec.Code != 415 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/invoices.attach", nil)
	rec = httptest.NewRecorder()
	uploadHandler().ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Fatalf("GET status %d", rec.Code)
	}
	procs := NewRouter(Upload("attach", attach)).Procedures()
	if procs[0].Kind != KindUpload || procs[0].Method() != "POST" {
		t.Fatalf("%+v", procs[0])
	}
}
