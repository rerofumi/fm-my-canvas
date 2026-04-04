package tools

import (
	"strings"
	"testing"

	"fm-my-canvas/artifacts"
)

func setupWriteTool(t *testing.T) (*WriteFileTool, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-write"
	return NewWriteFileTool(m), sid
}

func TestWriteFileToolName(t *testing.T) {
	tool, _ := setupWriteTool(t)
	if tool.Name() != "write_file" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "write_file")
	}
}

func TestWriteFileToolExecuteSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-write"
	tool := NewWriteFileTool(m)

	result, err := tool.Execute(sid, map[string]any{
		"path":    "output.txt",
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote to output.txt") {
		t.Errorf("result = %q, want success message", result)
	}

	content, _ := m.ReadFile(sid, "output.txt")
	if content != "hello world" {
		t.Errorf("file content = %q, want %q", content, "hello world")
	}
}

func TestWriteFileToolMissingPath(t *testing.T) {
	tool, sid := setupWriteTool(t)

	_, err := tool.Execute(sid, map[string]any{"content": "data"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if !strings.Contains(err.Error(), "missing required argument: path") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteFileToolMissingContent(t *testing.T) {
	tool, sid := setupWriteTool(t)

	_, err := tool.Execute(sid, map[string]any{"path": "file.txt"})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
	if !strings.Contains(err.Error(), "missing required argument: content") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteFileToolEmptyPath(t *testing.T) {
	tool, sid := setupWriteTool(t)

	_, err := tool.Execute(sid, map[string]any{"path": "", "content": "data"})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestWriteFileToolPathTraversal(t *testing.T) {
	tool, sid := setupWriteTool(t)

	_, err := tool.Execute(sid, map[string]any{
		"path":    "../../../tmp/evil.txt",
		"content": "hacked",
	})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error = %q, want path traversal", err.Error())
	}
}

func TestWriteFileToolNestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-write"
	tool := NewWriteFileTool(m)

	result, err := tool.Execute(sid, map[string]any{
		"path":    "components/App.vue",
		"content": "<template>Hi</template>",
	})
	if err != nil {
		t.Fatalf("Execute nested: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote to components/App.vue") {
		t.Errorf("result = %q", result)
	}
}

func TestWriteFileToolParameters(t *testing.T) {
	tool, _ := setupWriteTool(t)
	params := tool.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not map[string]any")
	}
	if _, ok := props["path"]; !ok {
		t.Error("missing 'path' property")
	}
	if _, ok := props["content"]; !ok {
		t.Error("missing 'content' property")
	}
}
