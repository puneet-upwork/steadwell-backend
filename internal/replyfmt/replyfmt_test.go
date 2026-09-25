package replyfmt

import (
	"strings"
	"testing"
)

func TestParseConsentJSON(t *testing.T) {
	raw := `{
  "message_body": "👋 Hi, I'm Steadwell🌿, your personal care companion.\n\nSteadwell is intended for:\n• Adults who are of legal age\n• Users aged 20+ in Thailand\n\nAre you eligible?",
  "button_body": "I confirm I meet the minimum age requirement",
  "consent_type": "age_consent",
  "accept_label": "I Agree",
  "reject_label": "Disagree"
}`
	reply, plain := Parse(raw)
	if reply.ParseMode != "HTML" {
		t.Fatalf("parse_mode = %q", reply.ParseMode)
	}
	if !strings.Contains(reply.MessageBody, "<b>") {
		t.Fatalf("expected bold HTML, got %q", reply.MessageBody)
	}
	if !strings.Contains(reply.MessageBody, "• Adults") {
		t.Fatalf("expected bullet, got %q", reply.MessageBody)
	}
	if reply.ButtonBody == "" {
		t.Fatal("expected button_body")
	}
	if len(reply.Buttons) != 1 || len(reply.Buttons[0]) != 2 {
		t.Fatalf("buttons = %#v", reply.Buttons)
	}
	if reply.Buttons[0][0].CallbackData != CallbackAccept || reply.Buttons[0][1].CallbackData != CallbackReject {
		t.Fatalf("callback data = %#v", reply.Buttons)
	}
	if !strings.Contains(plain, "I'm Steadwell") {
		t.Fatalf("plain = %q", plain)
	}
}

func TestParsePlainText(t *testing.T) {
	reply, plain := Parse("Hello **world**\n- one\n- two")
	if reply.ParseMode != "HTML" {
		t.Fatal(reply.ParseMode)
	}
	if !strings.Contains(reply.MessageBody, "<b>world</b>") {
		t.Fatalf("got %q", reply.MessageBody)
	}
	if !strings.Contains(reply.MessageBody, "• one") {
		t.Fatalf("got %q", reply.MessageBody)
	}
	if len(reply.Buttons) != 0 {
		t.Fatalf("buttons = %#v", reply.Buttons)
	}
	if plain == "" {
		t.Fatal("plain empty")
	}
}

func TestCallbackUserText(t *testing.T) {
	if CallbackUserText(CallbackAccept) != "I Agree" {
		t.Fatal(CallbackUserText(CallbackAccept))
	}
	if CallbackUserText(CallbackReject) != "Disagree" {
		t.Fatal(CallbackUserText(CallbackReject))
	}
}

func TestToHTMLEscapes(t *testing.T) {
	got := ToHTML(`a <b> & c`)
	if strings.Contains(got, "<b>") && !strings.Contains(got, "&lt;") {
		t.Fatalf("unescaped: %q", got)
	}
	if !strings.Contains(got, "&lt;") || !strings.Contains(got, "&amp;") {
		t.Fatalf("got %q", got)
	}
}
