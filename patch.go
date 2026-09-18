package bowline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"strings"

	routing "github.com/bowlinedev/bowline/internal/route"
)

const (
	MergePatchMediaType = "application/merge-patch+json"
	JSONPatchMediaType  = "application/json-patch+json"
)

func AutoPatch() HandlerOption {
	return func(h *handler) { h.autoPatch = true }
}

type patchRoute struct {
	read  *route
	write *route
}

func (h *handler) buildPatchRoutes() map[string]*patchRoute {
	if !h.autoPatch {
		return nil
	}
	reads := map[string]*route{}
	writes := map[string]*route{}
	for _, rt := range h.routes {
		path := rt.proc.HTTPPath
		if path == "" {
			continue
		}
		switch {
		case rt.proc.Kind == KindQuery && rt.proc.Method() == http.MethodGet:
			reads[path] = rt
		case rt.proc.Kind == KindMutation && rt.proc.Method() == http.MethodPut:
			writes[path] = rt
		}
	}
	out := map[string]*patchRoute{}
	for path, read := range reads {
		if write, ok := writes[path]; ok {
			out[path] = &patchRoute{read: read, write: write}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyMergePatch(target, patch []byte) ([]byte, error) {
	var patched any
	if err := json.Unmarshal(patch, &patched); err != nil {
		return nil, fmt.Errorf("the patch is not JSON: %w", err)
	}
	var current any
	if err := json.Unmarshal(target, &current); err != nil {
		return nil, fmt.Errorf("the current value is not JSON: %w", err)
	}
	return json.Marshal(mergeValue(current, patched))
}

func mergeValue(current, patch any) any {
	patchObject, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	currentObject, ok := current.(map[string]any)
	if !ok {
		currentObject = map[string]any{}
	}
	for key, value := range patchObject {
		if value == nil {
			delete(currentObject, key)
			continue
		}
		currentObject[key] = mergeValue(currentObject[key], value)
	}
	return currentObject
}

type patchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	From  string          `json:"from"`
	Value json.RawMessage `json:"value"`
}

func applyJSONPatch(target, patch []byte) ([]byte, error) {
	var ops []patchOperation
	if err := json.Unmarshal(patch, &ops); err != nil {
		return nil, fmt.Errorf("the patch is not a JSON Patch document: %w", err)
	}
	var doc any
	if err := json.Unmarshal(target, &doc); err != nil {
		return nil, fmt.Errorf("the current value is not JSON: %w", err)
	}
	for i, op := range ops {
		var err error
		doc, err = applyOperation(doc, op)
		if err != nil {
			return nil, fmt.Errorf("operation %d (%s %s): %w", i, op.Op, op.Path, err)
		}
	}
	return json.Marshal(doc)
}

func applyOperation(doc any, op patchOperation) (any, error) {
	switch op.Op {
	case "add", "replace":
		var value any
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}
		return setPointer(doc, op.Path, value)
	case "remove":
		return removePointer(doc, op.Path)
	case "copy":
		value, err := readPointer(doc, op.From)
		if err != nil {
			return nil, err
		}
		return setPointer(doc, op.Path, value)
	case "move":
		value, err := readPointer(doc, op.From)
		if err != nil {
			return nil, err
		}
		doc, err = removePointer(doc, op.From)
		if err != nil {
			return nil, err
		}
		return setPointer(doc, op.Path, value)
	case "test":
		value, err := readPointer(doc, op.Path)
		if err != nil {
			return nil, err
		}
		var want any
		if err := json.Unmarshal(op.Value, &want); err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}
		got, _ := json.Marshal(value)
		expected, _ := json.Marshal(want)
		if !bytes.Equal(got, expected) {
			return nil, fmt.Errorf("the value is %s, not %s", got, expected)
		}
		return doc, nil
	}
	return nil, fmt.Errorf("unsupported operation %q", op.Op)
}

func pointerTokens(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("a JSON pointer must start with a slash")
	}
	parts := strings.Split(pointer[1:], "/")
	for i, part := range parts {
		part = strings.ReplaceAll(part, "~1", "/")
		parts[i] = strings.ReplaceAll(part, "~0", "~")
	}
	return parts, nil
}

