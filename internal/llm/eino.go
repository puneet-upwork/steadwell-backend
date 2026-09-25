package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	openai "github.com/cloudwego/eino-ext/components/model/openai"

	"steadwell/internal/store"
)

// Eino generates replies via cloudwego/eino ChatModel (OpenAI-compat → LiteLLM).
type Eino struct {
	model *openai.ChatModel
}

// EinoConfig configures the OpenAI-compatible ChatModel (LiteLLM, OpenAI, etc.).
type EinoConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// NewEino builds an Eino-backed Generator pointing at an OpenAI-compatible endpoint.
func NewEino(ctx context.Context, cfg EinoConfig) (*Eino, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	model := strings.TrimSpace(cfg.Model)
	if base == "" {
		return nil, fmt.Errorf("llm: BaseURL is required")
	}
	if model == "" {
		return nil, fmt.Errorf("llm: Model is required")
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:     cfg.APIKey,
		BaseURL:    base,
		Model:      model,
		Timeout:    timeout,
		HTTPClient: cfg.HTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: eino openai chat model: %w", err)
	}
	return &Eino{model: cm}, nil
}

func (e *Eino) Generate(ctx context.Context, system string, history []store.ChatMessage, user string) (string, error) {
	if e == nil || e.model == nil {
		return "", fmt.Errorf("llm: eino generator is nil")
	}
	msgs := make([]*schema.Message, 0, 2+len(history))
	msgs = append(msgs, &schema.Message{Role: schema.System, Content: system})
	for _, h := range history {
		switch h.Role {
		case store.ChatRoleAssistant:
			msgs = append(msgs, &schema.Message{Role: schema.Assistant, Content: h.Content})
		default:
			msgs = append(msgs, &schema.Message{Role: schema.User, Content: h.Content})
		}
	}
	msgs = append(msgs, &schema.Message{Role: schema.User, Content: user})
	out, err := e.model.Generate(ctx, msgs)
	if err != nil {
		return "", err
	}
	if out == nil {
		return "", fmt.Errorf("llm: empty chat response")
	}
	return out.Content, nil
}
