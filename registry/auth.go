package registry

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) authorized(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return false
	}
	match := false
	for _, want := range s.tokens {
		if want == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1 {
			match = true
		}
	}
	return match
}
