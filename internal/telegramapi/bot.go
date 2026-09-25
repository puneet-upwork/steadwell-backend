package telegramapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"steadwell/internal/channel"
)

const apiBase = "https://api.telegram.org/bot"

// Bot sends outbound messages via the Telegram Bot API.
type Bot struct {
	token      string
	httpClient *http.Client
}

func NewBot(token string) *Bot {
	return &Bot{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

var _ Sender = (*Bot)(nil)

func (b *Bot) Send(ctx context.Context, chatID string, reply channel.Reply) error {
	if reply.MessageBody == "" && reply.ButtonBody == "" {
		return nil
	}
	if reply.MessageBody != "" {
		body := map[string]any{
			"chat_id": chatID,
			"text":    reply.MessageBody,
		}
		if reply.ParseMode != "" {
			body["parse_mode"] = reply.ParseMode
		}
		// Buttons alone on the first bubble when there is no separate button_body.
		if len(reply.Buttons) > 0 && reply.ButtonBody == "" {
			body["reply_markup"] = inlineKeyboard(reply.Buttons)
		}
		if err := b.post(ctx, "sendMessage", body); err != nil {
			return err
		}
	}
	if reply.ButtonBody != "" || (reply.MessageBody == "" && len(reply.Buttons) > 0) {
		text := reply.ButtonBody
		if text == "" {
			text = "Please choose:"
		}
		body := map[string]any{
			"chat_id": chatID,
			"text":    text,
		}
		if reply.ParseMode != "" {
			body["parse_mode"] = reply.ParseMode
		}
		if len(reply.Buttons) > 0 {
			body["reply_markup"] = inlineKeyboard(reply.Buttons)
		}
		if err := b.post(ctx, "sendMessage", body); err != nil {
			return err
		}
	}
	return nil
}

func inlineKeyboard(rows [][]channel.InlineButton) map[string]any {
	out := make([][]map[string]string, len(rows))
	for i, row := range rows {
		out[i] = make([]map[string]string, len(row))
		for j, btn := range row {
			out[i][j] = map[string]string{
				"text":          btn.Text,
				"callback_data": btn.CallbackData,
			}
		}
	}
	return map[string]any{"inline_keyboard": out}
}

func (b *Bot) AnswerCallback(ctx context.Context, callbackID, text string, showAlert bool) error {
	if callbackID == "" {
		return nil
	}
	body := map[string]any{"callback_query_id": callbackID}
	if text != "" {
		body["text"] = text
		body["show_alert"] = showAlert
	}
	return b.post(ctx, "answerCallbackQuery", body)
}

func (b *Bot) post(ctx context.Context, method string, body map[string]any) error {
	if b == nil || b.token == "" {
		return fmt.Errorf("telegram bot token not configured")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+b.token+"/"+method, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram %s: %s %s", method, resp.Status, string(respBody))
	}
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("telegram %s: %s", method, out.Description)
	}
	return nil
}
