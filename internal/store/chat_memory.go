package store

import "context"

type memChatMsg struct {
	Role    string
	Content string
}

// Ensure Catalog can hold in-memory chat for tests.
func (c *Catalog) ensureChat() {
	if c.chat == nil {
		c.chat = make(map[string][]memChatMsg)
	}
}

func chatKey(channel, chatID string) string { return channel + "|" + chatID }

func (c *Catalog) RecentChatMessages(_ context.Context, channel, chatID string, limit int) ([]ChatMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureChat()
	if limit <= 0 {
		limit = ChatHistoryLimit
	}
	raw := c.chat[chatKey(channel, chatID)]
	if len(raw) > limit {
		raw = raw[len(raw)-limit:]
	}
	out := make([]ChatMessage, len(raw))
	for i, m := range raw {
		out[i] = ChatMessage{Role: m.Role, Content: m.Content}
	}
	return out, nil
}

func (c *Catalog) AppendChatMessage(_ context.Context, channel, chatID, role, content string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureChat()
	key := chatKey(channel, chatID)
	c.chat[key] = append(c.chat[key], memChatMsg{Role: role, Content: content})
	keep := ChatHistoryLimit * 2
	if len(c.chat[key]) > keep {
		c.chat[key] = c.chat[key][len(c.chat[key])-keep:]
	}
	return nil
}
