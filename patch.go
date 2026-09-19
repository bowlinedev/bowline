package bowline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/big"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"

	routing "github.com/bowlinedev/bowline/internal/route"
)

const (
	MergePatchMediaType = "application/merge-patch+json"
	JSONPatchMediaType  = "application/json-patch+json"
)

type PatchOption func(*handler)

func RequireIfMatch() PatchOption {
	return func(h *handler) { h.patchRequiresIfMatch = true }
}

func AutoPatch(opts ...PatchOption) HandlerOption {
	return func(h *handler) {
		h.autoPatch = true
		for _, opt := range opts {
			opt(h)
		}
	}
}

type patchRoute struct {
	path     string
	segments int
	literals int
	read     *route
	write    *route
}

func deepCopyJSON(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = deepCopyJSON(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = deepCopyJSON(item)
		}
		return out
	}
	return value
}

func patchSpecificity(path string) (segments, literals int) {
	for part := range strings.SplitSeq(strings.Trim(path, "/"), "/") {
		if part == "" {
			continue
		}
		segments++
		if !strings.HasPrefix(part, "{") {
			literals++
		}
	}
	return segments, literals
}

func (h *handler) buildPatchRoutes() []*patchRoute {
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
	out := make([]*patchRoute, 0, len(reads))
	for path, read := range reads {
		write, ok := writes[path]
		if !ok {
			continue
		}
		segments, literals := patchSpecificity(path)
		out = append(out, &patchRoute{path: path, segments: segments, literals: literals, read: read, write: write})
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortFunc(out, func(a, b *patchRoute) int {
		if d := b.segments - a.segments; d != 0 {
			return d
		}
		if d := b.literals - a.literals; d != 0 {
			return d
		}
		return strings.Compare(a.path, b.path)
	})
	return out
}

const exactFloat64Digits = 15

func mayLosePrecision(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] < '0' || data[i] > '9' {
			continue
		}
		digits, dotted := 0, false
		for ; i < len(data); i++ {
			switch c := data[i]; {
			case c >= '0' && c <= '9':
				digits++
			case c == '.' && !dotted:
				dotted = true
			default:
				goto done
			}
		}
	done:
		if digits > exactFloat64Digits {
			return true
		}
	}
	return false
}

