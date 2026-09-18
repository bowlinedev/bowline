package bowline

import (
	"net/url"
	"strings"
)

func queryInput(rawQuery string) []byte {
	for rawQuery != "" {
		var pair string
		pair, rawQuery, _ = strings.Cut(rawQuery, "&")
		if pair == "" || strings.Contains(pair, ";") {
			continue
		}
		key, value, _ := strings.Cut(pair, "=")
		if key != "input" {
			decoded, err := url.QueryUnescape(key)
			if err != nil || decoded != "input" {
				continue
			}
		}
		decoded, ok := unescapeBytes(value)
		if !ok {
			continue
		}
		return decoded
	}
	return nil
}

func unescapeBytes(value string) ([]byte, bool) {
	if strings.IndexByte(value, '%') < 0 && strings.IndexByte(value, '+') < 0 {
		return []byte(value), true
	}
	out := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		switch c := value[i]; c {
		case '%':
			if i+2 >= len(value) {
				return nil, false
			}
			hi, ok := unhex(value[i+1])
			if !ok {
				return nil, false
			}
			lo, ok := unhex(value[i+2])
			if !ok {
				return nil, false
			}
			out = append(out, hi<<4|lo)
			i += 2
		case '+':
			out = append(out, ' ')
		default:
			out = append(out, c)
		}
	}
	return out, true
}

func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
