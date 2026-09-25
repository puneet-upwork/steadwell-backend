package store

import "context"

const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"
	// ChatHistoryLimit is how many prior turns are loaded for the model (not including the new user message).
	ChatHistoryLimit = 10
)

// ChatMessage is one stored conversation turn.
type ChatMessage struct {
	Role    string
	Content string
}

// ChatHistory loads and appends short per-chat transcripts for LLM context.
type ChatHistory interface {
	RecentChatMessages(ctx context.Context, channel, chatID string, limit int) ([]ChatMessage, error)
	AppendChatMessage(ctx context.Context, channel, chatID, role, content string) error
}
