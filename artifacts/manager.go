package artifacts

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

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

func (m *Manager) WriteFile(sessionID, filename, content string) error {
	dir := m.WorkspaceDir(sessionID)
	tmpPath := filepath.Join(dir, filename+".tmp")
	finalPath := filepath.Join(dir, filename)

	_ = os.MkdirAll(filepath.Dir(finalPath), 0755)

	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write tmp file: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
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
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}
