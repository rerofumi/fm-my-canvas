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
	APIKey string
	Model  string
}

func NewOpenRouter(apiKey, model string) *OpenRouterProvider {
	return &OpenRouterProvider{
		APIKey: apiKey,
		Model:  model,
	}
}

type openrouterRequest struct {
	Model    string              `json:"model"`
	Messages []openrouterMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type openrouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openrouterStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenRouterProvider) Stream(ctx context.Context, messages []types.Message, cb func(chunk string)) error {
	msgs := make([]openrouterMessage, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, openrouterMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	reqBody := openrouterRequest{
		Model:    p.Model,
		Messages: msgs,
		Stream:   true,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(jsonData))
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

		var chunk openrouterStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			cb(chunk.Choices[0].Delta.Content)
		}
	}

	return scanner.Err()
}
