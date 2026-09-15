package codec

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func Decode(data []byte, dst any, strict bool) error {
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	if !strict {
		return json.Unmarshal(data, dst)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after JSON value")
	}
	return nil
}
