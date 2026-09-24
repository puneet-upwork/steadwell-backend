package flows

import (
	"context"

	"steadwell/internal/store"
	"steadwell/internal/telegramapi"
)

type Generator interface {
	Generate(ctx context.Context, system, user string) (string, error)
}

type Deps struct {
	Catalog   store.Cataloger
	Telegram  telegramapi.Sender
	Generator Generator
}
