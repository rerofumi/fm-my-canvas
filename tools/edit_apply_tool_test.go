package tools

import (
	"strings"
	"testing"

	"fm-my-canvas/artifacts"
)

func setupApplyEditTool(t *testing.T) (*ApplyEditTool, *artifacts.Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-edit"
	return NewApplyEditTool(m), m, sid
}

func TestApplyEditTool_Execute_Success(t *testing.T) {
	tool, manager, sessionID := setupApplyEditTool(t)

	initialContent := "hello world\nhello universe"
	if err := manager.WriteFile(sessionID, "test.txt", initialContent); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	args := map[string]any{
		"path":    "test.txt",
		"search":  "world",
		"replace": "moon",
	}

	result, err := tool.Execute(sessionID, args)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedMsg := "Successfully edited test.txt (1 replacement)"
	if result != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, result)
	}

	content, err := manager.ReadFile(sessionID, "test.txt")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expectedContent := "hello moon\nhello universe"
	if content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, content)
	}
}

func TestApplyEditTool_Execute_FileNotFound(t *testing.T) {
	tool, _, sessionID := setupApplyEditTool(t)

	args := map[string]any{
		"path":    "nonexistent.txt",
		"search":  "world",
		"replace": "moon",
	}

	_, err := tool.Execute(sessionID, args)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "failed to read file"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error to contain %q, got %q", expectedErr, err.Error())
	}
}

func TestApplyEditTool_Execute_NoMatch(t *testing.T) {
	tool, manager, sessionID := setupApplyEditTool(t)

	initialContent := "hello world"
	if err := manager.WriteFile(sessionID, "test.txt", initialContent); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	args := map[string]any{
		"path":    "test.txt",
		"search":  "moon",
		"replace": "sun",
	}

	_, err := tool.Execute(sessionID, args)

	if !IsNoMatch(err) {
		t.Fatalf("expected errNoMatch, got %v", err)
	}
}

func TestApplyEditTool_Execute_MultipleMatches(t *testing.T) {
	tool, manager, sessionID := setupApplyEditTool(t)

	initialContent := "hello world\nhello world"
	if err := manager.WriteFile(sessionID, "test.txt", initialContent); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	args := map[string]any{
		"path":    "test.txt",
		"search":  "world",
		"replace": "moon",
	}

	_, err := tool.Execute(sessionID, args)

	if !IsMultipleMatches(err) {
		t.Fatalf("expected errMultipleMatches, got %v", err)
	}
}

func TestApplyEditTool_Execute_MissingPath(t *testing.T) {
	tool, _, sessionID := setupApplyEditTool(t)

	args := map[string]any{
		"search":  "world",
		"replace": "moon",
	}

	_, err := tool.Execute(sessionID, args)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "missing required argument: path"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestApplyEditTool_Execute_MissingSearch(t *testing.T) {
	tool, _, sessionID := setupApplyEditTool(t)

	args := map[string]any{
		"path":    "test.txt",
		"replace": "moon",
	}

	_, err := tool.Execute(sessionID, args)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "missing required argument: search"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestApplyEditTool_Execute_MissingReplace(t *testing.T) {
	tool, _, sessionID := setupApplyEditTool(t)

	args := map[string]any{
		"path":   "test.txt",
		"search": "world",
	}

	_, err := tool.Execute(sessionID, args)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "missing required argument: replace"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestApplyEditTool_Execute_InvalidArgumentType(t *testing.T) {
	tool, manager, sessionID := setupApplyEditTool(t)

	initialContent := "hello world"
	if err := manager.WriteFile(sessionID, "test.txt", initialContent); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	tests := []struct {
		name      string
		args      map[string]any
		expectErr string
	}{
		{
			name: "search is number",
			args: map[string]any{
				"path":    "test.txt",
				"search":  123,
				"replace": "moon",
			},
			expectErr: "missing required argument: search",
		},
		{
			name: "path is number",
			args: map[string]any{
				"path":    123,
				"search":  "world",
				"replace": "moon",
			},
			expectErr: "missing required argument: path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(sessionID, tt.args)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.expectErr {
				t.Errorf("expected error %q, got %q", tt.expectErr, err.Error())
			}
		})
	}
}

func TestApplyEditTool_Execute_PathTraversal(t *testing.T) {
	tool, manager, sessionID := setupApplyEditTool(t)

	initialContent := "hello world"
	if err := manager.WriteFile(sessionID, "safe.txt", initialContent); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	args := map[string]any{
		"path":    "../safe.txt",
		"search":  "world",
		"replace": "moon",
	}

	_, err := tool.Execute(sessionID, args)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "path traversal"
	if !strings.Contains(err.Error(), expectedErr) && !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("expected error to contain %q or %q, got %q", expectedErr, "invalid path", err.Error())
	}
}
