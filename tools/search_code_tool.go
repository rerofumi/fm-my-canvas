package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"fm-my-canvas/artifacts"
)

type SearchCodeTool struct {
	manager *artifacts.Manager
}

func NewSearchCodeTool(m *artifacts.Manager) *SearchCodeTool {
	return &SearchCodeTool{manager: m}
}

func (t *SearchCodeTool) Name() string {
	return "search_code"
}

func (t *SearchCodeTool) Description() string {
	return "Search for a pattern in all files in the artifact workspace. Returns matching files and line numbers."
}

func (t *SearchCodeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regex pattern to search for.",
			},
			"file_pattern": map[string]any{
				"type":        "string",
				"description": "Optional file pattern to filter (e.g., '*.ts', '*.go').",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *SearchCodeTool) Execute(sessionID string, args map[string]any) (string, error) {
	patternVal, ok := args["pattern"]
	if !ok {
		return "", fmt.Errorf("missing required argument: pattern")
	}
	pattern, ok := patternVal.(string)
	if !ok {
		return "", fmt.Errorf("argument 'pattern' must be a string")
	}
	if pattern == "" {
		return "", fmt.Errorf("argument 'pattern' must not be empty")
	}

	filePattern := ""
	if fp, ok := args["file_pattern"]; ok {
		s, ok := fp.(string)
		if !ok {
			return "", fmt.Errorf("argument 'file_pattern' must be a string")
		}
		filePattern = s
	}

	results, err := t.manager.SearchFiles(sessionID, pattern, filePattern)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return fmt.Sprintf("No matches found for pattern: %s", pattern), nil
	}

	fileSet := make(map[string]bool)
	var lines []string
	for _, r := range results {
		fileSet[r.File] = true
		lines = append(lines, fmt.Sprintf("%s:%d:  %s", filepath.ToSlash(r.File), r.Line, r.Content))
	}

	header := fmt.Sprintf("Found %d matches in %d files:", len(results), len(fileSet))
	return header + "\n\n" + strings.Join(lines, "\n"), nil
}
