package bowline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/bowlinedev/bowline/internal/codec"
	routing "github.com/bowlinedev/bowline/internal/route"
)

type File struct {
	Name        string
	ContentType string
	io.Reader
}

func MaxUploadSize(n int64) HandlerOption {
	return func(h *handler) { h.maxUpload = n }
}

func Upload[In, Out any](name string, fn func(context.Context, In, *File) (Out, error), opts ...ProcOption) Item {
	if fn == nil {
		panic(fmt.Sprintf("bowline: upload %q: nil handler", name))
	}
	p := prepare[In, Out](KindUpload, name)
	p.attachFile = func(in any, file *File) any {
		return &uploadInput[In]{in: in.(*In), file: file}
	}
	p.call = func(ctx context.Context, in any) (any, error) {
		bound := in.(*uploadInput[In])
		return fn(ctx, *bound.in, bound.file)
	}
	for _, opt := range opts {
		opt(p)
	}
	checkOptions(p)
	return procItem{p}
}

type uploadInput[In any] struct {
	in   *In
	file *File
}

func (h *handler) serveUpload(w http.ResponseWriter, req *http.Request, rt *route, pathParams map[string]string) {
	proc := rt.proc
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		h.writeError(w, nil, http.StatusUnsupportedMediaType, Errorf(InvalidArgument, "uploads require multipart/form-data with an input part followed by a file part"))
		return
	}
	limit := h.maxUpload
	if limit <= 0 {
		limit = 32 << 20
	}
	if proc.MaxBody > 0 {
		limit = proc.MaxBody
	}
	reader := multipart.NewReader(http.MaxBytesReader(w, req.Body, limit), params["boundary"])
	inputPart, err := reader.NextPart()
	if err != nil || inputPart.FormName() != "input" {
		h.writeError(w, nil, 0, Errorf(InvalidArgument, "the first multipart part must be named input"))
		return
	}
	raw, err := io.ReadAll(io.LimitReader(inputPart, h.maxBody+1))
	if err != nil {
		h.writeError(w, nil, 0, h.invalidInput(fmt.Errorf("reading input part: %w", err)))
		return
	}
	if int64(len(raw)) > h.maxBody {
		h.writeError(w, nil, http.StatusRequestEntityTooLarge, Errorf(InvalidArgument, "input part exceeds %d bytes", h.maxBody))
		return
	}
	ctx, ptr := proc.newFrame(req.Context(), Call{Procedure: &rt.procedure, Request: req})
	if err := codec.Decode(raw, ptr, h.strict); err != nil {
		h.writeError(w, nil, 0, h.invalidInput(err))
		return
	}
	if err := routing.Bind(ptr, pathParams); err != nil {
		h.writeError(w, nil, 0, Errorf(InvalidArgument, "%s", err.Error()))
		return
	}
	if issues := proc.checker.Check(ptr); len(issues) > 0 {
		e := Errorf(InvalidArgument, "invalid input")
		e.Issues = make([]Issue, len(issues))
		for i, issue := range issues {
			e.Issues[i] = Issue{Path: issue.Path, Rule: issue.Rule, Message: issue.Message}
		}
		h.writeError(w, nil, 0, e)
		return
	}
	filePart, err := reader.NextPart()
	if err != nil || filePart.FormName() != "file" {
		h.writeError(w, nil, 0, Errorf(InvalidArgument, "the second multipart part must be named file"))
		return
	}
	file := &File{Name: filePart.FileName(), ContentType: filePart.Header.Get("Content-Type"), Reader: filePart}
	out, err := h.invoke(ctx, rt, proc.attachFile(ptr, file))
	applyResponseHeader(w, ctx)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.writeError(w, nil, http.StatusRequestEntityTooLarge, Errorf(InvalidArgument, "upload exceeds %d bytes", limit))
			return
		}
		status, _, undeclared := classify(err, h.production, proc.variants)
		if status >= 500 {
			h.log.ErrorContext(ctx, "bowline: procedure failed", "procedure", rt.path, "error", err)
		}
		if undeclared && !h.production {
			h.log.WarnContext(ctx, "bowline: undeclared error variant", "procedure", rt.path, "error", err)
		}
		h.writeError(w, proc, 0, err)
		return
	}
	h.writeOutput(w, ctx, rt, out)
}
