package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fm-my-canvas/types"
	"fmt"
	"net/http"
)

type OllamaProvider struct {
	Endpoint string
	Model    string
}

func NewOllama(endpoint, model string) *OllamaProvider {
	return &OllamaProvider{
		Endpoint: endpoint,
		Model:    model,
	}
}

type ollamaRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaMessageFull `json:"messages"`
	Stream   bool                `json:"stream"`
	Tools    []ToolDefinition    `json:"tools,omitempty"`
}

type ollamaMessageFull struct {
	Role       string             `json:"role"`
	Content    string             `json:"content"`
	ToolCalls  []ollamaToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolFunction `json:"function"`
	ID       string             `json:"id,omitempty"`
}

type ollamaToolFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ollamaToolResponse struct {
	Message struct {
		Role      string             `json:"role"`
		Content   string             `json:"content"`
		ToolCalls []ollamaToolCall   `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done bool `json:"done"`
}

func toOllamaMessages(messages []types.Message) []ollamaMessageFull {
	result := make([]ollamaMessageFull, 0, len(messages))
	for _, m := range messages {
		msg := ollamaMessageFull{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
				ID: tc.ID,
				Function: ollamaToolFunction{
					Name:      tc.Name,
					Arguments: nil,
				},
			})
		}
		result = append(result, msg)
	}
	return result
}

func (p *OllamaProvider) Stream(ctx context.Context, messages []types.Message, cb func(chunk string)) error {
	ollamaMsgs := make([]ollamaMessageFull, 0, len(messages))
	for _, m := range messages {
		ollamaMsgs = append(ollamaMsgs, ollamaMessageFull{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	reqBody := ollamaRequest{
		Model:    p.Model,
		Messages: ollamaMsgs,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint+"/api/chat", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var chunk ollamaToolResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			cb(chunk.Message.Content)
		}
	}

	return scanner.Err()
}

func (p *OllamaProvider) StreamWithTools(ctx context.Context, messages []types.Message, tools []ToolDefinition, cb func(event StreamEvent)) error {
	reqBody := ollamaRequest{
		Model:    p.Model,
		Messages: toOllamaMessages(messages),
		Stream:   true,
		Tools:    tools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint+"/api/chat", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	callIndex := 0
	for scanner.Scan() {
		var chunk ollamaToolResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			cb(StreamEvent{
				Type:    EventContent,
				Content: chunk.Message.Content,
			})
		}

		if len(chunk.Message.ToolCalls) > 0 {
			var toolCalls []types.ToolCall
			for _, tc := range chunk.Message.ToolCalls {
				argsJSON, err := json.Marshal(tc.Function.Arguments)
				if err != nil {
					argsJSON = []byte("{}")
				}
				id := tc.ID
				if id == "" {
					id = fmt.Sprintf("ollama_tc_%d", callIndex)
					callIndex++
				}
				toolCalls = append(toolCalls, types.ToolCall{
					ID:        id,
					Name:      tc.Function.Name,
					Arguments: string(argsJSON),
				})
			}
			cb(StreamEvent{
				Type:      EventToolCall,
				ToolCalls: toolCalls,
			})
		}

		if chunk.Done {
			cb(StreamEvent{Type: EventDone})
		}
	}

	return scanner.Err()
}
