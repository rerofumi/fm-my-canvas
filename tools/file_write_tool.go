package tools

import (
	"fmt"

	"fm-my-canvas/artifacts"
)

type WriteFileTool struct {
	manager *artifacts.Manager
}

func NewWriteFileTool(m *artifacts.Manager) *WriteFileTool {
	return &WriteFileTool{manager: m}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file in the artifact workspace. Creates the file if it doesn't exist."
}

func (t *WriteFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The relative path of the file to write.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The full content to write to the file.",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(sessionID string, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing required argument: path")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: content")
	}
	if err := t.manager.WriteFile(sessionID, path, content); err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully wrote to %s", path), nil
}
