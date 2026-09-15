package websocket

import "encoding/json"

type frame struct {
	ID    int64           `json:"id"`
	Type  string          `json:"type"`
	Path  string          `json:"path,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error json.RawMessage `json:"error,omitempty"`
}

type envelope struct {
	Error json.RawMessage `json:"error"`
}

func errorFrame(id int64, body []byte) frame {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil || len(env.Error) == 0 {
		return frame{ID: id, Type: "error", Error: json.RawMessage(`{"code":"INTERNAL","message":"internal error"}`)}
	}
	return frame{ID: id, Type: "error", Error: env.Error}
}
