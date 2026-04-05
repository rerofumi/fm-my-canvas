package tools

import (
	"strings"
	"testing"

	"fm-my-canvas/artifacts"
)

func setupSearchCodeTool(t *testing.T) (*SearchCodeTool, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-search-code"
	return NewSearchCodeTool(m), sid
}

func TestSearchCodeToolName(t *testing.T) {
	tool, _ := setupSearchCodeTool(t)
	if tool.Name() != "search_code" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "search_code")
	}
}

func TestSearchCodeToolExecuteWithMatches(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-search-code"
	m.WriteFile(sid, "app.js", "function hello() {\n  return 'hello';\n}")
	m.WriteFile(sid, "util.js", "const greeting = 'hello';")
	tool := NewSearchCodeTool(m)

	result, err := tool.Execute(sid, map[string]any{"pattern": "hello"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "Found") {
		t.Errorf("result should contain 'Found': %q", result)
	}
	if !strings.Contains(result, "app.js:1") {
		t.Errorf("result should contain 'app.js:1': %q", result)
	}
	if !strings.Contains(result, "util.js:1") {
		t.Errorf("result should contain 'util.js:1': %q", result)
	}
	if !strings.Contains(result, "2 files") {
		t.Errorf("result should mention 2 files: %q", result)
	}
}

func TestSearchCodeToolExecuteNoMatches(t *testing.T) {
	tool, sid := setupSearchCodeTool(t)

	result, err := tool.Execute(sid, map[string]any{"pattern": "nonexistent"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "No matches found") {
		t.Errorf("result should contain 'No matches found': %q", result)
	}
	if !strings.Contains(result, "nonexistent") {
		t.Errorf("result should include pattern: %q", result)
	}
}

func TestSearchCodeToolMissingPattern(t *testing.T) {
	tool, sid := setupSearchCodeTool(t)

	_, err := tool.Execute(sid, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
	if !strings.Contains(err.Error(), "missing required argument: pattern") {
		t.Errorf("error = %q, want missing pattern", err.Error())
	}
}

func TestSearchCodeToolInvalidPatternType(t *testing.T) {
	tool, sid := setupSearchCodeTool(t)

	_, err := tool.Execute(sid, map[string]any{"pattern": 123})
	if err == nil {
		t.Fatal("expected error for non-string pattern")
	}
	if !strings.Contains(err.Error(), "must be a string") {
		t.Errorf("error = %q, want must be a string", err.Error())
	}
}

func TestSearchCodeToolEmptyPattern(t *testing.T) {
	tool, sid := setupSearchCodeTool(t)

	_, err := tool.Execute(sid, map[string]any{"pattern": ""})
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestSearchCodeToolInvalidFilePatternType(t *testing.T) {
	tool, sid := setupSearchCodeTool(t)

	_, err := tool.Execute(sid, map[string]any{"pattern": "test", "file_pattern": 42})
	if err == nil {
		t.Fatal("expected error for non-string file_pattern")
	}
	if !strings.Contains(err.Error(), "must be a string") {
		t.Errorf("error = %q, want must be a string", err.Error())
	}
}

func TestSearchCodeToolWithoutFilePattern(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-search-code"
	m.WriteFile(sid, "app.js", "const x = 1;")
	m.WriteFile(sid, "style.css", "/* const */ body {}")
	tool := NewSearchCodeTool(m)

	result, err := tool.Execute(sid, map[string]any{"pattern": "const"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "app.js") {
		t.Errorf("result should contain app.js: %q", result)
	}
	if !strings.Contains(result, "style.css") {
		t.Errorf("result should contain style.css: %q", result)
	}
}

func TestSearchCodeToolWithFilePattern(t *testing.T) {
	tmpDir := t.TempDir()
	m := artifacts.NewManagerWithDir(tmpDir)
	sid := "test-session-search-code"
	m.WriteFile(sid, "app.js", "const x = 1;")
	m.WriteFile(sid, "style.css", "/* const */ body {}")
	tool := NewSearchCodeTool(m)

	result, err := tool.Execute(sid, map[string]any{"pattern": "const", "file_pattern": "*.js"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "app.js") {
		t.Errorf("result should contain app.js: %q", result)
	}
	if strings.Contains(result, "style.css") {
		t.Errorf("result should not contain style.css: %q", result)
	}
}

func TestSearchCodeToolParameters(t *testing.T) {
	tool, _ := setupSearchCodeTool(t)
	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("type = %v, want object", params["type"])
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not map[string]any")
	}
	if _, ok := props["pattern"]; !ok {
		t.Error("missing 'pattern' property")
	}
	if _, ok := props["file_pattern"]; !ok {
		t.Error("missing 'file_pattern' property")
	}
}
