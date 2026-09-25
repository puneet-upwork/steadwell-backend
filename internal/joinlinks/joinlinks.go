package joinlinks

import (
	"net/url"
	"strings"
)

// Config holds channel deep-link wrappers. Empty fields mean that channel
// cannot build a join URL until configured.
type Config struct {
	TelegramBotUsername string // without @
	WhatsAppNumber      string // E.164 digits, no + (wa.me)
	LineLiffURL         string // https://liff.line.me/{liffId}
}

// FromPublic builds a join config from this org’s public bot fields only.
// Empty fields mean no QR / deep link — global env is not used.
func FromPublic(slug string, pub map[string]string) Config {
	return Config{}.MergePublic(slug, pub)
}

// MergePublic copies org public fields onto a config. Start from Config{} so
// only this org’s values are used.
func (c Config) MergePublic(slug string, pub map[string]string) Config {
	out := c
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "telegram":
		if v := strings.TrimPrefix(strings.TrimSpace(pub["bot_username"]), "@"); v != "" {
			out.TelegramBotUsername = v
		}
	case "whatsapp":
		if v := strings.TrimSpace(pub["phone_number"]); v != "" {
			out.WhatsAppNumber = v
		}
	case "line":
		if v := strings.TrimSpace(pub["liff_url"]); v != "" {
			out.LineLiffURL = v
		}
	}
	return out
}

// ForChannel builds the platform deep link that opens the messaging app
// with the join token. Empty string means not configured or unknown slug.
func ForChannel(cfg Config, slug, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "telegram":
		bot := strings.TrimPrefix(strings.TrimSpace(cfg.TelegramBotUsername), "@")
		if bot == "" {
			return ""
		}
		return "https://t.me/" + bot + "?start=" + url.QueryEscape(token)
	case "whatsapp":
		num := digitsOnly(cfg.WhatsAppNumber)
		if num == "" {
			return ""
		}
		return "https://wa.me/" + num + "?text=" + url.QueryEscape(token)
	case "line":
		base := strings.TrimRight(strings.TrimSpace(cfg.LineLiffURL), "/")
		if base == "" {
			return ""
		}
		sep := "?"
		if strings.Contains(base, "?") {
			sep = "&"
		}
		return base + sep + "liff.state=" + url.QueryEscape(token)
	default:
		return ""
	}
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TelegramStartToken extracts the payload from "/start <token>" (or /start@bot).
func TelegramStartToken(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) < 2 {
		return ""
	}
	cmd := fields[0]
	if cmd == "/start" || strings.HasPrefix(cmd, "/start@") {
		return fields[1]
	}
	return ""
}
