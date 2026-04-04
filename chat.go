package main

import (
	"context"
	"fm-my-canvas/artifacts"
	"fm-my-canvas/config"
	"fm-my-canvas/provider"
	"fm-my-canvas/session"
	"fm-my-canvas/types"
	"regexp"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ChatService struct {
	ctx      context.Context
	sessions *session.Manager
	config   *config.Config
	artifact *artifacts.Manager
	server   *artifacts.Server
}

func NewChatService(artifactMgr *artifacts.Manager, server *artifacts.Server) (*ChatService, error) {
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
		artifact: artifactMgr,
		server:   server,
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
	err := c.sessions.Delete(id)
	if err != nil {
		return err
	}
	_ = c.artifact.Cleanup(id)
	return nil
}

	var codeBlockRe = regexp.MustCompile("(?s)```(\\w+)(?:\\s+path=(\\S+))?\\s*\\n(.*?)```")


type parsedFile struct {
	Language string
	Path     string
	Content  string
}

func parseArtifacts(text string) []parsedFile {
	matches := codeBlockRe.FindAllStringSubmatch(text, -1)
	var files []parsedFile
	for _, m := range matches {
		lang := m[1]
		path := m[2]
		content := m[3]
		if path == "" {
			switch lang {
			case "html":
				path = "index.html"
			case "css":
				path = "style.css"
			case "javascript", "js":
				path = "script.js"
			default:
				continue
			}
		}
		files = append(files, parsedFile{Language: lang, Path: path, Content: content})
	}
	return files
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
	var p provider.Provider
	switch c.config.Provider {
	case "openrouter":
		p = provider.NewOpenRouter(c.config.OpenRouterAPIKey, c.config.OpenRouterModel)
	default:
		p = provider.NewOllama(c.config.OllamaEndpoint, c.config.OllamaModel)
	}
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

	files := parseArtifacts(accumulated)
	if len(files) > 0 {
		wsDir := c.artifact.WorkspaceDir(sessionID)
		for _, f := range files {
			_ = c.artifact.WriteFile(sessionID, f.Path, strings.TrimRight(f.Content, "\n"))
		}

		url, serr := c.server.Start(c.ctx, wsDir)
		if serr == nil {
			c.server.UpdateDir(wsDir)
			fileNames := make([]string, 0, len(files))
			for _, f := range files {
				fileNames = append(fileNames, f.Path)
			}
			wailsRuntime.EventsEmit(c.ctx, "artifact-update", map[string]string{
				"session_id":  sessionID,
				"preview_url": url + "/index.html",
				"files":       strings.Join(fileNames, ","),
			})
		}
	}

	return nil
}

func (c *ChatService) RestoreArtifacts(sessionID string) map[string]string {
	result := map[string]string{}
	files, err := c.artifact.ListFiles(sessionID)
	if err != nil || len(files) == 0 {
		return result
	}

	wsDir := c.artifact.WorkspaceDir(sessionID)
	url, serr := c.server.Start(c.ctx, wsDir)
	if serr != nil {
		return result
	}
	c.server.UpdateDir(wsDir)

	hasIndex := false
	for _, f := range files {
		if f == "index.html" {
			hasIndex = true
			break
		}
	}

	if hasIndex {
		result["preview_url"] = url + "/index.html"
	}
	result["files"] = strings.Join(files, ",")

	s, err := c.sessions.Get(sessionID)
	if err != nil || s == nil {
		return result
	}

	for i := len(s.Messages) - 1; i >= 0; i-- {
		msg := s.Messages[i]
		if msg.Role == types.RoleAssistant {
			parsed := parseArtifacts(msg.Content)
			if len(parsed) > 0 {
				fileNames := make([]string, 0, len(parsed))
				for _, f := range parsed {
					fileNames = append(fileNames, f.Path)
				}
				result["files"] = strings.Join(fileNames, ",")
				break
			}
		}
	}

	return result
}

func (c *ChatService) GetConfig() *config.Config {
	return c.config
}

func (c *ChatService) SaveConfig(cfg *config.Config) error {
	c.config = cfg
	return cfg.Save()
}
