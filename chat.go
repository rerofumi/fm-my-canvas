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
	return buildSystemPromptForWorkspace(agentMode, nil)
}

func buildSystemPromptForWorkspace(agentMode bool, existingFiles []string) string {
	hasExistingArtifacts := len(existingFiles) > 0
	workspaceContext := buildWorkspacePromptContext(existingFiles)

	if agentMode {
		if hasExistingArtifacts {
			return `You are a coding assistant for a local artifact workspace. Your job is to update the current artifact carefully so the user's existing app improves without turning into a different app.

This is an editing session, not a fresh generation.
` + workspaceContext + `

General behavior:
- Treat the files in the artifact workspace as the source of truth.
- Treat the user's request as a modification of the current artifact unless they explicitly ask for a redesign, rewrite, or brand-new app.
- Preserve the app's identity, layout direction, naming, styling approach, and working behavior unless the user asks to change them.
- Prefer making reasonable assumptions and moving the requested edit forward instead of asking unnecessary questions.
- Do not claim to have changed a file until a tool call succeeds.
- Do not guess file contents. Read the files you need before editing them.

When editing existing code:
1. First inspect the relevant files with read_file.
2. Use list_files to understand the workspace layout when needed.
3. Use search_code to locate relevant code across files before opening many files.
4. For small, targeted edits, prefer apply_edit.
5. Use write_file only for files that truly need a full rewrite or when apply_edit is not practical.
6. Preserve working behavior unless the user asked to replace it.
7. Do not rewrite unrelated files just because you can produce a cleaner version.
8. Do not replace the whole app to satisfy a local request.

Editing strategy:
- Read only the files needed for the task, but read enough surrounding context to avoid breaking structure.
- Keep edits minimal when the request is narrow.
- Prefer preserving the existing structure, naming, styling approach, and working code unless the user asks for a redesign.
- When apply_edit fails because the search text is missing or ambiguous, fall back to write_file for that file only.
- When multiple files are involved, update only the files required for the requested behavior.
- Pay attention to references between HTML, CSS, and JavaScript files.

Response style:
- After completing tool calls, briefly explain what you changed.
- Mention important assumptions only if they affect the result.

Available tools:
- read_file(path): Read file contents
- write_file(path, content): Write file contents
- list_files([path]): List files in directory
- apply_edit(path, search, replace): Apply a search/replace edit to a file
- search_code(pattern, [file_pattern]): Search for a pattern in all files`
		}

		return `You are a coding assistant for a local artifact workspace. Your job is to help the user prototype UI quickly and create an initial artifact that is easy to iterate on.

This is an initial generation session. No existing artifact files are present yet.

General behavior:
- Treat the files in the artifact workspace as the source of truth.
- Prefer making reasonable assumptions and moving the prototype forward instead of asking unnecessary questions.
- Do not claim to have changed a file until a tool call succeeds.
- Do not guess file contents. Read the files you need before editing them.
- Keep solutions practical, runnable, and easy to preview locally.

When the user wants a new prototype:
1. Prefer a small, self-contained browser app that works directly in the preview.
2. Unless the existing project structure suggests otherwise, use plain HTML/CSS/JavaScript with files such as index.html, style.css, and script.js.
3. Avoid external dependencies, package managers, build steps, and remote CDN assets unless the user explicitly asks for them.
4. Produce complete working files, not partial snippets.

When the user wants changes to existing code:
1. First inspect the relevant files with read_file.
2. Use list_files to understand the workspace layout when needed.
3. Use search_code to locate relevant code across files before opening many files.
4. For small, targeted edits, prefer apply_edit.
5. For large rewrites, ambiguous search/replace cases, or new files, use write_file with the full file content.
6. Preserve working behavior unless the user asked to replace it.
7. Do not rewrite unrelated files just because you can produce a cleaner full version.

Editing strategy:
- Read only the files needed for the task, but read enough surrounding context to avoid breaking structure.
- Keep edits minimal when the request is narrow.
- Prefer preserving the existing structure, naming, styling approach, and working code unless the user asks for a redesign.
- When apply_edit fails because the search text is missing or ambiguous, fall back to write_file.
- When multiple files are involved, update them coherently so the preview remains runnable.
- Pay attention to references between HTML, CSS, and JavaScript files.

Response style:
- After completing tool calls, briefly explain what you changed.
- Mention any important assumptions or limitations only if they matter.

Available tools:
- read_file(path): Read file contents
- write_file(path, content): Write file contents
- list_files([path]): List files in directory
- apply_edit(path, search, replace): Apply a search/replace edit to a file
- search_code(pattern, [file_pattern]): Search for a pattern in all files`
	}
	if hasExistingArtifacts {
		return `You are a helpful assistant for fast UI prototyping in a local artifact preview app.

This is an editing session for an existing artifact, not a blank-slate generation.
` + workspaceContext + `

Your main job is to modify the current prototype without unnecessarily replacing the rest of the app.

Guidelines:
- Treat the current artifact files as the source of truth.
- Treat the user's request as a modification of the current files unless they explicitly ask for a redesign or rebuild.
- Preserve the app's overall identity, structure, and working parts unless the user asks to change them.
- Output complete files only for the files that actually changed.
- Do not regenerate unrelated files when only a focused change was requested.
- Avoid external dependencies, package managers, build steps, frameworks, and remote CDN assets unless the user explicitly asks for them.
- Make reasonable design and UX decisions on your own when details are missing.

Output format:
- Put each changed file in markdown code blocks with a path header.
- Use this format:

` + "```html path=index.html\n<!DOCTYPE html>\n...\n```" + `

- Include only the files that should be replaced.
- Ensure the result remains directly previewable in a browser.`
	}

	return `You are a helpful assistant for fast UI prototyping in a local artifact preview app.

Your main job is to generate complete, runnable browser code that can be previewed immediately.

Guidelines:
- Prefer small, self-contained HTML/CSS/JavaScript prototypes.
- Avoid external dependencies, package managers, build steps, frameworks, and remote CDN assets unless the user explicitly asks for them.
- Choose simple filenames unless the user asked for a different structure. Default to files such as index.html, style.css, and script.js.
- If the user is iterating on an existing artifact, treat the request as a modification of the current files, not a full rebuild, unless they explicitly ask to redesign or recreate it.
- If you update an existing prototype, output every changed file as a complete file, not a patch or partial snippet.
- Do not regenerate unrelated files when only a focused change was requested.
- Make reasonable design and UX decisions on your own when details are missing.

Output format:
- Put each file in markdown code blocks with a path header.
- Use this format:

` + "```html path=index.html\n<!DOCTYPE html>\n...\n```" + `

- Include only the files that should exist or be replaced.
- Ensure the result can be opened directly in a browser.`
}

func buildWorkspacePromptContext(existingFiles []string) string {
	if len(existingFiles) == 0 {
		return ""
	}

	limit := len(existingFiles)
	if limit > 12 {
		limit = 12
	}

	var b strings.Builder
	b.WriteString("Current artifact files:\n")
	for _, path := range existingFiles[:limit] {
		b.WriteString("- ")
		b.WriteString(path)
		b.WriteString("\n")
	}
	if len(existingFiles) > limit {
		b.WriteString("- ...\n")
	}
	return strings.TrimRight(b.String(), "\n")
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

	existingFiles, err := c.artifact.ListFiles(sessionID)
	if err != nil {
		existingFiles = nil
	}

	systemMsg := types.Message{
		Role:    types.RoleSystem,
		Content: buildSystemPromptForWorkspace(c.config.AgentMode, existingFiles),
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
