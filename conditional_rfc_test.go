package bowline

import "testing"

func TestIfMatchUsesStrongComparison(t *testing.T) {
	cases := []struct {
		name       string
		candidates []string
		tag        string
		want       bool
	}{
		{"strong matches strong", []string{`"a"`}, `"a"`, true},
		{"weak candidate never matches", []string{`W/"a"`}, `"a"`, false},
		{"weak server tag never matches", []string{`"a"`}, `W/"a"`, false},
		{"weak both never matches", []string{`W/"a"`}, `W/"a"`, false},
		{"star matches a strong tag", []string{"*"}, `"a"`, true},
		{"star matches a weak tag", []string{"*"}, `W/"a"`, true},
		{"one of several", []string{`"x"`, `"a"`}, `"a"`, true},
		{"no match", []string{`"x"`}, `"a"`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := matchesETagStrongly(c.candidates, c.tag); got != c.want {
				t.Fatalf("matchesETagStrongly(%v, %q) = %v, want %v", c.candidates, c.tag, got, c.want)
			}
		})
	}
}

func TestIfNoneMatchUsesWeakComparison(t *testing.T) {
	cases := []struct {
		name       string
		candidates []string
		tag        string
		want       bool
	}{
		{"weak candidate matches strong", []string{`W/"a"`}, `"a"`, true},
		{"strong candidate matches weak", []string{`"a"`}, `W/"a"`, true},
		{"weak matches weak", []string{`W/"a"`}, `W/"a"`, true},
		{"star matches", []string{"*"}, `W/"a"`, true},
		{"different opaque values", []string{`W/"b"`}, `"a"`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := matchesETag(c.candidates, c.tag); got != c.want {
				t.Fatalf("matchesETag(%v, %q) = %v, want %v", c.candidates, c.tag, got, c.want)
			}
		})
	}
}
