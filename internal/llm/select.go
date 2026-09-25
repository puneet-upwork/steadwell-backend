package llm

import (
	"context"
	"fmt"
	"strings"

	"steadwell/internal/store"
)

// LiteLLMEnv holds OpenAI-compatible LiteLLM settings.
type LiteLLMEnv struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Generator is the chat completion surface used by flows.
type Generator interface {
	Generate(ctx context.Context, system string, history []store.ChatMessage, user string) (string, error)
}

// SelectGenerator returns Eino when LiteLLM BaseURL+Model are set; otherwise Echo.
func SelectGenerator(ctx context.Context, env LiteLLMEnv) (Generator, string, error) {
	base := strings.TrimSpace(env.BaseURL)
	model := strings.TrimSpace(env.Model)
	if base == "" || model == "" {
		return Echo{}, "echo", nil
	}
	gen, err := NewEino(ctx, EinoConfig{
		BaseURL: base,
		APIKey:  env.APIKey,
		Model:   model,
	})
	if err != nil {
		return nil, "", fmt.Errorf("litellm/eino: %w", err)
	}
	return gen, "eino", nil
}
