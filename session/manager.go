package session

import (
	"encoding/json"
	"fm-my-canvas/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Manager struct {
	baseDir string
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Join(home, ".config", "fm-my-canvas", "sessions")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &Manager{baseDir: baseDir}, nil
}

func (m *Manager) Create(title string) *types.Session {
	now := time.Now().Format(time.RFC3339)
	s := &types.Session{
		ID:        uuid.New().String(),
		Title:     title,
		Messages:  []types.Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	_ = m.save(s)
	return s
}

func (m *Manager) save(s *types.Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath(s.ID), data, 0644)
}

func (m *Manager) filePath(id string) string {
	return filepath.Join(m.baseDir, id+".json")
}

func (m *Manager) Get(id string) (*types.Session, error) {
	data, err := os.ReadFile(m.filePath(id))
	if err != nil {
		return nil, err
	}
	var s types.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (m *Manager) List() ([]*types.Session, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*types.Session{}, nil
		}
		return nil, err
	}
	var sessions []*types.Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		s, err := m.Get(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt > sessions[j].UpdatedAt
	})
	return sessions, nil
}

func (m *Manager) AddMessage(id string, msg types.Message) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now().Format(time.RFC3339)
	if s.Title == "New Chat" && msg.Role == types.RoleUser {
		title := msg.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		s.Title = title
	}
	return m.save(s)
}

func (m *Manager) Delete(id string) error {
	return os.Remove(m.filePath(id))
}
