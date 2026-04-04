package artifacts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const maxFileSize = 1 * 1024 * 1024

type Manager struct {
	mu       sync.Mutex
	baseDir  string
	sessions map[string]string
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Join(home, ".config", "fm-my-canvas", "artifacts")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &Manager{
		baseDir:  baseDir,
		sessions: make(map[string]string),
	}, nil
}

func NewManagerWithDir(baseDir string) *Manager {
	return &Manager{
		baseDir:  baseDir,
		sessions: make(map[string]string),
	}
}

func (m *Manager) WorkspaceDir(sessionID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if dir, ok := m.sessions[sessionID]; ok {
		return dir
	}
	dir := filepath.Join(m.baseDir, sessionID)
	_ = os.MkdirAll(dir, 0755)
	m.sessions[sessionID] = dir
	return dir
}

func (m *Manager) validateWorkspacePath(sessionID, filename string) (string, error) {
	dir := m.WorkspaceDir(sessionID)
	fullPath := filepath.Join(dir, filename)
	cleanDir := filepath.Clean(dir)
	cleanPath := filepath.Clean(fullPath)
	if cleanPath == cleanDir || !strings.HasPrefix(cleanPath, cleanDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal detected: %s", filename)
	}
	return cleanPath, nil
}

func (m *Manager) ReadFile(sessionID, filename string) (string, error) {
	fullPath, err := m.validateWorkspacePath(sessionID, filename)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", filename)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", filename)
	}
	if info.Size() > maxFileSize {
		return "", fmt.Errorf("file too large: %s", filename)
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %s: %w", filename, err)
	}
	return string(content), nil
}

func (m *Manager) WriteFile(sessionID, filename, content string) error {
	fullPath, err := m.validateWorkspacePath(sessionID, filename)
	if err != nil {
		return err
	}
	if len(content) > maxFileSize {
		return fmt.Errorf("content too large: %s", filename)
	}
	_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
	tmpPath := fullPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write tmp file: %w", err)
	}
	if err := os.Rename(tmpPath, fullPath); err != nil {
		return fmt.Errorf("failed to rename tmp file: %w", err)
	}
	return nil
}

func (m *Manager) Cleanup(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	dir, ok := m.sessions[sessionID]
	if !ok {
		dir = filepath.Join(m.baseDir, sessionID)
	}
	delete(m.sessions, sessionID)
	return os.RemoveAll(dir)
}

func (m *Manager) ListFiles(sessionID string) ([]string, error) {
	dir := m.WorkspaceDir(sessionID)
	evalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate workspace symlinks: %w", err)
	}
	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		evalPath, lerr := filepath.EvalSymlinks(path)
		if lerr != nil {
			return nil
		}
		if !strings.HasPrefix(evalPath, evalDir+string(os.PathSeparator)) {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}
