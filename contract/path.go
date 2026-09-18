package contract

import (
	"errors"
	"fmt"
	"strings"
)

type PathSegment struct {
	Text  string
	Param bool
}

func ParsePath(template string) ([]PathSegment, error) {
	if template == "" {
		return nil, errors.New("path is empty")
	}
	if strings.HasPrefix(template, "/") || strings.HasSuffix(template, "/") {
		return nil, fmt.Errorf("path %q must not start or end with a slash", template)
	}
	parts := strings.Split(template, "/")
	segments := make([]PathSegment, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("path %q has an empty segment", template)
		}
		open := strings.IndexByte(part, '{')
		shut := strings.IndexByte(part, '}')
		if open < 0 && shut < 0 {
			if strings.ContainsAny(part, " \t?#") {
				return nil, fmt.Errorf("path %q has a segment with a space or a reserved character", template)
			}
			segments = append(segments, PathSegment{Text: part})
			continue
		}
		if open != 0 || shut != len(part)-1 {
			return nil, fmt.Errorf("path %q must wrap a whole segment in braces, as in {id}", template)
		}
		name := part[1 : len(part)-1]
		if name == "" {
			return nil, fmt.Errorf("path %q has an unnamed parameter", template)
		}
		if strings.ContainsAny(name, "{}/ \t?#") {
			return nil, fmt.Errorf("path %q has a malformed parameter %q", template, name)
		}
		if seen[name] {
			return nil, fmt.Errorf("path %q repeats the parameter %q", template, name)
		}
		seen[name] = true
		segments = append(segments, PathSegment{Text: name, Param: true})
	}
	return segments, nil
}

func PathParams(template string) ([]string, error) {
	segments, err := ParsePath(template)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, s := range segments {
		if s.Param {
			names = append(names, s.Text)
		}
	}
	return names, nil
}
