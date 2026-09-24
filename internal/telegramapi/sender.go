package telegramapi

import (
	"context"

	"steadwell/internal/channel"
)

type Sender interface {
	Send(ctx context.Context, chatID string, reply channel.Reply) error
	AnswerCallback(ctx context.Context, callbackID, text string, showAlert bool) error
}
