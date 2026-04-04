package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"fm-my-canvas/types"
	"time"
)

type ToolManager struct {
	registry map[string]Tool
}

func NewToolManager() *ToolManager {
	return &ToolManager{
		registry: make(map[string]Tool),
	}
}

func (m *ToolManager) Register(tool Tool) {
	m.registry[tool.Name()] = tool
}

func (m *ToolManager) Tools() []Tool {
	result := make([]Tool, 0, len(m.registry))
	for _, t := range m.registry {
		result = append(result, t)
	}
	return result
}

func (m *ToolManager) Execute(sessionID string, tc types.ToolCall) (string, error) {
	tool, ok := m.registry[tc.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", tc.Name)
	}

	var args map[string]any
	if tc.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments for %s: %w", tc.Name, err)
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	return tool.Execute(sessionID, args)
}

func (m *ToolManager) ExecuteWithContext(ctx context.Context, sessionID string, tc types.ToolCall) (string, error) {
	resultCh := make(chan struct {
		result string
		err    error
	}, 1)

	go func() {
		r, e := m.Execute(sessionID, tc)
		resultCh <- struct {
			result string
			err    error
		}{r, e}
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("tool execution timed out: %s", tc.Name)
	case res := <-resultCh:
		return res.result, res.err
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("tool execution timed out: %s", tc.Name)
	}
}
