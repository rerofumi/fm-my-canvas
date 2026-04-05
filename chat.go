package main

import (
	"context"
	"fmt"
	"fm-my-canvas/artifacts"
	"fm-my-canvas/config"
	"fm-my-canvas/provider"
	"fm-my-canvas/session"
	"fm-my-canvas/tools"
	"fm-my-canvas/types"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxToolRounds = 10
const toolLoopTimeout = 5 * time.Minute
const maxToolResultBytes = 50 * 1024
const keepRecentRounds = 2
const summaryPrefix = "[Previous tool result summarized] "

type ChatService struct {
	ctx         context.Context
	sessions    *session.Manager
	config      *config.Config
	artifact    *artifacts.Manager
	server      *artifacts.Server
	toolManager *tools.ToolManager
	cancelMu    sync.Mutex
	cancelFn    context.CancelFunc
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

	tm := tools.NewToolManager()
	tm.Register(tools.NewReadFileTool(artifactMgr))
	tm.Register(tools.NewWriteFileTool(artifactMgr))
	tm.Register(tools.NewListFilesTool(artifactMgr))
	tm.Register(tools.NewApplyEditTool(artifactMgr))
	tm.Register(tools.NewSearchCodeTool(artifactMgr))

	return &ChatService{
		sessions:    mgr,
		config:      cfg,
		artifact:    artifactMgr,
		server:      server,
		toolManager: tm,
	}, nil
}

func (c *ChatService) SetContext(ctx context.Context) {
	c.ctx = ctx
}

func (c *ChatService) newProvider() provider.Provider {
	switch c.config.Provider {
	case "openrouter":
		return provider.NewOpenRouter(c.config.OpenRouterAPIKey, c.config.OpenRouterModel)
	default:
		return provider.NewOllama(c.config.OllamaEndpoint, c.config.OllamaModel)
	}
}

func (c *ChatService) setCancelFn(fn context.CancelFunc) {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()
	c.cancelFn = fn
}

func (c *ChatService) clearCancelFn() {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()
	c.cancelFn = nil
}

func (c *ChatService) CancelSend() {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()
	if c.cancelFn != nil {
		c.cancelFn()
		c.cancelFn = nil
	}
}

func languageFromExt(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	default:
		return strings.TrimPrefix(ext, ".")
	}
}

func (c *ChatService) GetArtifactFileContents(sessionID string) []types.ArtifactFileInfo {
	files, err := c.artifact.ListFiles(sessionID)
	if err != nil || len(files) == 0 {
		return nil
	}
	var result []types.ArtifactFileInfo
	for _, f := range files {
		content, err := c.artifact.ReadFile(sessionID, f)
		if err != nil {
			result = append(result, types.ArtifactFileInfo{Path: f, Language: languageFromExt(f), Content: ""})
			continue
		}
		result = append(result, types.ArtifactFileInfo{
			Path:     f,
			Language: languageFromExt(f),
			Content:  content,
		})
	}
	return result
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

func buildSystemPrompt(agentMode bool) string {
	if agentMode {
		return "You are a helpful coding assistant with file system access. You can read, write, and list files in the user's artifact workspace.\n\n" +
			"When asked to modify code:\n" +
			"1. First, use read_file to understand the current code\n" +
			"2. Analyze what needs to be changed\n" +
			"3. For minimal changes to existing code, use apply_edit to apply a search/replace edit\n" +
			"4. For large changes or new files, use write_file to write the full content\n" +
			"5. Always verify your changes make sense in the context of the whole project\n\n" +
			"When apply_edit fails (e.g., search text not found or multiple matches), the error will be reported back to you. " +
			"In that case, use write_file to rewrite the entire file as a fallback.\n\n" +
			"When working across multiple files or investigating an existing project:\n" +
			"1. Use list_files to understand the file layout\n" +
			"2. Use search_code to find relevant code patterns across files\n" +
			"3. Use read_file to inspect the specific files you need before making changes\n\n" +
			"Available tools:\n" +
			"- read_file(path): Read file contents\n" +
			"- write_file(path, content): Write file contents\n" +
			"- list_files([path]): List files in directory\n" +
			"- apply_edit(path, search, replace): Apply a search/replace edit to a file\n" +
			"- search_code(pattern, [file_pattern]): Search for a pattern in all files"
	}
	return "You are a helpful assistant that generates HTML, CSS, and JavaScript code for UI prototyping. " +
		"When the user asks you to create something, output the code in markdown code blocks with the filename in the header. " +
		"For example:\n\n```html path=index.html\n<!DOCTYPE html>\n...\n```\n\n" +
		"Always provide complete, working code that can be opened directly in a browser."
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
		Role:    types.RoleSystem,
		Content: buildSystemPrompt(c.config.AgentMode),
	}

	allMessages := make([]types.Message, 0, len(s.Messages)+1)
	allMessages = append(allMessages, systemMsg)
	allMessages = append(allMessages, s.Messages...)

	if c.config.AgentMode {
		return c.sendMessageWithTools(sessionID, allMessages)
	}
	return c.sendMessageMarkdown(sessionID, allMessages)
}

