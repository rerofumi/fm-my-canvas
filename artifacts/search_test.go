package artifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupSearchManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)
	return m, tmpDir
}

func TestSearchFilesBasic(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-search"
	m.WriteFile(sid, "app.js", "function hello() {\n  console.log('hello');\n}")
	m.WriteFile(sid, "style.css", "body { color: red; }")

	results, err := m.SearchFiles(sid, "hello", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}

	var foundFunc, foundLog bool
	for _, r := range results {
		if r.Line == 1 && strings.Contains(r.Content, "function hello()") {
			foundFunc = true
		}
		if r.Line == 2 && strings.Contains(r.Content, "console.log('hello')") {
			foundLog = true
		}
	}
	if !foundFunc {
		t.Error("missing 'function hello()' match at line 1")
	}
	if !foundLog {
		t.Error("missing 'console.log' match at line 2")
	}
}

func TestSearchFilesMultipleFiles(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-multi"
	m.WriteFile(sid, "a.js", "const x = 1;\nconst y = 2;")
	m.WriteFile(sid, "b.js", "const z = 3;")
	m.WriteFile(sid, "c.css", ".x { color: red; }")

	results, err := m.SearchFiles(sid, "const", "*.js")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results count = %d, want 3", len(results))
	}

	fileSet := make(map[string]int)
	for _, r := range results {
		fileSet[r.File]++
	}
	if fileSet["a.js"] != 2 {
		t.Errorf("a.js matches = %d, want 2", fileSet["a.js"])
	}
	if fileSet["b.js"] != 1 {
		t.Errorf("b.js matches = %d, want 1", fileSet["b.js"])
	}
	if _, ok := fileSet["c.css"]; ok {
		t.Error("c.css should not match file_pattern *.js")
	}
}

func TestSearchFilesFilePattern(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-filter"
	m.WriteFile(sid, "app.js", "console.log('js');")
	m.WriteFile(sid, "app.css", "console.log('css');")

	results, err := m.SearchFiles(sid, "console", "*.js")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].File != "app.js" {
		t.Errorf("file = %q, want %q", results[0].File, "app.js")
	}
}

func TestSearchFilesInvalidRegex(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-invalid"

	_, err := m.SearchFiles(sid, "[invalid", "")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "invalid regex") {
		t.Errorf("error = %q, want invalid regex", err.Error())
	}
}

func TestSearchFilesNoMatch(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-nomatch"
	m.WriteFile(sid, "app.js", "console.log('hi');")

	results, err := m.SearchFiles(sid, "nonexistent_pattern_xyz", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results count = %d, want 0", len(results))
	}
}

func TestSearchFilesBinarySkipped(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-binary"
	wsDir := m.WorkspaceDir(sid)

	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x00}
	binPath := filepath.Join(wsDir, "image.png")
	if err := os.WriteFile(binPath, binaryContent, 0644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	m.WriteFile(sid, "readme.txt", "some text with match")

	results, err := m.SearchFiles(sid, "match", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].File != "readme.txt" {
		t.Errorf("file = %q, want readme.txt", results[0].File)
	}
}

func TestSearchFilesLargeFileSkipped(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-large"
	wsDir := m.WorkspaceDir(sid)

	largeContent := strings.Repeat("x", 1024*1024+1)
	largePath := filepath.Join(wsDir, "large.txt")
	if err := os.WriteFile(largePath, []byte(largeContent), 0644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	results, err := m.SearchFiles(sid, "x", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results count = %d, want 0 (large file skipped)", len(results))
	}
}

func TestSearchFilesResultLimit(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-limit"

	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, "match line")
	}
	m.WriteFile(sid, "big.js", strings.Join(lines, "\n"))

	results, err := m.SearchFiles(sid, "match", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 50 {
		t.Errorf("results count = %d, want 50", len(results))
	}
}

func TestSearchFilesEmptyWorkspace(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-empty"

	results, err := m.SearchFiles(sid, "anything", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results count = %d, want 0", len(results))
	}
}

func TestSearchFilesSubdirectory(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-subdir"
	m.WriteFile(sid, filepath.Join("src", "main.js"), "function init() {}")
	m.WriteFile(sid, filepath.Join("src", "utils", "helper.js"), "function help() {}")

	results, err := m.SearchFiles(sid, "function", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}

	files := make(map[string]bool)
	for _, r := range results {
		files[r.File] = true
	}
	if !files[filepath.Join("src", "main.js")] {
		t.Error("missing src/main.js")
	}
	if !files[filepath.Join("src", "utils", "helper.js")] {
		t.Error("missing src/utils/helper.js")
	}
}

func TestSearchFilesResultOrdering(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-order"
	m.WriteFile(sid, "b.js", "line1 match\nline2 match")
	m.WriteFile(sid, "a.js", "line1 match")

	results, err := m.SearchFiles(sid, "match", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("results count = %d, want 3", len(results))
	}

	if results[0].File != "a.js" {
		t.Errorf("results[0].File = %q, want a.js", results[0].File)
	}
	if results[1].File != "b.js" {
		t.Errorf("results[1].File = %q, want b.js", results[1].File)
	}
	if results[1].Line != 1 {
		t.Errorf("results[1].Line = %d, want 1", results[1].Line)
	}
	if results[2].Line != 2 {
		t.Errorf("results[2].Line = %d, want 2", results[2].Line)
	}
}

func TestSearchFilesSymlinkEscape(t *testing.T) {
	m, tmpDir := setupSearchManager(t)
	sid := "sess-symlink"
	wsDir := m.WorkspaceDir(sid)
	m.WriteFile(sid, "safe.txt", "safe content")

	outsideTarget := filepath.Join(tmpDir, "outside.js")
	if err := os.WriteFile(outsideTarget, []byte("secret match"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkPath := filepath.Join(wsDir, "escape.js")
	if err := os.Symlink(outsideTarget, linkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	results, err := m.SearchFiles(sid, "match", "")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	for _, r := range results {
		if r.File == "escape.js" {
			t.Fatal("SearchFiles should not return symlink escaping workspace")
		}
	}
}

func TestSearchFilesFilePatternWithSeparator(t *testing.T) {
	m, _ := setupSearchManager(t)
	sid := "sess-seppattern"

	_, err := m.SearchFiles(sid, "test", "sub/dir/*.js")
	if err == nil {
		t.Fatal("expected error for file_pattern with path separator")
	}
	if !strings.Contains(err.Error(), "path separators") {
		t.Errorf("error = %q, want path separators", err.Error())
	}
}
