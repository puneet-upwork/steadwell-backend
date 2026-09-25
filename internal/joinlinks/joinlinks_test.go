package joinlinks

import "testing"

func TestForChannel(t *testing.T) {
	cfg := Config{
		TelegramBotUsername: "@SteadwellBot",
		WhatsAppNumber:      "+1 (555) 123-4567",
		LineLiffURL:         "https://liff.line.me/123-abc",
	}
	token := "tok-1"

	if got := ForChannel(cfg, "telegram", token); got != "https://t.me/SteadwellBot?start=tok-1" {
		t.Fatalf("telegram %q", got)
	}
	if got := ForChannel(cfg, "whatsapp", token); got != "https://wa.me/15551234567?text=tok-1" {
		t.Fatalf("whatsapp %q", got)
	}
	if got := ForChannel(cfg, "line", token); got != "https://liff.line.me/123-abc?liff.state=tok-1" {
		t.Fatalf("line %q", got)
	}
	if got := ForChannel(Config{}, "telegram", token); got != "" {
		t.Fatalf("expected empty without bot, got %q", got)
	}
	org := FromPublic("telegram", map[string]string{"bot_username": "orgbot"})
	if got := ForChannel(org, "telegram", token); got != "https://t.me/orgbot?start=tok-1" {
		t.Fatalf("org telegram %q", got)
	}
	if got := ForChannel(FromPublic("telegram", nil), "telegram", token); got != "" {
		t.Fatalf("no org bot must not use env, got %q", got)
	}
}

func TestTelegramStartToken(t *testing.T) {
	if got := TelegramStartToken("/start abc-123"); got != "abc-123" {
		t.Fatalf("got %q", got)
	}
	if got := TelegramStartToken("/start@SteadwellBot abc-123"); got != "abc-123" {
		t.Fatalf("got %q", got)
	}
	if got := TelegramStartToken("hello"); got != "" {
		t.Fatalf("got %q", got)
	}
}
