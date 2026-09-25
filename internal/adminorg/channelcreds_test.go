package adminorg

import "testing"

func TestValidateChannelCredsTelegram(t *testing.T) {
	errs := validateChannelCreds("telegram", ChannelInput{Enabled: true}, false)
	if len(errs) != 2 {
		t.Fatalf("want username + token, got %v", errs)
	}
	in := ChannelInput{Enabled: true, BotUsername: "bot", BotToken: "t"}
	if errs := validateChannelCreds("telegram", in, false); len(errs) != 0 {
		t.Fatalf("webhook_secret is optional, got %v", errs)
	}
	keep := ChannelInput{Enabled: true, BotUsername: "bot"}
	if errs := validateChannelCreds("telegram", keep, true); len(errs) != 0 {
		t.Fatalf("existing should skip secrets, got %v", errs)
	}
}

func TestValidateChannelCredsDisabled(t *testing.T) {
	if errs := validateChannelCreds("telegram", ChannelInput{}, false); len(errs) != 0 {
		t.Fatalf("disabled needs no creds: %v", errs)
	}
}

func TestValidateChannelCredsWhatsAppAndLine(t *testing.T) {
	if errs := validateChannelCreds("whatsapp", ChannelInput{Enabled: true}, false); len(errs) != 2 {
		t.Fatalf("want phone + token, got %v", errs)
	}
	if errs := validateChannelCreds("line", ChannelInput{Enabled: true, LiffURL: "https://liff.line.me/x"}, false); len(errs) != 2 {
		t.Fatalf("want line secrets, got %v", errs)
	}
	okWA := ChannelInput{Enabled: true, PhoneNumber: "1555", APIToken: "t"}
	if errs := validateChannelCreds("whatsapp", okWA, false); len(errs) != 0 {
		t.Fatalf("got %v", errs)
	}
	okLine := ChannelInput{Enabled: true, LiffURL: "https://liff.line.me/x", ChannelSecret: "s", ChannelAccessToken: "a"}
	if errs := validateChannelCreds("line", okLine, false); len(errs) != 0 {
		t.Fatalf("got %v", errs)
	}
}
