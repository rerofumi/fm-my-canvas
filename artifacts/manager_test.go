package artifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)
	return m, tmpDir
}

func TestNewManagerWithDir(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManagerWithDir(tmpDir)
	if m == nil {
		t.Fatal("NewManagerWithDir returned nil")
	}
	if m.baseDir != tmpDir {
		t.Errorf("baseDir = %q, want %q", m.baseDir, tmpDir)
	}
}

func TestWorkspaceDir(t *testing.T) {
	m, tmpDir := setupManager(t)
	sid := "test-session"
	dir := m.WorkspaceDir(sid)

	expected := filepath.Join(tmpDir, sid)
	if dir != expected {
		t.Errorf("WorkspaceDir = %q, want %q", dir, expected)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("workspace dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("workspace dir is not a directory")
	}
}

func TestWriteAndReadFile(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-rw"

	err := m.WriteFile(sid, "hello.txt", "hello world")
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	content, err := m.ReadFile(sid, "hello.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content != "hello world" {
		t.Errorf("content = %q, want %q", content, "hello world")
	}
}

func TestWriteFileNestedPath(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-nested"

	err := m.WriteFile(sid, "sub/dir/file.txt", "nested content")
	if err != nil {
		t.Fatalf("WriteFile nested: %v", err)
	}

	content, err := m.ReadFile(sid, "sub/dir/file.txt")
	if err != nil {
		t.Fatalf("ReadFile nested: %v", err)
	}
	if content != "nested content" {
		t.Errorf("content = %q, want %q", content, "nested content")
	}
}

func TestReadFileNotFound(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-notfound"

	_, err := m.ReadFile(sid, "missing.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("error = %q, want file not found", err.Error())
	}
}

func TestReadFileIsDirectory(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-dirread"
	wsDir := m.WorkspaceDir(sid)

	subDir := filepath.Join(wsDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := m.ReadFile(sid, "subdir")
	if err == nil {
		t.Fatal("expected error for directory read")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("error = %q, want directory error", err.Error())
	}
}

func TestPathTraversalDotDot(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-traversal"

	err := m.WriteFile(sid, "../../../etc/passwd", "hacked")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error = %q, want path traversal", err.Error())
	}

	_, err = m.ReadFile(sid, "../../secret.txt")
	if err == nil {
		t.Fatal("expected error for path traversal read")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error = %q, want path traversal", err.Error())
	}
}

func TestReadFileTooLarge(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-toolarge"
	wsDir := m.WorkspaceDir(sid)

	bigFile := filepath.Join(wsDir, "big.txt")
	bigContent := strings.Repeat("x", 1024*1024+1)
	if err := os.WriteFile(bigFile, []byte(bigContent), 0644); err != nil {
		t.Fatalf("write big file: %v", err)
	}

	_, err := m.ReadFile(sid, "big.txt")
	if err == nil {
		t.Fatal("expected error for file too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want too large", err.Error())
	}
}

func TestWriteFileContentTooLarge(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-writetoolarge"

	bigContent := strings.Repeat("x", 1024*1024+1)
	err := m.WriteFile(sid, "big.txt", bigContent)
	if err == nil {
		t.Fatal("expected error for content too large")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want too large", err.Error())
	}
}

func TestListFiles(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-list"

	m.WriteFile(sid, "index.html", "<html></html>")
	m.WriteFile(sid, "style.css", "body {}")
	m.WriteFile(sid, filepath.Join("scripts", "app.js"), "console.log('hi')")

	files, err := m.ListFiles(sid)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("files count = %d, want 3: %v", len(files), files)
	}

	expected := map[string]bool{
		"index.html":                   false,
		"style.css":                    false,
		filepath.Join("scripts", "app.js"): false,
	}
	for _, f := range files {
		if _, ok := expected[f]; ok {
			expected[f] = true
		} else {
			t.Errorf("unexpected file: %q", f)
		}
	}
	for f, found := range expected {
		if !found {
			t.Errorf("missing file: %q", f)
		}
	}
}

func TestListFilesEmpty(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-empty"

	files, err := m.ListFiles(sid)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("files count = %d, want 0", len(files))
	}
}

func TestListFilesExcludesWorkspaceEscape(t *testing.T) {
	m, tmpDir := setupManager(t)
	sid := "sess-escape"
	wsDir := m.WorkspaceDir(sid)
	m.WriteFile(sid, "safe.txt", "ok")

	outsideTarget := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outsideTarget, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkPath := filepath.Join(wsDir, "link.txt")
	if err := os.Symlink(outsideTarget, linkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	files, err := m.ListFiles(sid)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	for _, f := range files {
		if f == "link.txt" {
			t.Fatalf("ListFiles should not return symlink escaping workspace: %q", f)
		}
	}

	cleanDir, err := filepath.EvalSymlinks(wsDir)
	if err != nil {
		t.Fatalf("EvalSymlinks workspace: %v", err)
	}
	for _, f := range files {
		fullPath := filepath.Join(wsDir, f)
		cleanPath, err := filepath.EvalSymlinks(fullPath)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q): %v", fullPath, err)
		}
		if !strings.HasPrefix(cleanPath, cleanDir+string(os.PathSeparator)) {
			t.Errorf("file outside workspace returned: %q", f)
		}
	}
}

func TestCleanup(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-cleanup"
	m.WriteFile(sid, "temp.txt", "temporary")

	err := m.Cleanup(sid)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	_, rerr := m.ReadFile(sid, "temp.txt")
	if rerr == nil {
		t.Error("file should be cleaned up")
	}
}

func TestWriteFileOverwrite(t *testing.T) {
	m, _ := setupManager(t)
	sid := "sess-overwrite"

	m.WriteFile(sid, "file.txt", "original")
	m.WriteFile(sid, "file.txt", "updated")

	content, err := m.ReadFile(sid, "file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content != "updated" {
		t.Errorf("content = %q, want %q", content, "updated")
	}
}
