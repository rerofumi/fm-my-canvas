package main

import (
	"context"
	"fm-my-canvas/config"
	"fm-my-canvas/provider"
	"fm-my-canvas/session"
	"fm-my-canvas/types"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ChatService struct {
	ctx      context.Context
	sessions *session.Manager
	config   *config.Config
}

func NewChatService() (*ChatService, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	mgr, err := session.NewManager()
	if err != nil {
		return nil, err
	}
	return &ChatService{
		sessions: mgr,
		config:   cfg,
	}, nil
}

func (c *ChatService) SetContext(ctx context.Context) {
	c.ctx = ctx
}

func (c *ChatService) CreateSession(title string) string {
	s := c.sessions.Create(title)
	return s.ID
}

func (c *ChatService) ListSessions() []*types.Session {
	sessions, err := c.sessions.List()
	if err != nil {
		return []*types.Session{}
	}
	return sessions
}

func (c *ChatService) GetSession(id string) *types.Session {
	s, err := c.sessions.Get(id)
	if err != nil {
		return nil
	}
	return s
}

func (c *ChatService) DeleteSession(id string) error {
	return c.sessions.Delete(id)
}

func (c *ChatService) SendMessage(sessionID string, message string) error {
	userMsg := types.Message{
		Role:      types.RoleUser,
		Content:   message,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := c.sessions.AddMessage(sessionID, userMsg); err != nil {
		return err
	}

	s, err := c.sessions.Get(sessionID)
	if err != nil {
		return err
	}

	systemMsg := types.Message{
		Role: types.RoleSystem,
		Content: "You are a helpful assistant that generates HTML, CSS, and JavaScript code for UI prototyping. " +
			"When the user asks you to create something, output the code in markdown code blocks with the filename in the header. " +
			"For example:\n\n```html path=index.html\n<!DOCTYPE html>\n...\n```\n\n" +
			"Always provide complete, working code that can be opened directly in a browser.",
	}

	allMessages := make([]types.Message, 0, len(s.Messages)+1)
	allMessages = append(allMessages, systemMsg)
	allMessages = append(allMessages, s.Messages...)

	var accumulated string
	p := provider.NewOllama(c.config.OllamaEndpoint, c.config.OllamaModel)
	err = p.Stream(c.ctx, allMessages, func(chunk string) {
		accumulated += chunk
		wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
			"type":       "chunk",
			"content":    chunk,
			"session_id": sessionID,
		})
	})

	if err != nil {
		wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
			"type":       "error",
			"content":    err.Error(),
			"session_id": sessionID,
		})
		return err
	}

	assistantMsg := types.Message{
		Role:      types.RoleAssistant,
		Content:   accumulated,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := c.sessions.AddMessage(sessionID, assistantMsg); err != nil {
		return err
	}

	wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
		"type":       "done",
		"content":    accumulated,
		"session_id": sessionID,
	})

	return nil
}

func (c *ChatService) GetConfig() *config.Config {
	return c.config
}

func (c *ChatService) SaveConfig(cfg *config.Config) error {
	c.config = cfg
	return cfg.Save()
}
