package telegramflow

import (
	"context"
	"fmt"
	"log/slog"

	"steadwell/internal/channel"
	"steadwell/internal/entitlements"
	"steadwell/internal/flows"
	"steadwell/internal/joinlinks"
	"steadwell/internal/prompts"
	"steadwell/internal/store"
	"steadwell/internal/telegramapi"
)

type Result struct {
	OK             bool          `json:"ok"`
	FeatureBlocked bool          `json:"feature_blocked,omitempty"`
	UserText       string        `json:"user_text,omitempty"`
	SystemPrompt   string        `json:"system_prompt,omitempty"`
	Reply          channel.Reply `json:"reply"`
}

func Run(ctx context.Context, deps *flows.Deps, update telegramapi.Update) (Result, error) {
	var result Result
	if deps == nil || deps.Catalog == nil {
		return result, fmt.Errorf("catalog is not configured")
	}
	in := telegramapi.Inbound(update)
	if in.Identity.ParticipantID == "" {
		return result, fmt.Errorf("missing telegram participant")
	}
	if in.Text == "" && in.MediaKind == channel.MediaText {
		slog.Warn("telegram message.text is empty; put chat and text next to from, not inside from",
			"update_id", in.UpdateID, "participant_id", in.Identity.ParticipantID)
	}

	user, err := deps.Catalog.LandTelegramUser(
		ctx,
		in.Identity.ParticipantID,
		in.Identity.DisplayName,
		joinlinks.TelegramStartToken(in.Text),
	)
	if err != nil {
		return result, err
	}

	if !entitlements.Allows(deps.Catalog, user.OrganizationID, in.MediaKind) {
		result.FeatureBlocked = true
		result.OK = true
		result.Reply = channel.Reply{MessageBody: store.UnsupportedMediaMessage(in.MediaKind)}
		return result, send(ctx, deps, in, result.Reply)
	}

	system := prompts.Assemble(deps.Catalog, user.OrganizationID)
	userText := in.Text
	if userText == "" {
		userText = "(" + in.MediaKind + " message)"
	}
	if deps.Generator == nil {
		return result, fmt.Errorf("llm generator is not configured")
	}
	body, err := deps.Generator.Generate(ctx, system, userText)
	if err != nil {
		return result, err
	}
	result.OK = true
	result.UserText = userText
	result.SystemPrompt = system
	result.Reply = channel.Reply{MessageBody: body}
	slog.Info("telegram reply",
		"update_id", in.UpdateID,
		"participant_id", in.Identity.ParticipantID,
		"org", user.OrganizationSlug,
		"plan", user.PlanSlug,
		"user_text", userText,
		"prompt_chars", len(system),
		"reply", body,
	)
	return result, send(ctx, deps, in, result.Reply)
}

func send(ctx context.Context, deps *flows.Deps, in channel.Inbound, reply channel.Reply) error {
	if reply.SkipSend {
		return nil
	}
	if deps.Telegram == nil {
		return nil
	}
	chatID := in.ChatID
	if chatID == "" {
		chatID = in.Identity.ParticipantID
	}
	return deps.Telegram.Send(ctx, chatID, reply)
}
