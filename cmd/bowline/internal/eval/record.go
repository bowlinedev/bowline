package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/bowlinedev/bowline/cmd/bowline/internal/tools"
	"github.com/bowlinedev/bowline/contract"
	"github.com/bowlinedev/bowline/mcp"
)

const Version = 1

type Recording struct {
	Bowline    string    `json:"bowline"`
	Eval       int       `json:"eval"`
	Contract   string    `json:"contract"`
	RecordedAt time.Time `json:"recordedAt"`
	Volatile   []string  `json:"volatile"`
	Steps      []Step    `json:"steps"`
}

type Step struct {
	ID         string          `json:"id"`
	Tool       string          `json:"tool"`
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      *StepError      `json:"error,omitempty"`
	IsError    bool            `json:"isError"`
	DurationMs int64           `json:"durationMs"`
}

type StepError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Type    string          `json:"type,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
	Issues  json.RawMessage `json:"issues,omitempty"`
}

type Call struct {
	ID    string          `json:"id"`
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}

type Script struct {
	Volatile []string `json:"volatile"`
	Calls    []Call   `json:"calls"`
}

type Runner struct {
	Dispatcher mcp.Dispatcher
	Tools      []tools.Tool
	Contract   string
}

func (r *Runner) Record(ctx context.Context, calls []Call, volatile []string, now func() time.Time) (*Recording, error) {
	if now == nil {
		now = time.Now
	}
	sorted := append([]string(nil), volatile...)
	slices.Sort(sorted)
	if sorted == nil {
		sorted = []string{}
	}
	rec := &Recording{Bowline: contract.Version, Eval: Version, Contract: r.Contract, RecordedAt: now().UTC().Truncate(time.Second), Volatile: sorted, Steps: []Step{}}
	for i, call := range calls {
		step, err := r.run(ctx, call, i, sorted)
		if err != nil {
			return nil, err
		}
		rec.Steps = append(rec.Steps, step)
	}
	return rec, nil
}

func (r *Runner) Replay(ctx context.Context, rec *Recording, strictMessages bool) ([]Mismatch, error) {
	if rec.Eval != Version {
		return nil, fmt.Errorf("recording format %d is not supported; this bowline replays format %d", rec.Eval, Version)
	}
	if rec.Contract != r.Contract {
		return nil, fmt.Errorf("recording was made against contract %s but the current contract is %s; re-record after an intentional API change", rec.Contract, r.Contract)
	}
	var mismatches []Mismatch
	for i, recorded := range rec.Steps {
		live, err := r.run(ctx, Call{ID: recorded.ID, Tool: recorded.Tool, Input: recorded.Input}, i, rec.Volatile)
		if err != nil {
			return nil, err
		}
		prefix := fmt.Sprintf("/steps/%d", i)
		if recorded.IsError != live.IsError {
			mismatches = append(mismatches, Mismatch{recorded.ID, prefix + "/isError", render(recorded.IsError), render(live.IsError)})
			continue
		}
		if recorded.IsError {
			if recorded.Error.Code != live.Error.Code {
				mismatches = append(mismatches, Mismatch{recorded.ID, prefix + "/error/code", recorded.Error.Code, live.Error.Code})
			}
			if strictMessages && recorded.Error.Message != live.Error.Message {
				mismatches = append(mismatches, Mismatch{recorded.ID, prefix + "/error/message", recorded.Error.Message, live.Error.Message})
			}
			continue
		}
		want, err := decode(Normalize(recorded.Output, rec.Volatile))
		if err != nil {
			return nil, fmt.Errorf("step %s: recorded output is not JSON: %w", recorded.ID, err)
		}
		got, err := decode(live.Output)
		if err != nil {
			return nil, fmt.Errorf("step %s: live output is not JSON: %w", recorded.ID, err)
		}
		compare(recorded.ID, prefix+"/output", want, got, &mismatches)
	}
	return mismatches, nil
}

func (r *Runner) run(ctx context.Context, call Call, index int, volatile []string) (Step, error) {
	id := call.ID
	if id == "" {
		id = fmt.Sprint(index + 1)
	}
	input := call.Input
	if len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage("{}")
	}
	step := Step{ID: id, Tool: call.Tool, Input: Normalize(input, nil)}
	tool, ok := r.tool(call.Tool)
	if !ok {
		return step, fmt.Errorf("step %s: unknown tool %q", id, call.Tool)
	}
	started := time.Now()
	status, body, err := r.Dispatcher.Dispatch(ctx, tool.Procedure, tool.Method, input, http.Header{})
	step.DurationMs = time.Since(started).Milliseconds()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return step, err
		}
		step.IsError = true
		step.Error = &StepError{Code: "UNAVAILABLE", Message: err.Error()}
		return step, nil
	}
	if status >= 200 && status < 300 {
		step.Output = Normalize(body, volatile)
		if step.Output == nil {
			step.Output = json.RawMessage("null")
		}
		return step, nil
	}
	step.IsError = true
	step.Error = parseError(status, body)
	return step, nil
}

func (r *Runner) tool(name string) (tools.Tool, bool) {
	for _, t := range r.Tools {
		if t.Name == name {
			return t, true
		}
	}
	return tools.Tool{}, false
}

func parseError(status int, body []byte) *StepError {
	var envelope struct {
		Error *StepError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error == nil || envelope.Error.Code == "" {
		return &StepError{Code: "UNKNOWN", Message: fmt.Sprintf("upstream returned HTTP %d", status)}
	}
	envelope.Error.Details = Normalize(envelope.Error.Details, nil)
	envelope.Error.Issues = Normalize(envelope.Error.Issues, nil)
	return envelope.Error
}

func Encode(rec *Recording) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Parse(data []byte) (*Recording, error) {
	var rec Recording
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parsing recording: %w", err)
	}
	for i, step := range rec.Steps {
		if step.IsError && step.Error == nil {
			return nil, fmt.Errorf("step %d is marked isError without an error", i)
		}
	}
	return &rec, nil
}

func ParseScript(data []byte) (*Script, error) {
	var script Script
	if err := json.Unmarshal(data, &script); err != nil {
		return nil, fmt.Errorf("parsing script: %w", err)
	}
	if len(script.Calls) == 0 {
		return nil, errors.New("script has no calls")
	}
	return &script, nil
}

func ParseCalls(data []byte) ([]Call, error) {
	var calls []Call
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var call Call
		if err := dec.Decode(&call); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parsing call stream: %w", err)
		}
		if call.Tool == "" {
			return nil, errors.New("parsing call stream: every line needs a tool")
		}
		calls = append(calls, call)
	}
	if len(calls) == 0 {
		return nil, errors.New("no calls on stdin")
	}
	return calls, nil
}
