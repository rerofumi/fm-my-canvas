package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Provider         string `json:"provider"`
	OllamaEndpoint   string `json:"ollama_endpoint"`
	OllamaModel      string `json:"ollama_model"`
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	OpenRouterModel  string `json:"openrouter_model"`
	AgentMode        bool   `json:"agent_mode"`
}

func DefaultConfig() *Config {
	return &Config{
		Provider:       "ollama",
		OllamaEndpoint: "http://localhost:11434",
		OllamaModel:    "llama3",
	}
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "fm-my-canvas"), nil
}

func Load() (*Config, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			_ = cfg.Save()
			return cfg, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}
