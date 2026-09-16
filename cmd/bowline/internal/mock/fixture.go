package mock

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/bowlinedev/bowline/contract"
)

type Fixture struct {
	Bowline    string          `json:"bowline"`
	Procedure  string          `json:"procedure"`
	RecordedAt string          `json:"recordedAt"`
	Request    FixtureRequest  `json:"request"`
	Response   FixtureResponse `json:"response"`
}

type FixtureRequest struct {
	Method string          `json:"method"`
	Input  json.RawMessage `json:"input"`
}

type FixtureResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body"`
}

func CanonicalInput(raw []byte) ([]byte, string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(sum[:]), nil
}

func FixturePath(procedure, hash string) string {
	return path.Join(procedure, hash+".json")
}

func Encode(fx *Fixture) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (fx *Fixture) serve(w http.ResponseWriter) {
	names := slices.Sorted(maps.Keys(fx.Response.Headers))
	for _, name := range names {
		w.Header().Set(name, fx.Response.Headers[name])
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	status := fx.Response.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	var compact bytes.Buffer
	if json.Compact(&compact, fx.Response.Body) == nil {
		w.Write(compact.Bytes())
		return
	}
	w.Write(fx.Response.Body)
}

type fixtureSet struct {
	byKey map[string]*Fixture
}

func loadFixtures(fsys fs.FS) (*fixtureSet, error) {
	set := &fixtureSet{byKey: map[string]*Fixture{}}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		var fx Fixture
		if err := json.Unmarshal(data, &fx); err != nil {
			return err
		}
		if fx.Procedure == "" {
			return nil
		}
		_, hash, err := CanonicalInput(fx.Request.Input)
		if err != nil {
			return err
		}
		set.byKey[fx.Procedure+"/"+hash] = &fx
		return nil
	})
	if err != nil {
		return set, err
	}
	return set, nil
}

func (s *fixtureSet) lookup(procedure string, raw []byte) (*Fixture, bool) {
	_, hash, err := CanonicalInput(raw)
	if err != nil {
		return nil, false
	}
	fx, ok := s.byKey[procedure+"/"+hash]
	return fx, ok
}

func NewFixture(p *contract.Procedure, input []byte, status int, headers http.Header, body []byte, recordedAt string) (*Fixture, string, error) {
	canonical, hash, err := CanonicalInput(input)
	if err != nil {
		return nil, "", err
	}
	kept := map[string]string{}
	for _, name := range []string{"Deprecation", "Sunset", "Content-Type"} {
		if v := headers.Get(name); v != "" {
			kept[name] = v
		}
	}
	if len(kept) == 0 {
		kept = nil
	}
	if !json.Valid(body) {
		body, _ = json.Marshal(string(body))
	}
	fx := &Fixture{
		Bowline:    contract.Version,
		Procedure:  p.Path,
		RecordedAt: recordedAt,
		Request:    FixtureRequest{Method: p.Method, Input: canonical},
		Response:   FixtureResponse{Status: status, Headers: kept, Body: body},
	}
	return fx, hash, nil
}
