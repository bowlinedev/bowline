package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
)

func Normalize(v json.RawMessage, volatile []string) json.RawMessage {
	if len(bytes.TrimSpace(v)) == 0 {
		return nil
	}
	value, err := decode(v)
	if err != nil {
		return v
	}
	skip := map[string]bool{}
	for _, key := range volatile {
		skip[key] = true
	}
	out, err := json.Marshal(strip(value, skip))
	if err != nil {
		return v
	}
	return out
}

func decode(v json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(v))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func strip(value any, skip map[string]bool) any {
	switch t := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, v := range t {
			if skip[k] {
				continue
			}
			out[k] = strip(v, skip)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, v := range t {
			out[i] = strip(v, skip)
		}
		return out
	case json.Number:
		return canonicalNumber(t)
	default:
		return value
	}
}

func canonicalNumber(n json.Number) json.Number {
	if i, ok := new(big.Int).SetString(n.String(), 10); ok {
		return json.Number(i.String())
	}
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil {
		return n
	}
	return json.Number(strconv.FormatFloat(f, 'g', -1, 64))
}

type Mismatch struct {
	Step string `json:"step"`
	Path string `json:"path"`
	Want string `json:"want"`
	Got  string `json:"got"`
}

func (m Mismatch) String() string {
	return fmt.Sprintf("step %s: %s: recorded %s, got %s", m.Step, m.Path, m.Want, m.Got)
}

func compare(step, path string, want, got any, out *[]Mismatch) {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			*out = append(*out, Mismatch{step, path, render(want), render(got)})
			return
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			wv, inWant := w[k]
			gv, inGot := g[k]
			child := path + "/" + escapePointer(k)
			switch {
			case !inWant:
				*out = append(*out, Mismatch{step, child, "absent", render(gv)})
			case !inGot:
				*out = append(*out, Mismatch{step, child, render(wv), "absent"})
			default:
				compare(step, child, wv, gv, out)
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			*out = append(*out, Mismatch{step, path, render(want), render(got)})
			return
		}
		for i := range w {
			compare(step, path+"/"+strconv.Itoa(i), w[i], g[i], out)
		}
	default:
		if render(want) != render(got) {
			*out = append(*out, Mismatch{step, path, render(want), render(got)})
		}
	}
}

func render(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

func escapePointer(key string) string {
	out := make([]byte, 0, len(key))
	for i := 0; i < len(key); i++ {
		switch key[i] {
		case '~':
			out = append(out, '~', '0')
		case '/':
			out = append(out, '~', '1')
		default:
			out = append(out, key[i])
		}
	}
	return string(out)
}
