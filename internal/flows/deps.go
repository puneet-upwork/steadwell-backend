package flows

import (
	"context"

	"steadwell/internal/store"
	"steadwell/internal/telegramapi"
)

type Generator interface {
	Generate(ctx context.Context, system string, history []store.ChatMessage, user string) (string, error)
}

type Deps struct {
	Catalog   store.Cataloger
	Chat      store.ChatHistory
	Telegram  telegramapi.Sender
	Generator Generator
}