func (c *ChatService) sendMessageMarkdown(sessionID string, allMessages []types.Message) error {
	ctx, cancel := context.WithCancel(c.ctx)
	c.setCancelFn(cancel)
	defer func() {
		cancel()
		c.clearCancelFn()
	}()

	var accumulated string
	p := c.newProvider()
	err := p.Stream(ctx, allMessages, func(chunk string) {
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

func truncateToolResult(result string) string {
	if len(result) <= maxToolResultBytes {
		return result
	}
	half := maxToolResultBytes / 2
	return result[:half] + "\n\n... (truncated) ...\n\n" + result[len(result)-half:]
}

func summarizeOldToolResults(messages []types.Message) []types.Message {
	summarized := make([]types.Message, len(messages))
	copy(summarized, messages)

	toolIndices := []int{}
	for i, m := range summarized {
		if m.Role == types.RoleTool {
			toolIndices = append(toolIndices, i)
		}
	}

	if len(toolIndices) <= keepRecentRounds {
		return summarized
	}

	cutoff := len(toolIndices) - keepRecentRounds
	for _, idx := range toolIndices[:cutoff] {
		content := summarized[idx].Content
		firstLine := content
		if newlinePos := strings.Index(content, "\n"); newlinePos >= 0 {
			firstLine = content[:newlinePos]
		}
		if len(firstLine) > 100 {
			firstLine = firstLine[:100] + "..."
		}
		summarized[idx] = types.Message{
			Role:       summarized[idx].Role,
			Content:    summaryPrefix + firstLine,
			ToolCallID: summarized[idx].ToolCallID,
			CreatedAt:  summarized[idx].CreatedAt,
		}
	}

	return summarized
}

func (c *ChatService) sendMessageWithTools(sessionID string, messages []types.Message) error {
	ctx, cancel := context.WithTimeout(c.ctx, toolLoopTimeout)
	c.setCancelFn(cancel)
	defer func() {
		cancel()
		c.clearCancelFn()
	}()

	p := c.newProvider()

	toolDefs := buildToolDefinitions(c.toolManager)

	allMessages := make([]types.Message, len(messages))
	copy(allMessages, messages)

	for round := 0; round < maxToolRounds; round++ {
		select {
		case <-ctx.Done():
			wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
				"type":       "error",
				"content":    "tool call loop cancelled or timed out",
				"session_id": sessionID,
			})
			return fmt.Errorf("tool call loop cancelled or timed out")
		default:
		}

		var textAccumulated string
		var toolCalls []types.ToolCall

		messagesForLLM := summarizeOldToolResults(allMessages)
		err := p.StreamWithTools(ctx, messagesForLLM, toolDefs, func(event provider.StreamEvent) {
			switch event.Type {
			case provider.EventContent:
				textAccumulated += event.Content
				wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
					"type":       "chunk",
					"content":    event.Content,
					"session_id": sessionID,
				})
			case provider.EventToolCall:
				toolCalls = append(toolCalls, event.ToolCalls...)
			case provider.EventDone:
			}
		})

		if err != nil {
			wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
				"type":       "error",
				"content":    err.Error(),
				"session_id": sessionID,
			})
			return err
		}

		if len(toolCalls) == 0 {
			assistantMsg := types.Message{
				Role:      types.RoleAssistant,
				Content:   textAccumulated,
				CreatedAt: time.Now().Format(time.RFC3339),
			}
			if err := c.sessions.AddMessage(sessionID, assistantMsg); err != nil {
				return err
			}

			wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
				"type":       "done",
				"content":    textAccumulated,
				"session_id": sessionID,
			})

			c.emitArtifactUpdate(sessionID)
			return nil
		}

		allMessages = append(allMessages, types.Message{
			Role:      types.RoleAssistant,
			Content:   textAccumulated,
			ToolCalls: toolCalls,
			CreatedAt: time.Now().Format(time.RFC3339),
		})

		for _, tc := range toolCalls {
			wailsRuntime.EventsEmit(c.ctx, "tool-event", map[string]any{
				"type":       "tool_call",
				"tool_name":  tc.Name,
				"tool_args":  tc.Arguments,
				"session_id": sessionID,
			})

			result, execErr := c.toolManager.ExecuteWithContext(ctx, sessionID, tc)
			success := execErr == nil
			if execErr != nil {
				result = fmt.Sprintf("Error executing %s: %s", tc.Name, execErr.Error())
			}

			result = truncateToolResult(result)

			wailsRuntime.EventsEmit(c.ctx, "tool-event", map[string]any{
				"type":       "tool_result",
				"tool_name":  tc.Name,
				"result":     result,
				"success":    success,
				"session_id": sessionID,
			})

			allMessages = append(allMessages, types.Message{
				Role:       types.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
				CreatedAt:  time.Now().Format(time.RFC3339),
			})
		}
	}

	assistantMsg := types.Message{
		Role:      types.RoleAssistant,
		Content:   textAccumulatedOrDefault(allMessages),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := c.sessions.AddMessage(sessionID, assistantMsg); err != nil {
		return err
	}

	wailsRuntime.EventsEmit(c.ctx, "llm-event", map[string]string{
		"type":       "done",
		"content":    "reached maximum tool call rounds",
		"session_id": sessionID,
	})

	c.emitArtifactUpdate(sessionID)
	return nil
}

