package agent

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/bowlinedev/bowline"
)

type Tracer interface {
	Start(ctx context.Context, call Call) (context.Context, func(Result))
}

type RecordingTracer struct {
	mu  sync.Mutex
	out io.Writer
	now func() time.Time
}

func NewRecordingTracer(w io.Writer) *RecordingTracer {
	return &RecordingTracer{out: w, now: time.Now}
}

type recordedLine struct {
	ID         string          `json:"id"`
	Tool       string          `json:"tool"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      *recordedError  `json:"error,omitempty"`
	DurationMs int64           `json:"durationMs"`
}

type recordedError struct {
	Code    bowline.Code    `json:"code"`
	Message string          `json:"message"`
	Issues  []bowline.Issue `json:"issues,omitempty"`
}

func (t *RecordingTracer) Start(ctx context.Context, call Call) (context.Context, func(Result)) {
	start := t.now()
	return ctx, func(r Result) {
		line := recordedLine{ID: call.ID, Tool: call.Tool, Input: call.Input, Output: r.Output, DurationMs: t.now().Sub(start).Milliseconds()}
		if len(line.Input) == 0 {
			line.Input = json.RawMessage("{}")
		}
		if r.Error != nil {
			line.Error = &recordedError{Code: r.Error.Code, Message: r.Error.Message, Issues: r.Error.Issues}
		}
		data, err := json.Marshal(line)
		if err != nil {
			return
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		_, _ = t.out.Write(append(data, '\n'))
	}
}
