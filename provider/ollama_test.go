package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fm-my-canvas/types"
)

func ollamaSSEHandler(t *testing.T, responses []ollamaToolResponse) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, resp := range responses {
			data, _ := json.Marshal(resp)
			w.Write(data)
			w.Write([]byte("\n"))
		}
	}
}

func TestOllamaStreamTextOnly(t *testing.T) {
	responses := []ollamaToolResponse{
		{Message: struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
		}{Role: "assistant", Content: "Hello "}, Done: false},
		{Message: struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
		}{Role: "assistant", Content: "world"}, Done: false},
		{Message: struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
		}{Role: "assistant", Content: ""}, Done: true},
	}

	server := httptest.NewServer(ollamaSSEHandler(t, responses))
	defer server.Close()

	p := NewOllama(server.URL, "test-model")

	var chunks []string
	err := p.Stream(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "hi"},
	}, func(chunk string) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks count = %d, want 2", len(chunks))
	}
	combined := strings.Join(chunks, "")
	if combined != "Hello world" {
		t.Errorf("combined = %q, want %q", combined, "Hello world")
	}
}

func TestOllamaStreamWithToolCalls(t *testing.T) {
	responses := []ollamaToolResponse{
		{
			Message: struct {
				Role      string           `json:"role"`
				Content   string           `json:"content"`
				ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
			}{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{
						ID: "call_1",
						Function: ollamaToolFunction{
							Name: "read_file",
							Arguments: map[string]any{
								"path": "index.html",
							},
						},
					},
				},
			},
			Done: true,
		},
	}

	server := httptest.NewServer(ollamaSSEHandler(t, responses))
	defer server.Close()

	p := NewOllama(server.URL, "test-model")

	var events []StreamEvent
	err := p.StreamWithTools(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "read index.html"},
	}, []ToolDefinition{}, func(event StreamEvent) {
		events = append(events, event)
	})

	if err != nil {
		t.Fatalf("StreamWithTools: %v", err)
	}

	hasToolCall := false
	hasDone := false
	for _, e := range events {
		if e.Type == EventToolCall {
			hasToolCall = true
			if len(e.ToolCalls) != 1 {
				t.Fatalf("ToolCalls count = %d, want 1", len(e.ToolCalls))
			}
			tc := e.ToolCalls[0]
			if tc.Name != "read_file" {
				t.Errorf("ToolCall Name = %q, want %q", tc.Name, "read_file")
			}
			if tc.ID != "call_1" {
				t.Errorf("ToolCall ID = %q, want %q", tc.ID, "call_1")
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				t.Fatalf("unmarshal arguments: %v", err)
			}
			if args["path"] != "index.html" {
				t.Errorf("arguments path = %v, want %q", args["path"], "index.html")
			}
		}
		if e.Type == EventDone {
			hasDone = true
		}
	}
	if !hasToolCall {
		t.Error("missing EventToolCall")
	}
	if !hasDone {
		t.Error("missing EventDone")
	}
}

func TestOllamaStreamWithEmptyToolCallID(t *testing.T) {
	responses := []ollamaToolResponse{
		{
			Message: struct {
				Role      string           `json:"role"`
				Content   string           `json:"content"`
				ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
			}{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{
						Function: ollamaToolFunction{
							Name: "list_files",
							Arguments: map[string]any{},
						},
					},
				},
			},
			Done: true,
		},
	}

	server := httptest.NewServer(ollamaSSEHandler(t, responses))
	defer server.Close()

	p := NewOllama(server.URL, "test-model")

	var events []StreamEvent
	err := p.StreamWithTools(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "list files"},
	}, []ToolDefinition{}, func(event StreamEvent) {
		events = append(events, event)
	})

	if err != nil {
		t.Fatalf("StreamWithTools: %v", err)
	}

	for _, e := range events {
		if e.Type == EventToolCall && len(e.ToolCalls) > 0 {
			if e.ToolCalls[0].ID == "" {
				t.Error("expected auto-generated tool call ID, got empty")
			}
			if !strings.HasPrefix(e.ToolCalls[0].ID, "ollama_tc_") {
				t.Errorf("expected auto-generated ID with prefix, got %q", e.ToolCalls[0].ID)
			}
		}
	}
}

func TestOllamaStreamTextAndToolMixed(t *testing.T) {
	responses := []ollamaToolResponse{
		{
			Message: struct {
				Role      string           `json:"role"`
				Content   string           `json:"content"`
				ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
			}{Role: "assistant", Content: "Let me read that file."},
		},
		{
			Message: struct {
				Role      string           `json:"role"`
				Content   string           `json:"content"`
				ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
			}{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{
						ID: "call_1",
						Function: ollamaToolFunction{
							Name:      "read_file",
							Arguments: map[string]any{"path": "test.txt"},
						},
					},
				},
			},
			Done: true,
		},
	}

	server := httptest.NewServer(ollamaSSEHandler(t, responses))
	defer server.Close()

	p := NewOllama(server.URL, "test-model")

	var events []StreamEvent
	err := p.StreamWithTools(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "read test.txt"},
	}, []ToolDefinition{}, func(event StreamEvent) {
		events = append(events, event)
	})

	if err != nil {
		t.Fatalf("StreamWithTools: %v", err)
	}

	hasContent := false
	hasToolCall := false
	for _, e := range events {
		if e.Type == EventContent && strings.Contains(e.Content, "Let me read") {
			hasContent = true
		}
		if e.Type == EventToolCall {
			hasToolCall = true
		}
	}
	if !hasContent {
		t.Error("missing content event")
	}
	if !hasToolCall {
		t.Error("missing tool call event")
	}
}
