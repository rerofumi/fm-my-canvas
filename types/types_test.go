package types

import (
	"encoding/json"
	"testing"
)

func TestRoleTool(t *testing.T) {
	if RoleTool != "tool" {
		t.Errorf("RoleTool = %q, want %q", RoleTool, "tool")
	}
}

func TestMessageMarshalUnmarshal(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "hello",
		ToolCalls: []ToolCall{
			{ID: "tc1", Name: "read_file", Arguments: `{"path":"index.html"}`},
		},
		ToolCallID: "tc1",
		CreatedAt:  "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Role != msg.Role {
		t.Errorf("Role = %q, want %q", got.Role, msg.Role)
	}
	if got.Content != msg.Content {
		t.Errorf("Content = %q, want %q", got.Content, msg.Content)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(got.ToolCalls))
	}
	if got.ToolCalls[0].ID != "tc1" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", got.ToolCalls[0].ID, "tc1")
	}
	if got.ToolCalls[0].Name != "read_file" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", got.ToolCalls[0].Name, "read_file")
	}
	if got.ToolCalls[0].Arguments != `{"path":"index.html"}` {
		t.Errorf("ToolCalls[0].Arguments = %q, want %q", got.ToolCalls[0].Arguments, `{"path":"index.html"}`)
	}
	if got.ToolCallID != "tc1" {
		t.Errorf("ToolCallID = %q, want %q", got.ToolCallID, "tc1")
	}
	if got.CreatedAt != msg.CreatedAt {
		t.Errorf("CreatedAt = %q, want %q", got.CreatedAt, msg.CreatedAt)
	}
}

func TestToolRoleMessageMarshal(t *testing.T) {
	msg := Message{
		Role:       RoleTool,
		Content:    "file contents here",
		ToolCallID: "tc_abc",
		CreatedAt:  "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Role != RoleTool {
		t.Errorf("Role = %q, want %q", got.Role, RoleTool)
	}
	if got.ToolCallID != "tc_abc" {
		t.Errorf("ToolCallID = %q, want %q", got.ToolCallID, "tc_abc")
	}
	if len(got.ToolCalls) != 0 {
		t.Errorf("ToolCalls should be empty, got %d", len(got.ToolCalls))
	}
}

func TestOmitEmpty(t *testing.T) {
	msg := Message{
		Role:      RoleAssistant,
		Content:   "response",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	if _, ok := raw["tool_calls"]; ok {
		t.Error("tool_calls should be omitted when empty")
	}
	if _, ok := raw["tool_call_id"]; ok {
		t.Error("tool_call_id should be omitted when empty")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	oldJSON := `{"role":"user","content":"hello","created_at":"2026-01-01T00:00:00Z"}`

	var msg Message
	if err := json.Unmarshal([]byte(oldJSON), &msg); err != nil {
		t.Fatalf("unmarshal old format: %v", err)
	}

	if msg.Role != RoleUser {
		t.Errorf("Role = %q, want %q", msg.Role, RoleUser)
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello")
	}
	if len(msg.ToolCalls) != 0 {
		t.Errorf("ToolCalls should be nil, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCallID != "" {
		t.Errorf("ToolCallID should be empty, got %q", msg.ToolCallID)
	}
}

func TestToolCallMarshal(t *testing.T) {
	tc := ToolCall{
		ID:        "call_123",
		Name:      "write_file",
		Arguments: `{"path":"test.txt","content":"hello"}`,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ToolCall
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != tc.ID {
		t.Errorf("ID = %q, want %q", got.ID, tc.ID)
	}
	if got.Name != tc.Name {
		t.Errorf("Name = %q, want %q", got.Name, tc.Name)
	}
	if got.Arguments != tc.Arguments {
		t.Errorf("Arguments = %q, want %q", got.Arguments, tc.Arguments)
	}
}

func TestSessionMarshalWithToolMessages(t *testing.T) {
	s := Session{
		ID:        "sess-1",
		Title:     "Test",
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
		Messages: []Message{
			{Role: RoleUser, Content: "read index.html", CreatedAt: "2026-01-01T00:00:00Z"},
			{Role: RoleAssistant, Content: "", ToolCalls: []ToolCall{
				{ID: "tc1", Name: "read_file", Arguments: `{"path":"index.html"}`},
			}, CreatedAt: "2026-01-01T00:00:01Z"},
			{Role: RoleTool, Content: "<html></html>", ToolCallID: "tc1", CreatedAt: "2026-01-01T00:00:02Z"},
			{Role: RoleAssistant, Content: "Here's the file content.", CreatedAt: "2026-01-01T00:00:03Z"},
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}

	var got Session
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal session: %v", err)
	}

	if len(got.Messages) != 4 {
		t.Fatalf("Messages len = %d, want 4", len(got.Messages))
	}
	if got.Messages[1].Role != RoleAssistant {
		t.Errorf("Messages[1].Role = %q, want assistant", got.Messages[1].Role)
	}
	if len(got.Messages[1].ToolCalls) != 1 {
		t.Errorf("Messages[1].ToolCalls len = %d, want 1", len(got.Messages[1].ToolCalls))
	}
	if got.Messages[2].Role != RoleTool {
		t.Errorf("Messages[2].Role = %q, want tool", got.Messages[2].Role)
	}
	if got.Messages[2].ToolCallID != "tc1" {
		t.Errorf("Messages[2].ToolCallID = %q, want %q", got.Messages[2].ToolCallID, "tc1")
	}
}
