package tools

import (
	"context"
	"testing"
	"time"

	"fm-my-canvas/types"
)

type mockTool struct {
	name    string
	desc    string
	params  map[string]any
	execute func(sessionID string, args map[string]any) (string, error)
}

func (m *mockTool) Name() string                          { return m.name }
func (m *mockTool) Description() string                   { return m.desc }
func (m *mockTool) Parameters() map[string]any            { return m.params }
func (m *mockTool) Execute(sessionID string, args map[string]any) (string, error) {
	return m.execute(sessionID, args)
}

func TestRegisterAndTools(t *testing.T) {
	tm := NewToolManager()

	t1 := &mockTool{name: "tool_a", desc: "Tool A"}
	t2 := &mockTool{name: "tool_b", desc: "Tool B"}

	tm.Register(t1)
	tm.Register(t2)

	tools := tm.Tools()
	if len(tools) != 2 {
		t.Fatalf("Tools() count = %d, want 2", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	if !names["tool_a"] || !names["tool_b"] {
		t.Errorf("expected tool_a and tool_b, got %v", names)
	}
}

func TestExecuteSuccess(t *testing.T) {
	tm := NewToolManager()
	tm.Register(&mockTool{
		name: "echo",
		execute: func(sessionID string, args map[string]any) (string, error) {
			return args["msg"].(string), nil
		},
	})

	tc := types.ToolCall{
		ID:        "tc1",
		Name:      "echo",
		Arguments: `{"msg":"hello"}`,
	}

	result, err := tm.Execute("sess-1", tc)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "hello" {
		t.Errorf("result = %q, want %q", result, "hello")
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	tm := NewToolManager()

	tc := types.ToolCall{
		ID:        "tc1",
		Name:      "nonexistent",
		Arguments: `{}`,
	}

	_, err := tm.Execute("sess-1", tc)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestExecuteInvalidJSON(t *testing.T) {
	tm := NewToolManager()
	tm.Register(&mockTool{
		name: "test",
		execute: func(sessionID string, args map[string]any) (string, error) {
			return "ok", nil
		},
	})

	tc := types.ToolCall{
		ID:        "tc1",
		Name:      "test",
		Arguments: `{invalid json`,
	}

	_, err := tm.Execute("sess-1", tc)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExecuteEmptyArguments(t *testing.T) {
	tm := NewToolManager()
	tm.Register(&mockTool{
		name: "test",
		execute: func(sessionID string, args map[string]any) (string, error) {
			return "empty", nil
		},
	})

	tc := types.ToolCall{
		ID:        "tc1",
		Name:      "test",
		Arguments: "",
	}

	result, err := tm.Execute("sess-1", tc)
	if err != nil {
		t.Fatalf("Execute with empty args: %v", err)
	}
	if result != "empty" {
		t.Errorf("result = %q, want %q", result, "empty")
	}
}

func TestExecuteWithContext(t *testing.T) {
	tm := NewToolManager()
	tm.Register(&mockTool{
		name: "slow",
		execute: func(sessionID string, args map[string]any) (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "done", nil
		},
	})

	ctx := context.Background()
	tc := types.ToolCall{
		ID:        "tc1",
		Name:      "slow",
		Arguments: `{}`,
	}

	result, err := tm.ExecuteWithContext(ctx, "sess-1", tc)
	if err != nil {
		t.Fatalf("ExecuteWithContext: %v", err)
	}
	if result != "done" {
		t.Errorf("result = %q, want %q", result, "done")
	}
}

func TestExecuteWithContextCancellation(t *testing.T) {
	tm := NewToolManager()
	tm.Register(&mockTool{
		name: "blocked",
		execute: func(sessionID string, args map[string]any) (string, error) {
			time.Sleep(5 * time.Second)
			return "done", nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tc := types.ToolCall{
		ID:        "tc1",
		Name:      "blocked",
		Arguments: `{}`,
	}

	_, err := tm.ExecuteWithContext(ctx, "sess-1", tc)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestExecuteWithContextTimeoutWhileRunning(t *testing.T) {
	tm := NewToolManager()
	tm.Register(&mockTool{
		name: "blocked",
		execute: func(sessionID string, args map[string]any) (string, error) {
			time.Sleep(200 * time.Millisecond)
			return "done", nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	tc := types.ToolCall{
		ID:        "tc1",
		Name:      "blocked",
		Arguments: `{}`,
	}

	start := time.Now()
	_, err := tm.ExecuteWithContext(ctx, "sess-1", tc)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error while tool is running")
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("ExecuteWithContext returned too slowly: %v", elapsed)
	}
}
