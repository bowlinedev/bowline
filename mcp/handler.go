package mcp

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/contract"
)

const maxMessageSize = 4 << 20

func Handler(r *bowline.Router, doc []byte, opts ...Option) (http.Handler, error) {
	parsed, err := contract.Parse(doc)
	if err != nil {
		return nil, err
	}
	source, err := SchemasFromContract(parsed)
	if err != nil {
		return nil, err
	}
	tools, err := ToolsFromContract(parsed, source)
	if err != nil {
		return nil, err
	}
	o := newOptions(opts)
	server := NewServer(tools, RouterDispatcher(r, o.runtime...), opts...)
	return Serve(server, opts...), nil
}

func Serve(server *Server, opts ...Option) http.Handler {
	o := newOptions(opts)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, maxMessageSize))
		if err != nil {
			http.Error(w, "request body too large or unreadable", http.StatusRequestEntityTooLarge)
			return
		}
		headers := http.Header{}
		for _, name := range o.forward {
			for _, v := range req.Header.Values(name) {
				headers.Add(name, v)
			}
		}
		ctx := req.Context()
		trimmed := trimLeft(body)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			var batch []json.RawMessage
			if err := json.Unmarshal(trimmed, &batch); err != nil {
				writeJSON(w, errorResponse(nil, codeParse, "parse error: "+err.Error()))
				return
			}
			responses := make([]json.RawMessage, 0, len(batch))
			for _, msg := range batch {
				resp, err := server.HandleWithHeaders(ctx, msg, headers)
				if err != nil {
					o.logger.ErrorContext(ctx, "mcp: encode response", "error", err)
					continue
				}
				if resp != nil {
					responses = append(responses, resp)
				}
			}
			if len(responses) == 0 {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			writeJSON(w, responses)
			return
		}
		resp, err := server.HandleWithHeaders(ctx, body, headers)
		if err != nil {
			writeJSON(w, errorResponse(nil, codeInternal, err.Error()))
			return
		}
		if resp == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	})
}

func trimLeft(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}

func writeJSON(w http.ResponseWriter, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
