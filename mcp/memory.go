package mcp

import (
	"bytes"
	"net/http"
)

type memoryWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newMemoryWriter() *memoryWriter {
	return &memoryWriter{header: http.Header{}, status: http.StatusOK}
}

func (w *memoryWriter) Header() http.Header {
	return w.header
}

func (w *memoryWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *memoryWriter) WriteHeader(status int) {
	w.status = status
}

func (w *memoryWriter) Flush() {}
