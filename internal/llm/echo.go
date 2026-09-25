package llm

import (
	"context"

	"steadwell/internal/store"
)

// Echo replies without a model. Used when LITELLM_BASE_URL / LITELLM_MODEL are unset.
type Echo struct{}

func (Echo) Generate(_ context.Context, _ string, _ []store.ChatMessage, user string) (string, error) {
	return "I received your message: " + user, nil
}
