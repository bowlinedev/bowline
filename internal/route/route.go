package route

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type Entry struct {
	Pattern string
	Method  string
	Key     string
}

type Match struct {
	Key    string
	Params map[string]string
}

type pattern struct {
	segments []string
	literals int
	params   []string
}

type compiled struct {
	pattern
	method string
	key    string
}

type Table struct {
	entries []compiled
}

func Params(p string) ([]string, error) {
	c, err := compile(p)
	if err != nil {
		return nil, err
	}
	return c.params, nil
}

func compile(p string) (pattern, error) {
	parsed, err := contract.ParsePath(p)
	if err != nil {
		return pattern{}, fmt.Errorf("route pattern %q: %w", p, err)
	}
	out := pattern{segments: make([]string, len(parsed)), params: nil}
	for i, seg := range parsed {
		if seg.Param {
			out.segments[i] = "{" + seg.Text + "}"
			out.params = append(out.params, seg.Text)
			continue
		}
		out.segments[i] = seg.Text
		out.literals++
	}
	return out, nil
}

func (p pattern) isParam(i int) bool {
	return strings.HasPrefix(p.segments[i], "{")
}

func (p pattern) sameShapeAs(other pattern) bool {
	if len(p.segments) != len(other.segments) {
		return false
	}
	for i := range p.segments {
		if p.isParam(i) != other.isParam(i) {
			return false
		}
		if !p.isParam(i) && p.segments[i] != other.segments[i] {
			return false
		}
	}
	return true
}

func New(entries []Entry) (*Table, error) {
	t := &Table{}
	for _, e := range entries {
		p, err := compile(e.Pattern)
		if err != nil {
			return nil, err
		}
		t.entries = append(t.entries, compiled{pattern: p, method: e.Method, key: e.Key})
	}
	for i, a := range t.entries {
		for j, b := range t.entries {
			if i >= j || a.method != b.method {
				continue
			}
			if a.sameShapeAs(b.pattern) {
				return nil, fmt.Errorf("routes %q and %q both answer %s at %q; give one of them a different path", a.key, b.key, a.method, strings.Join(a.segments, "/"))
			}
		}
	}
	slices.SortStableFunc(t.entries, func(a, b compiled) int {
		if d := len(b.segments) - len(a.segments); d != 0 {
			return d
		}
		return b.literals - a.literals
	})
	return t, nil
}

func segmentsOf(escaped string) []string {
	escaped = strings.TrimSuffix(escaped, "/")
	escaped = strings.TrimPrefix(escaped, "/")
	if escaped == "" {
		return nil
	}
	parts := strings.Split(escaped, "/")
	for i, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil
		}
		parts[i] = decoded
	}
	return parts
}

func (c compiled) match(segs []string) (map[string]string, bool) {
	if len(c.segments) > len(segs) {
		return nil, false
	}
	offset := len(segs) - len(c.segments)
	var params map[string]string
	for i, seg := range c.segments {
		got := segs[offset+i]
		if !c.isParam(i) {
			if seg != got {
				return nil, false
			}
			continue
		}
		if got == "" {
			return nil, false
		}
		if params == nil {
			params = make(map[string]string, len(c.params))
		}
		params[seg[1:len(seg)-1]] = got
	}
	return params, true
}

func (t *Table) Match(path, method string) (Match, bool) {
	segs := segmentsOf(path)
	if len(segs) == 0 {
		return Match{}, false
	}
	for _, c := range t.entries {
		if c.method != method {
			continue
		}
		if params, ok := c.match(segs); ok {
			return Match{Key: c.key, Params: params}, true
		}
	}
	return Match{}, false
}

func (t *Table) Allowed(path string) []string {
	segs := segmentsOf(path)
	if len(segs) == 0 {
		return nil
	}
	var methods []string
	for _, c := range t.entries {
		if slices.Contains(methods, c.method) {
			continue
		}
		if _, ok := c.match(segs); ok {
			methods = append(methods, c.method)
		}
	}
	return methods
}
