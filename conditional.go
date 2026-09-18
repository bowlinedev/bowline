package bowline

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

func ETags() HandlerOption {
	return func(h *handler) { h.etags = true }
}

func (c *Call) SetETag(tag string) {
	c.ResponseHeader().Set("ETag", quoteETag(tag))
}

func IfMatch(ctx context.Context) []string {
	return conditionHeader(ctx, "If-Match")
}

func IfNoneMatch(ctx context.Context) []string {
	return conditionHeader(ctx, "If-None-Match")
}

func conditionHeader(ctx context.Context, name string) []string {
	return requestConditions(requestFrom(ctx), name)
}

func quoteETag(tag string) string {
	if tag == "" {
		return ""
	}
	if strings.HasPrefix(tag, "W/\"") || (strings.HasPrefix(tag, "\"") && strings.HasSuffix(tag, "\"")) {
		return tag
	}
	return "\"" + tag + "\""
}

func unquoteETag(tag string) string {
	tag = strings.TrimPrefix(tag, "W/")
	return strings.Trim(tag, "\"")
}

func etagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return "\"" + base64.RawURLEncoding.EncodeToString(sum[:16]) + "\""
}

func matchesETag(candidates []string, tag string) bool {
	want := unquoteETag(tag)
	for _, candidate := range candidates {
		if candidate == "*" || unquoteETag(candidate) == want {
			return true
		}
	}
	return false
}

func matchesETagStrongly(candidates []string, tag string) bool {
	if isWeak(tag) {
		for _, candidate := range candidates {
			if candidate == "*" {
				return true
			}
		}
		return false
	}
	want := unquoteETag(tag)
	for _, candidate := range candidates {
		if candidate == "*" {
			return true
		}
		if !isWeak(candidate) && unquoteETag(candidate) == want {
			return true
		}
	}
	return false
}

func isWeak(tag string) bool {
	return strings.HasPrefix(strings.TrimSpace(tag), "W/")
}

func cacheableMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func requestConditions(req *http.Request, name string) []string {
	if req == nil {
		return nil
	}
	raw := req.Header.Get(name)
	if raw == "" {
		return nil
	}
	var out []string
	for value := range strings.SplitSeq(raw, ",") {
		if tag := strings.TrimSpace(value); tag != "" {
			out = append(out, tag)
		}
	}
	return out
}
