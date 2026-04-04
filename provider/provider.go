package provider

import (
	"context"
	"fm-my-canvas/types"
)

type StreamEventType string

const (
	EventContent  StreamEventType = "content"
	EventToolCall StreamEventType = "tool_call"
	EventDone     StreamEventType = "done"
)

type StreamEvent struct {
	Type      StreamEventType
	Content   string
	ToolCalls []types.ToolCall
}

type ToolDefinition struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

type Provider interface {
	Stream(ctx context.Context, messages []types.Message, cb func(chunk string)) error
	StreamWithTools(ctx context.Context, messages []types.Message, tools []ToolDefinition, cb func(event StreamEvent)) error
}
