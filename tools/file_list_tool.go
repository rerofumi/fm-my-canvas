package tools

import (
	"fmt"
	"os"
	"strings"

	"fm-my-canvas/artifacts"
)

type ListFilesTool struct {
	manager *artifacts.Manager
}

func NewListFilesTool(m *artifacts.Manager) *ListFilesTool {
	return &ListFilesTool{manager: m}
}

func (t *ListFilesTool) Name() string {
	return "list_files"
}

func (t *ListFilesTool) Description() string {
	return "List all files in the artifact workspace or a specific directory."
}

func (t *ListFilesTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The relative path of the directory to list. Empty for workspace root.",
			},
		},
		"required": []string{},
	}
}

func (t *ListFilesTool) Execute(sessionID string, args map[string]any) (string, error) {
	subPath := ""
	if p, ok := args["path"]; ok {
		if s, ok := p.(string); ok {
			subPath = s
		}
	}

	files, err := t.manager.ListFiles(sessionID)
	if err != nil {
		return "", err
	}

	subPathSep := subPath + string(os.PathSeparator)
	var filtered []string
	for _, f := range files {
		if subPath == "" || strings.HasPrefix(f, subPathSep) || f == subPath {
			filtered = append(filtered, f)
		}
	}

	if len(filtered) == 0 {
		return "(no files)", nil
	}

	return fmt.Sprintf("%s\n(%d files)", strings.Join(filtered, "\n"), len(filtered)), nil
}
