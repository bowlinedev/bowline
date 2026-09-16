package bowline

import (
	"net/url"
	"strings"
)

func queryInput(rawQuery string) string {
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
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			continue
		}
		return decoded
	}
	return ""
}
