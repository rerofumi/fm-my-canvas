package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fm-my-canvas/types"
)

func openrouterSSEHandler(t *testing.T, chunks []string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", chunk)
		}
	}
}

func TestOpenRouterStreamTextOnly(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"content":"Hi "},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"there"},"finish_reason":null}]}`,
		`[DONE]`,
	}

	server := httptest.NewServer(openrouterSSEHandler(t, chunks))
	defer server.Close()

	p := NewOpenRouter("test-key", "test-model")
	p.baseURL = server.URL

	var collected string
	err := p.Stream(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "hello"},
	}, func(chunk string) {
		collected += chunk
	})

	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if collected != "Hi there" {
		t.Errorf("collected = %q, want %q", collected, "Hi there")
	}
}

func TestOpenRouterStreamWithToolCalls(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"content":"","tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"read_file","arguments":""}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"","tool_calls":[{"index":0,"id":"","type":"","function":{"name":"","arguments":"{\"path\":"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"","tool_calls":[{"index":0,"id":"","type":"","function":{"name":"","arguments":"\"index.html\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`[DONE]`,
	}

	server := httptest.NewServer(openrouterSSEHandler(t, chunks))
	defer server.Close()

	p := NewOpenRouter("test-key", "test-model")
	p.baseURL = server.URL

	var events []StreamEvent
	err := p.StreamWithTools(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "read index"},
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
			if tc.ID != "call_abc" {
				t.Errorf("ID = %q, want %q", tc.ID, "call_abc")
			}
			if tc.Name != "read_file" {
				t.Errorf("Name = %q, want %q", tc.Name, "read_file")
			}
			expectedArgs := `{"path":"index.html"}`
			if tc.Arguments != expectedArgs {
				t.Errorf("Arguments = %q, want %q", tc.Arguments, expectedArgs)
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				t.Fatalf("arguments not valid JSON: %v", err)
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

func TestOpenRouterStreamMultipleToolCalls(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"content":"","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.txt\"}"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"","tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"b.txt\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`[DONE]`,
	}

	server := httptest.NewServer(openrouterSSEHandler(t, chunks))
	defer server.Close()

	p := NewOpenRouter("test-key", "test-model")
	p.baseURL = server.URL

	var events []StreamEvent
	err := p.StreamWithTools(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "read both"},
	}, []ToolDefinition{}, func(event StreamEvent) {
		events = append(events, event)
	})

	if err != nil {
		t.Fatalf("StreamWithTools: %v", err)
	}

	for _, e := range events {
		if e.Type == EventToolCall {
			if len(e.ToolCalls) != 2 {
				t.Fatalf("ToolCalls count = %d, want 2", len(e.ToolCalls))
			}
			if e.ToolCalls[0].Name != "read_file" || e.ToolCalls[1].Name != "read_file" {
				t.Errorf("tool names = %q, %q", e.ToolCalls[0].Name, e.ToolCalls[1].Name)
			}
			if e.ToolCalls[0].ID != "call_1" || e.ToolCalls[1].ID != "call_2" {
				t.Errorf("tool IDs = %q, %q", e.ToolCalls[0].ID, e.ToolCalls[1].ID)
			}
		}
	}
}

func TestOpenRouterStreamDeltaAccumulation(t *testing.T) {
	parts := []string{
		`{"path":"in`,
		`dex.html","co`,
		`ntent":"<h1>Hi</h1>"}`,
	}

	chunks := []string{
		fmt.Sprintf(`{"choices":[{"delta":{"content":"","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"write_file","arguments":""}}]},"finish_reason":null}]}`),
	}
	for i, part := range parts {
		escaped, _ := json.Marshal(part)
		isLast := i == len(parts)-1
		finishReason := "null"
		if isLast {
			finishReason = `"tool_calls"`
		}
		chunks = append(chunks, fmt.Sprintf(`{"choices":[{"delta":{"content":"","tool_calls":[{"index":0,"id":"","type":"","function":{"name":"","arguments":%s}}]},"finish_reason":%s}]}`, escaped, finishReason))
	}
	chunks = append(chunks, `[DONE]`)

	server := httptest.NewServer(openrouterSSEHandler(t, chunks))
	defer server.Close()

	p := NewOpenRouter("test-key", "test-model")
	p.baseURL = server.URL

	var events []StreamEvent
	err := p.StreamWithTools(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "write file"},
	}, []ToolDefinition{}, func(event StreamEvent) {
		events = append(events, event)
	})

	if err != nil {
		t.Fatalf("StreamWithTools: %v", err)
	}

	for _, e := range events {
		if e.Type == EventToolCall && len(e.ToolCalls) > 0 {
			expected := `{"path":"index.html","content":"<h1>Hi</h1>"}`
			if e.ToolCalls[0].Arguments != expected {
				t.Errorf("accumulated arguments = %q, want %q", e.ToolCalls[0].Arguments, expected)
			}
		}
	}
}

func TestOpenRouterStreamContentAndToolCall(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"content":"Reading file...","tool_calls":[]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"test.txt\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`[DONE]`,
	}

	server := httptest.NewServer(openrouterSSEHandler(t, chunks))
	defer server.Close()

	p := NewOpenRouter("test-key", "test-model")
	p.baseURL = server.URL

	var events []StreamEvent
	err := p.StreamWithTools(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "read"},
	}, []ToolDefinition{}, func(event StreamEvent) {
		events = append(events, event)
	})

	if err != nil {
		t.Fatalf("StreamWithTools: %v", err)
	}

	hasContent := false
	hasToolCall := false
	for _, e := range events {
		if e.Type == EventContent {
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

func TestOpenRouterStreamError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	p := NewOpenRouter("test-key", "test-model")
	p.baseURL = server.URL

	err := p.Stream(context.Background(), []types.Message{
		{Role: types.RoleUser, Content: "test"},
	}, func(chunk string) {})

	if err == nil {
		t.Fatal("expected error for bad status")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want 400 status", err.Error())
	}
}

func TestToOpenRouterMessages(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleSystem, Content: "system prompt"},
		{Role: types.RoleUser, Content: "hello"},
		{Role: types.RoleAssistant, Content: "", ToolCalls: []types.ToolCall{
			{ID: "tc1", Name: "read_file", Arguments: `{"path":"x"}`},
		}},
		{Role: types.RoleTool, Content: "file content", ToolCallID: "tc1"},
		{Role: types.RoleAssistant, Content: "here's the file"},
		{Role: types.RoleTool, Content: "", ToolCallID: ""},
	}

	result := toOpenRouterMessages(messages)

	if len(result) != 5 {
		t.Fatalf("messages count = %d, want 5 (empty tool msg filtered)", len(result))
	}

	if result[2].Role != "assistant" {
		t.Errorf("msg[2] role = %q, want assistant", result[2].Role)
	}
	if len(result[2].ToolCalls) != 1 {
		t.Errorf("msg[2] tool_calls = %d, want 1", len(result[2].ToolCalls))
	}

	if result[3].Role != "tool" {
		t.Errorf("msg[3] role = %q, want tool", result[3].Role)
	}
	if result[3].ToolCallID != "tc1" {
		t.Errorf("msg[3] tool_call_id = %q, want tc1", result[3].ToolCallID)
	}
}