func decodeJSON(data []byte, out *any) error {
	if !mayLosePrecision(data) {
		return json.Unmarshal(data, out)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func jsonEqual(a, b any) bool {
	switch left := a.(type) {
	case json.Number:
		right, ok := b.(json.Number)
		if !ok {
			return false
		}
		if left == right {
			return true
		}
		x, okx := new(big.Rat).SetString(left.String())
		y, oky := new(big.Rat).SetString(right.String())
		return okx && oky && x.Cmp(y) == 0
	case map[string]any:
		right, ok := b.(map[string]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for key, value := range left {
			other, ok := right[key]
			if !ok || !jsonEqual(value, other) {
				return false
			}
		}
		return true
	case []any:
		right, ok := b.([]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for i, value := range left {
			if !jsonEqual(value, right[i]) {
				return false
			}
		}
		return true
	}
	return a == b
}

func applyMergePatch(target, patch []byte) ([]byte, error) {
	var patched any
	if err := decodeJSON(patch, &patched); err != nil {
		return nil, fmt.Errorf("the patch is not JSON: %w", err)
	}
	var current any
	if err := decodeJSON(target, &current); err != nil {
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
	if err := decodeJSON(target, &doc); err != nil {
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
	case "add":
		var value any
		if err := decodeJSON(op.Value, &value); err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}
		return addAt(doc, op.Path, value)
	case "replace":
		var value any
		if err := decodeJSON(op.Value, &value); err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}
		return replaceAt(doc, op.Path, value)
	case "remove":
		return removeAt(doc, op.Path)
	case "copy":
		value, err := readPointer(doc, op.From)
		if err != nil {
			return nil, err
		}
		return addAt(doc, op.Path, deepCopyJSON(value))
	case "move":
		if op.From != op.Path && strings.HasPrefix(op.Path, op.From+"/") {
			return nil, errors.New("a location cannot be moved into one of its children")
		}
		value, err := readPointer(doc, op.From)
		if err != nil {
			return nil, err
		}
		doc, err = removeAt(doc, op.From)
		if err != nil {
			return nil, err
		}
		return addAt(doc, op.Path, value)
	case "test":
		value, err := readPointer(doc, op.Path)
		if err != nil {
			return nil, err
		}
		var want any
		if err := decodeJSON(op.Value, &want); err != nil {
			return nil, fmt.Errorf("value is not JSON: %w", err)
		}
		if !jsonEqual(value, want) {
			got, _ := json.Marshal(value)
			expected, _ := json.Marshal(want)
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

func arrayIndex(token string, length int, allowEnd bool) (int, error) {
	if token == "-" {
		if !allowEnd {
			return 0, errors.New("\"-\" only addresses the end of an array when adding")
		}
		return length, nil
	}
	if token == "" || (len(token) > 1 && token[0] == '0') {
		return 0, fmt.Errorf("%q is not an array index", token)
	}
	n, err := strconv.Atoi(token)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%q is not an array index", token)
	}
	if n > length || (n == length && !allowEnd) {
		return 0, fmt.Errorf("index %d is outside an array of %d", n, length)
	}
	return n, nil
}

func childOf(parent any, token string) (any, error) {
	switch container := parent.(type) {
	case map[string]any:
		value, ok := container[token]
		if !ok {
			return nil, fmt.Errorf("%q does not exist", token)
		}
		return value, nil
	case []any:
		i, err := arrayIndex(token, len(container), false)
		if err != nil {
			return nil, err
		}
		return container[i], nil
	}
	return nil, fmt.Errorf("%q has no member %q", parent, token)
}

func walkTo(doc any, tokens []string) (any, error) {
	current := doc
	for _, token := range tokens {
		next, err := childOf(current, token)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

func readPointer(doc any, pointer string) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	value, err := walkTo(doc, tokens)
	if err != nil {
		return nil, fmt.Errorf("%q does not exist: %w", pointer, err)
	}
	return value, nil
}

func editParent(doc any, pointer string, edit func(parent any, token string) (any, error)) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return edit(nil, "")
	}
	parent, err := walkTo(doc, tokens[:len(tokens)-1])
	if err != nil {
		return nil, fmt.Errorf("%q does not exist: %w", pointer, err)
	}
	replaced, err := edit(parent, tokens[len(tokens)-1])
	if err != nil {
		return nil, err
	}
	if len(tokens) == 1 {
		return replaced, nil
	}
	return setChild(doc, tokens[:len(tokens)-1], replaced)
}

func setChild(doc any, tokens []string, value any) (any, error) {
	if len(tokens) == 0 {
		return value, nil
	}
	parent, err := walkTo(doc, tokens[:len(tokens)-1])
	if err != nil {
		return nil, err
	}
	last := tokens[len(tokens)-1]
	switch container := parent.(type) {
	case map[string]any:
		container[last] = value
	case []any:
		i, err := arrayIndex(last, len(container), false)
		if err != nil {
			return nil, err
		}
		container[i] = value
	default:
		return nil, fmt.Errorf("%q cannot hold a member", last)
	}
	return doc, nil
}

func addAt(doc any, pointer string, value any) (any, error) {
	return editParent(doc, pointer, func(parent any, token string) (any, error) {
		switch container := parent.(type) {
		case nil:
			return value, nil
		case map[string]any:
			container[token] = value
			return container, nil
		case []any:
			i, err := arrayIndex(token, len(container), true)
			if err != nil {
				return nil, err
			}
			grown := append(container, nil)
			copy(grown[i+1:], grown[i:])
			grown[i] = value
			return grown, nil
		}
		return nil, fmt.Errorf("%q cannot hold a member", token)
	})
}

func replaceAt(doc any, pointer string, value any) (any, error) {
	if _, err := readPointer(doc, pointer); err != nil {
		return nil, err
	}
	return editParent(doc, pointer, func(parent any, token string) (any, error) {
		switch container := parent.(type) {
		case nil:
			return value, nil
		case map[string]any:
			container[token] = value
			return container, nil
		case []any:
			i, err := arrayIndex(token, len(container), false)
			if err != nil {
				return nil, err
			}
			container[i] = value
			return container, nil
		}
		return nil, fmt.Errorf("%q cannot hold a member", token)
	})
}

func removeAt(doc any, pointer string) (any, error) {
	if pointer == "" {
		return nil, errors.New("the whole document cannot be removed")
	}
	if _, err := readPointer(doc, pointer); err != nil {
		return nil, err
	}
	return editParent(doc, pointer, func(parent any, token string) (any, error) {
		switch container := parent.(type) {
		case map[string]any:
			delete(container, token)
			return container, nil
		case []any:
			i, err := arrayIndex(token, len(container), false)
			if err != nil {
				return nil, err
			}
			return append(container[:i:i], container[i+1:]...), nil
		}
		return nil, fmt.Errorf("%q cannot hold a member", token)
	})
}

func (h *handler) patchRouteFor(req *http.Request) (*patchRoute, []routing.Param, bool) {
	if len(h.patches) == 0 || req.Method != http.MethodPatch || h.table == nil {
		return nil, nil, false
	}
	escaped := req.URL.EscapedPath()
	for _, pair := range h.patches {
		if match, ok := h.table.MatchPattern(escaped, pair.path); ok {
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
	if h.csrf != nil && !h.csrf.Allows(req) {
		h.writeError(w, req, nil, 0, Errorf(PermissionDenied, "cross-origin request rejected"))
		return true
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
	tag := current.header.Get("ETag")
	if tag == "" {
		tag = etagOf(current.body.Bytes())
	}
	conditions := requestConditions(req, "If-Match")
	switch {
	case len(conditions) == 0 && h.patchRequiresIfMatch:
		w.Header().Set("ETag", tag)
		h.writeError(w, req, nil, http.StatusPreconditionRequired, Errorf(FailedPrecondition, "this PATCH needs an If-Match header; the current entity tag is %s", tag))
		return true
	case len(conditions) > 0 && !matchesETagStrongly(conditions, tag):
		w.Header().Set("ETag", tag)
		h.writeError(w, req, nil, 0, Errorf(FailedPrecondition, "the resource has changed since %s", strings.Join(conditions, ", ")))
		return true
	}
	if absent := requestConditions(req, "If-None-Match"); len(absent) > 0 && matchesETag(absent, tag) {
		w.Header().Set("ETag", tag)
		h.writeError(w, req, nil, 0, Errorf(FailedPrecondition, "the resource already matches %s", strings.Join(absent, ", ")))
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
	h.dispatch(w, write, pair.write, params)
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
