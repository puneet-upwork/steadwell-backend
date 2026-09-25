package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (p *Postgres) RecentChatMessages(ctx context.Context, channel, chatID string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = ChatHistoryLimit
	}
	rows, err := p.pool.Query(ctx, `
		SELECT role, content FROM (
			SELECT role, content, created_at
			FROM chat_messages
			WHERE channel = $1 AND chat_id = $2
			ORDER BY created_at DESC
			LIMIT $3
		) recent
		ORDER BY created_at ASC
	`, channel, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (p *Postgres) AppendChatMessage(ctx context.Context, channel, chatID, role, content string) error {
	if role != ChatRoleUser && role != ChatRoleAssistant {
		return fmt.Errorf("chat role %q", role)
	}
	id := uuid.NewString()
	_, err := p.pool.Exec(ctx, `
		INSERT INTO chat_messages (id, channel, chat_id, role, content)
		VALUES ($1, $2, $3, $4, $5)
	`, id, channel, chatID, role, content)
	if err != nil {
		return err
	}
	// Keep a little more than the read window so mid-turn pairs survive trims.
	keep := ChatHistoryLimit * 2
	_, err = p.pool.Exec(ctx, `
		DELETE FROM chat_messages
		WHERE id IN (
			SELECT id FROM chat_messages
			WHERE channel = $1 AND chat_id = $2
			ORDER BY created_at DESC
			OFFSET $3
		)
	`, channel, chatID, keep)
	return err
}
