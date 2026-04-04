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
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

func (p *OllamaProvider) Stream(ctx context.Context, messages []types.Message, cb func(chunk string)) error {
	ollamaMsgs := make([]ollamaMessage, 0, len(messages))
	for _, m := range messages {
		ollamaMsgs = append(ollamaMsgs, ollamaMessage{
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
		var chunk ollamaResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			cb(chunk.Message.Content)
		}
	}

	return scanner.Err()
}
