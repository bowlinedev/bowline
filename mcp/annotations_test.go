package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func listedTools(t *testing.T) map[string]listedTool {
	t.Helper()
	raw, err := testServer(t).Handle(context.Background(), json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Result struct {
			Tools []listedTool `json:"tools"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	if envelope.Error != nil {
		t.Fatalf("tools/list failed: %s", raw)
	}
	listing := envelope.Result
	out := map[string]listedTool{}
	for _, tool := range listing.Tools {
		out[tool.Name] = tool
	}
	return out
}

func TestEveryToolDeclaresAllFourHints(t *testing.T) {
	tools := listedTools(t)
	if len(tools) == 0 {
		t.Fatal("no tools were listed")
	}
	for name, tool := range tools {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(tool.Annotations)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			for _, hint := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
				value, present := fields[hint]
				if !present {
					t.Errorf("%s is missing; a host cannot warn a user about a hint that is absent, and a directory may reject the server", hint)
					continue
				}
				if _, ok := value.(bool); !ok {
					t.Errorf("%s is %T, want a boolean", hint, value)
				}
			}
		})
	}
}

func TestAnnotationsMatchTheDeclaration(t *testing.T) {
	tools := listedTools(t)
	byProcedure := map[string]Tool{}
	for _, tool := range testTools(t) {
		byProcedure[tool.Name] = tool
	}
	for name, listed := range tools {
		declared, ok := byProcedure[name]
		if !ok {
			t.Fatalf("listed tool %q has no declaration", name)
		}
		t.Run(name, func(t *testing.T) {
			if listed.Annotations.ReadOnlyHint != declared.ReadOnly {
				t.Errorf("readOnlyHint %v, want %v", listed.Annotations.ReadOnlyHint, declared.ReadOnly)
			}
			if listed.Annotations.DestructiveHint != declared.Destructive {
				t.Errorf("destructiveHint %v, want %v", listed.Annotations.DestructiveHint, declared.Destructive)
			}
			if declared.ReadOnly && !listed.Annotations.IdempotentHint {
				t.Error("a read-only tool is idempotent by definition, so the hint must be true")
			}
			if listed.Annotations.OpenWorldHint {
				t.Error("a procedure talks to this API only, which is a closed domain, so openWorldHint must be false")
			}
		})
	}
}

func TestReadOnlyToolsAreNeverDestructive(t *testing.T) {
	for name, tool := range listedTools(t) {
		if tool.Annotations.ReadOnlyHint && tool.Annotations.DestructiveHint {
			t.Errorf("%s claims to be read-only and destructive at once", name)
		}
	}
}
