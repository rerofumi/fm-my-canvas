package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fm-my-canvas/types"
	"fmt"
	"net/http"
	"strings"
)

type OpenRouterProvider struct {
	APIKey  string
	Model   string
	baseURL string
}

func NewOpenRouter(apiKey, model string) *OpenRouterProvider {
	return &OpenRouterProvider{
		APIKey:  apiKey,
		Model:   model,
		baseURL: "https://openrouter.ai/api/v1/chat/completions",
	}
}

type openrouterToolRequest struct {
	Model    string                    `json:"model"`
	Messages []openrouterMessageFull   `json:"messages"`
	Stream   bool                      `json:"stream"`
	Tools    []ToolDefinition          `json:"tools,omitempty"`
}

type openrouterMessageFull struct {
	Role       string                   `json:"role"`
	Content    string                   `json:"content,omitempty"`
	ToolCalls  []openrouterToolCallFull `json:"tool_calls,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
}

type openrouterToolCallFull struct {
	ID       string                       `json:"id,omitempty"`
	Type     string                       `json:"type,omitempty"`
	Function openrouterToolCallFunction   `json:"function"`
}

type openrouterToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openrouterToolStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string                          `json:"content"`
			ToolCalls []openrouterToolCallDelta        `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openrouterToolCallDelta struct {
	Index    int                               `json:"index"`
	ID       string                            `json:"id,omitempty"`
	Type     string                            `json:"type,omitempty"`
	Function openrouterToolCallFunctionDelta   `json:"function"`
}

type openrouterToolCallFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func toOpenRouterMessages(messages []types.Message) []openrouterMessageFull {
	result := make([]openrouterMessageFull, 0, len(messages))
	for _, m := range messages {
		msg := openrouterMessageFull{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, openrouterToolCallFull{
				ID:   tc.ID,
				Type: "function",
				Function: openrouterToolCallFunction{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		if msg.Role == string(types.RoleTool) && msg.Content != "" && msg.ToolCallID != "" {
			result = append(result, msg)
		} else if msg.Role != string(types.RoleTool) {
			result = append(result, msg)
		}
	}
	return result
}

func (p *OpenRouterProvider) Stream(ctx context.Context, messages []types.Message, cb func(chunk string)) error {
	msgs := make([]openrouterMessageFull, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, openrouterMessageFull{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	reqBody := openrouterToolRequest{
		Model:    p.Model,
		Messages: msgs,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("HTTP-Referer", "https://fm-my-canvas.app")
	req.Header.Set("X-Title", "FM My Canvas")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		scanner := bufio.NewScanner(resp.Body)
		var body string
		if scanner.Scan() {
			body = scanner.Text()
		}
		return fmt.Errorf("openrouter returned status %d: %s", resp.StatusCode, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openrouterToolStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			cb(chunk.Choices[0].Delta.Content)
		}
	}

	return scanner.Err()
}

type toolCallAccumulator struct {
	ID        string
	Name      string
	Arguments string
}

func (p *OpenRouterProvider) StreamWithTools(ctx context.Context, messages []types.Message, tools []ToolDefinition, cb func(event StreamEvent)) error {
	reqBody := openrouterToolRequest{
		Model:    p.Model,
		Messages: toOpenRouterMessages(messages),
		Stream:   true,
		Tools:    tools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("HTTP-Referer", "https://fm-my-canvas.app")
	req.Header.Set("X-Title", "FM My Canvas")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		scanner := bufio.NewScanner(resp.Body)
		var body string
		if scanner.Scan() {
			body = scanner.Text()
		}
		return fmt.Errorf("openrouter returned status %d: %s", resp.StatusCode, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	accumulators := make(map[int]*toolCallAccumulator)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			cb(StreamEvent{Type: EventDone})
			break
		}

		var chunk openrouterToolStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		if choice.Delta.Content != "" {
			cb(StreamEvent{
				Type:    EventContent,
				Content: choice.Delta.Content,
			})
		}

		for _, tcDelta := range choice.Delta.ToolCalls {
			idx := tcDelta.Index
			acc, ok := accumulators[idx]
			if !ok {
				acc = &toolCallAccumulator{}
				accumulators[idx] = acc
			}
			if tcDelta.ID != "" {
				acc.ID = tcDelta.ID
			}
			if tcDelta.Function.Name != "" {
				acc.Name = tcDelta.Function.Name
			}
			if tcDelta.Function.Arguments != "" {
				acc.Arguments += tcDelta.Function.Arguments
			}
		}

		if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
			var toolCalls []types.ToolCall
			for i := 0; i < len(accumulators); i++ {
				acc, ok := accumulators[i]
				if !ok {
					continue
				}
				toolCalls = append(toolCalls, types.ToolCall{
					ID:        acc.ID,
					Name:      acc.Name,
					Arguments: acc.Arguments,
				})
			}
			if len(toolCalls) > 0 {
				cb(StreamEvent{
					Type:      EventToolCall,
					ToolCalls: toolCalls,
				})
			}
			accumulators = make(map[int]*toolCallAccumulator)
		}
	}

	return scanner.Err()
}
