package tools

import (
	"strings"
	"testing"

	"fm-my-canvas/artifacts"
)

func setupReadTool(t *testing.T) (*ReadFileTool, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-read"
	return NewReadFileTool(m), sid
}

func TestReadFileToolName(t *testing.T) {
	tool, _ := setupReadTool(t)
	if tool.Name() != "read_file" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "read_file")
	}
}

func TestReadFileToolExecuteSuccess(t *testing.T) {
	tool, sid := setupReadTool(t)

	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	m.WriteFile(sid, "test.txt", "file content")
	tool = NewReadFileTool(m)

	result, err := tool.Execute(sid, map[string]any{"path": "test.txt"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "file content" {
		t.Errorf("result = %q, want %q", result, "file content")
	}
}

func TestReadFileToolMissingPath(t *testing.T) {
	tool, sid := setupReadTool(t)

	_, err := tool.Execute(sid, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if !strings.Contains(err.Error(), "missing required argument: path") {
		t.Errorf("error = %q, want missing path", err.Error())
	}
}

func TestReadFileToolEmptyPath(t *testing.T) {
	tool, sid := setupReadTool(t)

	_, err := tool.Execute(sid, map[string]any{"path": ""})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestReadFileToolFileNotFound(t *testing.T) {
	tool, sid := setupReadTool(t)

	_, err := tool.Execute(sid, map[string]any{"path": "nonexistent.txt"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadFileToolPathTraversal(t *testing.T) {
	tool, sid := setupReadTool(t)

	_, err := tool.Execute(sid, map[string]any{"path": "../../etc/passwd"})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error = %q, want path traversal", err.Error())
	}
}

func TestReadFileToolParameters(t *testing.T) {
	tool, _ := setupReadTool(t)
	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("type = %v, want object", params["type"])
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not map[string]any")
	}
	if _, ok := props["path"]; !ok {
		t.Error("missing 'path' property")
	}
}