func textAccumulatedOrDefault(messages []types.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleAssistant && messages[i].Content != "" {
			return messages[i].Content
		}
	}
	return ""
}

func buildToolDefinitions(tm *tools.ToolManager) []provider.ToolDefinition {
	toolList := tm.Tools()
	defs := make([]provider.ToolDefinition, 0, len(toolList))
	for _, t := range toolList {
		def := provider.ToolDefinition{
			Type: "function",
		}
		def.Function.Name = t.Name()
		def.Function.Description = t.Description()
		def.Function.Parameters = t.Parameters()
		defs = append(defs, def)
	}
	return defs
}

func (c *ChatService) resolveArtifactInfo(sessionID string) (files []string, previewURL string, ok bool) {
	files, err := c.artifact.ListFiles(sessionID)
	if err != nil || len(files) == 0 {
		return nil, "", false
	}

	wsDir := c.artifact.WorkspaceDir(sessionID)
	url, serr := c.server.Start(c.ctx, wsDir)
	if serr != nil {
		return nil, "", false
	}
	c.server.UpdateDir(wsDir)

	previewURL = url
	for _, f := range files {
		if f == "index.html" {
			previewURL = url + "/index.html"
			break
		}
	}
	return files, previewURL, true
}

func (c *ChatService) emitArtifactUpdate(sessionID string) {
	files, previewURL, ok := c.resolveArtifactInfo(sessionID)
	if !ok {
		return
	}

	evt := map[string]string{
		"session_id": sessionID,
		"files":      strings.Join(files, ","),
	}
	if previewURL != "" {
		evt["preview_url"] = previewURL
	}
	wailsRuntime.EventsEmit(c.ctx, "artifact-update", evt)
}

func (c *ChatService) RestoreArtifacts(sessionID string) map[string]string {
	files, previewURL, ok := c.resolveArtifactInfo(sessionID)
	if !ok {
		return map[string]string{}
	}

	result := map[string]string{"files": strings.Join(files, ",")}
	if previewURL != "" {
		result["preview_url"] = previewURL
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
