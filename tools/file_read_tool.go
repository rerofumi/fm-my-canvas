package tools

import (
	"fmt"

	"fm-my-canvas/artifacts"
)

type ReadFileTool struct {
	manager *artifacts.Manager
}

func NewReadFileTool(m *artifacts.Manager) *ReadFileTool {
	return &ReadFileTool{manager: m}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file from the current artifact workspace."
}

func (t *ReadFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The relative path of the file to read (e.g., 'index.html', 'components/Button.vue').",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(sessionID string, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing required argument: path")
	}
	return t.manager.ReadFile(sessionID, path)
}