func readPointer(doc any, pointer string) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	current := doc
	for _, token := range tokens {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q does not exist", pointer)
		}
		current, ok = object[token]
		if !ok {
			return nil, fmt.Errorf("%q does not exist", pointer)
		}
	}
	return current, nil
}

func setPointer(doc any, pointer string, value any) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return value, nil
	}
	object, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q has no parent object", pointer)
	}
	parent := object
	for _, token := range tokens[:len(tokens)-1] {
		next, ok := parent[token].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q does not exist", pointer)
		}
		parent = next
	}
	parent[tokens[len(tokens)-1]] = value
	return object, nil
}

func removePointer(doc any, pointer string) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, errors.New("the whole document cannot be removed")
	}
	object, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q has no parent object", pointer)
	}
	parent := object
	for _, token := range tokens[:len(tokens)-1] {
		next, ok := parent[token].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q does not exist", pointer)
		}
		parent = next
	}
	last := tokens[len(tokens)-1]
	if _, ok := parent[last]; !ok {
		return nil, fmt.Errorf("%q does not exist", pointer)
	}
	delete(parent, last)
	return object, nil
}

func (h *handler) patchRouteFor(req *http.Request) (*patchRoute, []routing.Param, bool) {
	if len(h.patches) == 0 || req.Method != http.MethodPatch || h.table == nil {
		return nil, nil, false
	}
	for path, pair := range h.patches {
		if match, ok := h.table.MatchPattern(req.URL.EscapedPath(), path); ok {
			return pair, match, true
		}
	}
	return nil, nil, false
}

func (h *handler) servePatch(w http.ResponseWriter, req *http.Request) bool {
	pair, params, ok := h.patchRouteFor(req)
	if !ok {
		return false
	}
	apply, err := patchApplier(req.Header.Get("Content-Type"))
	if err != nil {
		h.writeError(w, req, nil, http.StatusUnsupportedMediaType, err)
		return true
	}
	limit := h.bodyLimit(pair.write.proc)
	patch, readErr := readBody(http.MaxBytesReader(w, req.Body, limit), req.ContentLength, limit)
	if readErr != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(readErr, &tooLarge) {
			h.writeError(w, req, nil, http.StatusRequestEntityTooLarge, Errorf(InvalidArgument, "patch exceeds %d bytes", limit))
			return true
		}
		h.writeError(w, req, nil, 0, h.invalidInput(readErr))
		return true
	}
	current := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
	read := req.Clone(req.Context())
	read.Method = http.MethodGet
	read.Body = http.NoBody
	read.ContentLength = 0
	h.execute(current, read, pair.read, params)
	if current.status != http.StatusOK {
		current.flushTo(w)
		return true
	}
	merged, err := apply(current.body.Bytes(), patch)
	if err != nil {
		h.writeError(w, req, nil, 0, Errorf(InvalidArgument, "%s", err.Error()))
		return true
	}
	write := req.Clone(req.Context())
	write.Method = http.MethodPut
	write.Body = io.NopCloser(bytes.NewReader(merged))
	write.ContentLength = int64(len(merged))
	write.Header.Set("Content-Type", "application/json")
	h.execute(w, write, pair.write, params)
	return true
}

func patchApplier(contentType string) (func(target, patch []byte) ([]byte, error), error) {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil && contentType != "" {
		return nil, Errorf(InvalidArgument, "content type %q is not a media type", contentType)
	}
	switch media {
	case MergePatchMediaType, "application/json", "":
		return applyMergePatch, nil
	case JSONPatchMediaType:
		return applyJSONPatch, nil
	}
	return nil, Errorf(InvalidArgument, "a PATCH body must be %s or %s", MergePatchMediaType, JSONPatchMediaType)
}

type bufferedResponse struct {
	header  http.Header
	body    bytes.Buffer
	status  int
	written bool
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(status int) {
	if !b.written {
		b.status, b.written = status, true
	}
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	b.written = true
	return b.body.Write(p)
}

func (b *bufferedResponse) flushTo(w http.ResponseWriter) {
	maps.Copy(w.Header(), b.header)
	w.WriteHeader(b.status)
	w.Write(b.body.Bytes())
}
