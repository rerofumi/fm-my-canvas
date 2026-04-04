package tools

import (
	"path/filepath"
	"strings"
	"testing"

	"fm-my-canvas/artifacts"
)

func setupListTool(t *testing.T) (*ListFilesTool, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-list"
	return NewListFilesTool(m), sid
}

func TestListFilesToolName(t *testing.T) {
	tool, _ := setupListTool(t)
	if tool.Name() != "list_files" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "list_files")
	}
}

func TestListFilesToolEmpty(t *testing.T) {
	tool, sid := setupListTool(t)

	result, err := tool.Execute(sid, map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "(no files)" {
		t.Errorf("result = %q, want %q", result, "(no files)")
	}
}

func TestListFilesToolWithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-list"
	m.WriteFile(sid, "index.html", "<html></html>")
	m.WriteFile(sid, "style.css", "body{}")
	tool := NewListFilesTool(m)

	result, err := tool.Execute(sid, map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "index.html") {
		t.Errorf("result missing index.html: %q", result)
	}
	if !strings.Contains(result, "style.css") {
		t.Errorf("result missing style.css: %q", result)
	}
	if !strings.Contains(result, "2 files") {
		t.Errorf("result missing file count: %q", result)
	}
}

func TestListFilesToolWithPathFilter(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-list"
	m.WriteFile(sid, "index.html", "<html></html>")
	m.WriteFile(sid, filepath.Join("components", "App.vue"), "<template></template>")
	m.WriteFile(sid, filepath.Join("components", "Button.vue"), "<button></button>")
	tool := NewListFilesTool(m)

	files, _ := m.ListFiles(sid)
	_ = files

	result, err := tool.Execute(sid, map[string]any{"path": "components"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, f := range files {
		t.Logf("file: %q", f)
	}
	t.Logf("result: %q", result)

	if strings.Contains(result, "index.html") {
		t.Errorf("result should not contain index.html: %q", result)
	}
	hasApp := strings.Contains(result, "App.vue")
	hasButton := strings.Contains(result, "Button.vue")
	if !hasApp || !hasButton {
		t.Errorf("result should contain App.vue and Button.vue: %q", result)
	}
}

func TestListFilesToolWithNonexistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-list"
	m.WriteFile(sid, "index.html", "<html></html>")
	tool := NewListFilesTool(m)

	result, err := tool.Execute(sid, map[string]any{"path": "nonexistent"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result != "(no files)" {
		t.Errorf("result = %q, want %q", result, "(no files)")
	}
}

func TestListFilesToolParameters(t *testing.T) {
	tool, _ := setupListTool(t)
	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("type = %v, want object", params["type"])
	}
}
