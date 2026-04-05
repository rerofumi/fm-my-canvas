package tools

import (
	"fmt"

	"fm-my-canvas/artifacts"
)

type ApplyEditTool struct {
	manager *artifacts.Manager
	engine  *EditEngine
}

func NewApplyEditTool(m *artifacts.Manager) *ApplyEditTool {
	return &ApplyEditTool{
		manager: m,
		engine:  NewEditEngine(),
	}
}

func (t *ApplyEditTool) Name() string {
	return "apply_edit"
}

func (t *ApplyEditTool) Description() string {
	return "Apply a search/replace edit to an existing file. Finds the exact search text and replaces it with the replace text. The search text must match exactly one location in the file."
}

func (t *ApplyEditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The relative path of the file to edit.",
			},
			"search": map[string]any{
				"type":        "string",
				"description": "The exact text to find in the file.",
			},
			"replace": map[string]any{
				"type":        "string",
				"description": "The text to replace the search text with.",
			},
		},
		"required": []string{"path", "search", "replace"},
	}
}

func (t *ApplyEditTool) Execute(sessionID string, args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing required argument: path")
	}

	search, ok := args["search"].(string)
	if !ok || search == "" {
		return "", fmt.Errorf("missing required argument: search")
	}

	replace, ok := args["replace"].(string)
	if !ok {
		return "", fmt.Errorf("missing required argument: replace")
	}

	content, err := t.manager.ReadFile(sessionID, path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	updated, err := t.engine.Apply(content, search, replace)
	if err != nil {
		return "", err
	}

	if err := t.manager.WriteFile(sessionID, path, updated); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s (1 replacement)", path), nil
}
