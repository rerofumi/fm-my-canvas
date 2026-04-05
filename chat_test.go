package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"fm-my-canvas/artifacts"
	"fm-my-canvas/types"
)

type ollamaChatTestRequest struct {
	Messages []struct {
		Role       string `json:"role"`
		Content    string `json:"content"`
		ToolCallID string `json:"tool_call_id,omitempty"`
	} `json:"messages"`
}

func newAgentModeChatServiceForTest(t *testing.T) (*ChatService, *artifacts.Manager) {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	artifactMgr := artifacts.NewManagerWithDir(filepath.Join(homeDir, "artifacts"))
	server := artifacts.NewServer()

	svc, err := NewChatService(artifactMgr, server)
	if err != nil {
		t.Fatalf("NewChatService: %v", err)
	}
	svc.SetContext(context.Background())
	return svc, artifactMgr
}

func writeOllamaToolResponse(t *testing.T, w http.ResponseWriter, resp providerTestOllamaResponse) {
	t.Helper()
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal ollama response: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write ollama response: %v", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		t.Fatalf("write newline: %v", err)
	}
}

type providerTestOllamaResponse struct {
	Message struct {
		Role      string                    `json:"role"`
		Content   string                    `json:"content"`
		ToolCalls []providerTestOllamaCall  `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done bool `json:"done"`
}

type providerTestOllamaCall struct {
	ID       string                        `json:"id,omitempty"`
	Function providerTestOllamaFunction    `json:"function"`
}

type providerTestOllamaFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func TestParseArtifacts(t *testing.T) {
	text := "Here's your page:\n\n```html path=index.html\n<!DOCTYPE html>\n<html></html>\n```\n\nAnd some CSS:\n\n```css path=style.css\nbody { margin: 0; }\n```"

	files := parseArtifacts(text)
	if len(files) != 2 {
		t.Fatalf("files count = %d, want 2", len(files))
	}

	if files[0].Path != "index.html" {
		t.Errorf("files[0].Path = %q, want %q", files[0].Path, "index.html")
	}
	if !strings.Contains(files[0].Content, "<!DOCTYPE html>") {
		t.Errorf("files[0].Content missing expected content: %q", files[0].Content)
	}
	if files[1].Path != "style.css" {
		t.Errorf("files[1].Path = %q, want %q", files[1].Path, "style.css")
	}
}

func TestParseArtifactsDefaultPath(t *testing.T) {
	text := "```html\n<html></html>\n```"

	files := parseArtifacts(text)
	if len(files) != 1 {
		t.Fatalf("files count = %d, want 1", len(files))
	}
	if files[0].Path != "index.html" {
		t.Errorf("Path = %q, want %q", files[0].Path, "index.html")
	}
}

func TestParseArtifactsCSS(t *testing.T) {
	text := "```css\nbody {}\n```"

	files := parseArtifacts(text)
	if len(files) != 1 {
		t.Fatalf("files count = %d, want 1", len(files))
	}
	if files[0].Path != "style.css" {
		t.Errorf("Path = %q, want %q", files[0].Path, "style.css")
	}
}

func TestParseArtifactsJS(t *testing.T) {
	text := "```javascript\nconsole.log('hi');\n```"

	files := parseArtifacts(text)
	if len(files) != 1 {
		t.Fatalf("files count = %d, want 1", len(files))
	}
	if files[0].Path != "script.js" {
		t.Errorf("Path = %q, want %q", files[0].Path, "script.js")
	}
}

func TestParseArtifactsNoMatch(t *testing.T) {
	text := "Just plain text without code blocks."

	files := parseArtifacts(text)
	if len(files) != 0 {
		t.Errorf("files count = %d, want 0", len(files))
	}
}

func TestParseArtifactsUnknownLang(t *testing.T) {
	text := "```python\nprint('hi')\n```"

	files := parseArtifacts(text)
	if len(files) != 0 {
		t.Errorf("files count = %d, want 0 for unknown lang", len(files))
	}
}

func TestTruncateToolResultUnderLimit(t *testing.T) {
	result := "small result"
	got := truncateToolResult(result)
	if got != result {
		t.Errorf("truncateToolResult modified small result: %q", got)
	}
}

func TestTruncateToolResultAtLimit(t *testing.T) {
	result := strings.Repeat("x", maxToolResultBytes)
	got := truncateToolResult(result)
	if got != result {
		t.Error("result at exact limit should not be truncated")
	}
}

func TestTruncateToolResultOverLimit(t *testing.T) {
	result := strings.Repeat("x", maxToolResultBytes+1000)
	got := truncateToolResult(result)

	if len(got) > maxToolResultBytes+50 {
		t.Errorf("truncated result too large: %d", len(got))
	}
	if !strings.Contains(got, "truncated") {
		t.Error("truncated result should contain truncation marker")
	}

	half := maxToolResultBytes / 2
	if !strings.HasPrefix(got, strings.Repeat("x", half)) {
		t.Error("truncated result should start with first half")
	}
	if !strings.HasSuffix(got, strings.Repeat("x", half)) {
		t.Error("truncated result should end with last half")
	}
}

func TestBuildSystemPromptAgentMode(t *testing.T) {
	prompt := buildSystemPrompt(true)
	if !strings.Contains(prompt, "file system access") {
		t.Error("agent prompt should mention file system access")
	}
	if !strings.Contains(prompt, "read_file") {
		t.Error("agent prompt should mention read_file")
	}
	if !strings.Contains(prompt, "write_file") {
		t.Error("agent prompt should mention write_file")
	}
	if !strings.Contains(prompt, "list_files") {
		t.Error("agent prompt should mention list_files")
	}
	if !strings.Contains(prompt, "apply_edit") {
		t.Error("agent prompt should mention apply_edit")
	}
}

func TestBuildSystemPromptMarkdownMode(t *testing.T) {
	prompt := buildSystemPrompt(false)
	if !strings.Contains(prompt, "HTML") {
		t.Error("markdown prompt should mention HTML")
	}
	if !strings.Contains(prompt, "markdown code blocks") {
		t.Error("markdown prompt should mention markdown code blocks")
	}
	if strings.Contains(prompt, "read_file") {
		t.Error("markdown prompt should not mention read_file")
	}
}

func TestTextAccumulatedOrDefaultEmpty(t *testing.T) {
	got := textAccumulatedOrDefault(nil)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestTextAccumulatedOrDefaultLastAssistant(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleUser, Content: "hi"},
		{Role: types.RoleAssistant, Content: "first"},
		{Role: types.RoleTool, Content: "result"},
		{Role: types.RoleAssistant, Content: "final"},
	}
	got := textAccumulatedOrDefault(messages)
	if got != "final" {
		t.Errorf("got %q, want %q", got, "final")
	}
}

func TestTextAccumulatedOrDefaultNoAssistant(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleUser, Content: "hi"},
	}
	got := textAccumulatedOrDefault(messages)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestTextAccumulatedOrDefaultAssistantEmptyContent(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleAssistant, Content: "has content"},
		{Role: types.RoleAssistant, Content: ""},
	}
	got := textAccumulatedOrDefault(messages)
	if got != "has content" {
		t.Errorf("got %q, want %q", got, "has content")
	}
}

func TestSendMessageAgentModePersistsToolLoopMessages(t *testing.T) {
	t.Skip("requires a Wails lifecycle context; enable in an integration harness that provides runtime events")

	service, artifactMgr := newAgentModeChatServiceForTest(t)
	if err := artifactMgr.WriteFile("unused", "placeholder.txt", "placeholder"); err != nil {
		t.Fatalf("pre-flight write: %v", err)
	}

	var requestCount atomic.Int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")

		switch requestCount.Add(1) {
		case 1:
			var resp providerTestOllamaResponse
			resp.Message.Role = "assistant"
			resp.Message.ToolCalls = []providerTestOllamaCall{
				{
					ID: "call-read",
					Function: providerTestOllamaFunction{
						Name:      "read_file",
						Arguments: map[string]any{"path": "index.html"},
					},
				},
			}
			resp.Done = true
			writeOllamaToolResponse(t, w, resp)
		case 2:
			var resp providerTestOllamaResponse
			resp.Message.Role = "assistant"
			resp.Message.ToolCalls = []providerTestOllamaCall{
				{
					ID: "call-write",
					Function: providerTestOllamaFunction{
						Name: "write_file",
						Arguments: map[string]any{
							"path":    "index.html",
							"content": "<html><body>updated</body></html>",
						},
					},
				},
			}
			resp.Done = true
			writeOllamaToolResponse(t, w, resp)
		default:
			var resp providerTestOllamaResponse
			resp.Message.Role = "assistant"
			resp.Message.Content = "All done."
			resp.Done = true
			writeOllamaToolResponse(t, w, resp)
		}
	}))
	defer testServer.Close()

	service.config.Provider = "ollama"
	service.config.OllamaEndpoint = testServer.URL
	service.config.OllamaModel = "test-model"
	service.config.AgentMode = true

	sessionID := service.CreateSession("New Chat")
	if err := artifactMgr.WriteFile(sessionID, "index.html", "<html><body>old</body></html>"); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := service.SendMessage(sessionID, "Update the HTML file"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	s := service.GetSession(sessionID)
	if s == nil {
		t.Fatal("GetSession returned nil")
	}

	if len(s.Messages) != 6 {
		t.Fatalf("messages len = %d, want 6 (user + assistant/tool loop + final assistant)", len(s.Messages))
	}

	if s.Messages[1].Role != types.RoleAssistant || len(s.Messages[1].ToolCalls) != 1 || s.Messages[1].ToolCalls[0].Name != "read_file" {
		t.Fatalf("messages[1] should persist assistant tool call for read_file, got %+v", s.Messages[1])
	}
	if s.Messages[2].Role != types.RoleTool || s.Messages[2].ToolCallID != "call-read" {
		t.Fatalf("messages[2] should persist tool result for call-read, got %+v", s.Messages[2])
	}
	if s.Messages[3].Role != types.RoleAssistant || len(s.Messages[3].ToolCalls) != 1 || s.Messages[3].ToolCalls[0].Name != "write_file" {
		t.Fatalf("messages[3] should persist assistant tool call for write_file, got %+v", s.Messages[3])
	}
	if s.Messages[4].Role != types.RoleTool || s.Messages[4].ToolCallID != "call-write" {
		t.Fatalf("messages[4] should persist tool result for call-write, got %+v", s.Messages[4])
	}
	if s.Messages[5].Role != types.RoleAssistant || s.Messages[5].Content != "All done." {
		t.Fatalf("messages[5] should be final assistant response, got %+v", s.Messages[5])
	}

	content, err := artifactMgr.ReadFile(sessionID, "index.html")
	if err != nil {
		t.Fatalf("ReadFile after SendMessage: %v", err)
	}
	if content != "<html><body>updated</body></html>" {
		t.Fatalf("index.html = %q, want updated content", content)
	}
}

func TestSendMessageAgentModeReinjectsTruncatedToolResults(t *testing.T) {
	t.Skip("requires a Wails lifecycle context; enable in an integration harness that provides runtime events")

	service, artifactMgr := newAgentModeChatServiceForTest(t)

	largeContent := strings.Repeat("A", maxToolResultBytes+2048)

	var mu sync.Mutex
	var requests []ollamaChatTestRequest
	var requestCount atomic.Int32

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")

		var req ollamaChatTestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()

		switch requestCount.Add(1) {
		case 1:
			var resp providerTestOllamaResponse
			resp.Message.Role = "assistant"
			resp.Message.ToolCalls = []providerTestOllamaCall{
				{
					ID: "call-read",
					Function: providerTestOllamaFunction{
						Name:      "read_file",
						Arguments: map[string]any{"path": "large.txt"},
					},
				},
			}
			resp.Done = true
			writeOllamaToolResponse(t, w, resp)
		default:
			var resp providerTestOllamaResponse
			resp.Message.Role = "assistant"
			resp.Message.Content = "Processed truncated tool result."
			resp.Done = true
			writeOllamaToolResponse(t, w, resp)
		}
	}))
	defer testServer.Close()

	service.config.Provider = "ollama"
	service.config.OllamaEndpoint = testServer.URL
	service.config.OllamaModel = "test-model"
	service.config.AgentMode = true

	sessionID := service.CreateSession("New Chat")
	if err := artifactMgr.WriteFile(sessionID, "large.txt", largeContent); err != nil {
		t.Fatalf("seed large file: %v", err)
	}

	if err := service.SendMessage(sessionID, "Read the large file and summarize it"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(requests) < 2 {
		t.Fatalf("requests len = %d, want at least 2", len(requests))
	}

	second := requests[1]
	if len(second.Messages) == 0 {
		t.Fatal("second round request should include messages")
	}

	last := second.Messages[len(second.Messages)-1]
	if last.Role != string(types.RoleTool) {
		t.Fatalf("last message role = %q, want tool", last.Role)
	}
	if !strings.Contains(last.Content, "(truncated)") {
		t.Fatalf("tool result should be truncated before reinjection, got content without marker")
	}
	if len(last.Content) <= maxToolResultBytes {
		t.Fatalf("tool result should preserve head/tail around truncation marker, got len %d", len(last.Content))
	}
	if len(last.Content) > maxToolResultBytes+len("\n\n... (truncated) ...\n\n") {
		t.Fatalf("truncated tool result too large: %d", len(last.Content))
	}
}

func TestSummarizeOldToolResults_NoToolMessages(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleUser, Content: "hello"},
		{Role: types.RoleAssistant, Content: "hi there"},
	}

	result := summarizeOldToolResults(messages)

	if len(result) != len(messages) {
		t.Errorf("result len = %d, want %d", len(result), len(messages))
	}

	for i, m := range result {
		if m.Content != messages[i].Content {
			t.Errorf("message %d content changed", i)
		}
	}
}

func TestSummarizeOldToolResults_WithinKeepRounds(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleUser, Content: "user"},
		{Role: types.RoleAssistant, Content: "assistant", ToolCalls: []types.ToolCall{{ID: "call1"}}},
		{Role: types.RoleTool, Content: "tool result 1", ToolCallID: "call1"},
	}

	result := summarizeOldToolResults(messages)

	if len(result) != len(messages) {
		t.Errorf("result len = %d, want %d", len(result), len(messages))
	}

	if result[2].Content != "tool result 1" {
		t.Errorf("tool message within keep rounds should not be summarized, got %q", result[2].Content)
	}
}

func TestSummarizeOldToolResults_ExceedsKeepRounds(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleUser, Content: "user"},
		{Role: types.RoleAssistant, Content: "assistant 1", ToolCalls: []types.ToolCall{{ID: "call1"}}},
		{Role: types.RoleTool, Content: "tool result 1", ToolCallID: "call1"},
		{Role: types.RoleAssistant, Content: "assistant 2", ToolCalls: []types.ToolCall{{ID: "call2"}}},
		{Role: types.RoleTool, Content: "tool result 2", ToolCallID: "call2"},
		{Role: types.RoleAssistant, Content: "assistant 3", ToolCalls: []types.ToolCall{{ID: "call3"}}},
		{Role: types.RoleTool, Content: "tool result 3", ToolCallID: "call3"},
	}

	result := summarizeOldToolResults(messages)

	if len(result) != len(messages) {
		t.Errorf("result len = %d, want %d", len(result), len(messages))
	}

	if !strings.Contains(result[2].Content, summaryPrefix) {
		t.Errorf("old tool result should be summarized, got %q", result[2].Content)
	}

	if strings.Contains(result[4].Content, summaryPrefix) {
		t.Errorf("recent tool result should not be summarized, got %q", result[4].Content)
	}

	if result[4].Content != "tool result 2" {
		t.Errorf("recent tool result should be preserved, got %q", result[4].Content)
	}

	if strings.Contains(result[6].Content, summaryPrefix) {
		t.Errorf("recent tool result should not be summarized, got %q", result[6].Content)
	}

	if result[6].Content != "tool result 3" {
		t.Errorf("recent tool result should be preserved, got %q", result[6].Content)
	}
}

func TestSummarizeOldToolResults_DoesNotModifyOriginal(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleAssistant, Content: "assistant 1", ToolCalls: []types.ToolCall{{ID: "call1"}}},
		{Role: types.RoleTool, Content: "tool result 1", ToolCallID: "call1"},
		{Role: types.RoleAssistant, Content: "assistant 2", ToolCalls: []types.ToolCall{{ID: "call2"}}},
		{Role: types.RoleTool, Content: "tool result 2", ToolCallID: "call2"},
		{Role: types.RoleAssistant, Content: "assistant 3", ToolCalls: []types.ToolCall{{ID: "call3"}}},
		{Role: types.RoleTool, Content: "tool result 3", ToolCallID: "call3"},
	}

	originalContent := make([]string, len(messages))
	for i, m := range messages {
		originalContent[i] = m.Content
	}

	_ = summarizeOldToolResults(messages)

	for i, m := range messages {
		if m.Content != originalContent[i] {
			t.Errorf("original message %d was modified, got %q, want %q", i, m.Content, originalContent[i])
		}
	}
}

func TestSummarizeOldToolResults_SummaryLength(t *testing.T) {
	longContent := strings.Repeat("x", 200)
	messages := []types.Message{
		{Role: types.RoleAssistant, Content: "assistant 1", ToolCalls: []types.ToolCall{{ID: "call1"}}},
		{Role: types.RoleTool, Content: longContent, ToolCallID: "call1"},
		{Role: types.RoleAssistant, Content: "assistant 2", ToolCalls: []types.ToolCall{{ID: "call2"}}},
		{Role: types.RoleTool, Content: "tool result 2", ToolCallID: "call2"},
		{Role: types.RoleAssistant, Content: "assistant 3", ToolCalls: []types.ToolCall{{ID: "call3"}}},
		{Role: types.RoleTool, Content: "tool result 3", ToolCallID: "call3"},
	}

	result := summarizeOldToolResults(messages)

	maxLength := len(summaryPrefix) + 100 + 3 // prefix + 100 chars + "..."
	if len(result[1].Content) > maxLength {
		t.Errorf("summarized content too long: %d, max %d", len(result[1].Content), maxLength)
	}

	if !strings.HasPrefix(result[1].Content, summaryPrefix) {
		t.Errorf("summarized content should have prefix, got %q", result[1].Content)
	}

	if !strings.Contains(result[1].Content, "...") {
		t.Errorf("summarized content should have ellipsis, got %q", result[1].Content)
	}

	if result[3].Content != "tool result 2" {
		t.Errorf("recent tool result should be preserved, got %q", result[3].Content)
	}
}

func TestSummarizeOldToolResults_PreservesToolCallID(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleAssistant, Content: "assistant 1", ToolCalls: []types.ToolCall{{ID: "call1"}}},
		{Role: types.RoleTool, Content: "tool result 1", ToolCallID: "call1", CreatedAt: "2024-01-01T00:00:00Z"},
		{Role: types.RoleAssistant, Content: "assistant 2", ToolCalls: []types.ToolCall{{ID: "call2"}}},
		{Role: types.RoleTool, Content: "tool result 2", ToolCallID: "call2", CreatedAt: "2024-01-01T00:01:00Z"},
		{Role: types.RoleAssistant, Content: "assistant 3", ToolCalls: []types.ToolCall{{ID: "call3"}}},
		{Role: types.RoleTool, Content: "tool result 3", ToolCallID: "call3", CreatedAt: "2024-01-01T00:02:00Z"},
	}

	result := summarizeOldToolResults(messages)

	if result[1].ToolCallID != "call1" {
		t.Errorf("ToolCallID not preserved for message 1, got %q, want call1", result[1].ToolCallID)
	}

	if result[1].CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt not preserved for message 1, got %q, want 2024-01-01T00:00:00Z", result[1].CreatedAt)
	}

	if result[3].ToolCallID != "call2" {
		t.Errorf("ToolCallID not preserved for message 3, got %q, want call2", result[3].ToolCallID)
	}

	if result[3].CreatedAt != "2024-01-01T00:01:00Z" {
		t.Errorf("CreatedAt not preserved for message 3, got %q, want 2024-01-01T00:01:00Z", result[3].CreatedAt)
	}

	if result[5].ToolCallID != "call3" {
		t.Errorf("ToolCallID not preserved for message 5, got %q, want call3", result[5].ToolCallID)
	}

	if result[5].CreatedAt != "2024-01-01T00:02:00Z" {
		t.Errorf("CreatedAt not preserved for message 5, got %q, want 2024-01-01T00:02:00Z", result[5].CreatedAt)
	}
}

func TestSummarizeOldToolResults_MultilineContent(t *testing.T) {
	messages := []types.Message{
		{Role: types.RoleAssistant, Content: "assistant 1", ToolCalls: []types.ToolCall{{ID: "call1"}}},
		{Role: types.RoleTool, Content: "line1\nline2\nline3", ToolCallID: "call1"},
		{Role: types.RoleAssistant, Content: "assistant 2", ToolCalls: []types.ToolCall{{ID: "call2"}}},
		{Role: types.RoleTool, Content: "tool result 2", ToolCallID: "call2"},
		{Role: types.RoleAssistant, Content: "assistant 3", ToolCalls: []types.ToolCall{{ID: "call3"}}},
		{Role: types.RoleTool, Content: "tool result 3", ToolCallID: "call3"},
	}

	result := summarizeOldToolResults(messages)

	if !strings.Contains(result[1].Content, "line1") {
		t.Errorf("summarized content should contain first line, got %q", result[1].Content)
	}

	if strings.Contains(result[1].Content, "line2") {
		t.Errorf("summarized content should not contain second line, got %q", result[1].Content)
	}

	if result[3].Content != "tool result 2" {
		t.Errorf("recent tool result should be preserved, got %q", result[3].Content)
	}

	if result[5].Content != "tool result 3" {
		t.Errorf("recent tool result should be preserved, got %q", result[5].Content)
	}
}
