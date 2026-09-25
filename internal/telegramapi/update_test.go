package telegramapi

import (
	"testing"

	"steadwell/internal/channel"
)

func TestInboundIgnoresTextNestedInsideFrom(t *testing.T) {
	u, err := ParseJSON([]byte(`{"update_id":1,"message":{"from":{"id":42,"first_name":"Shanu","chat":{"id":42},"text":"hi"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	in := Inbound(u)
	if in.Text != "" {
		t.Fatalf("text nested under from must be ignored, got %q", in.Text)
	}
	if in.ChatID != "" {
		t.Fatalf("chat nested under from must be ignored, got %q", in.ChatID)
	}
	if in.Identity.ParticipantID != "42" {
		t.Fatalf("participant = %q", in.Identity.ParticipantID)
	}
}

func TestInboundText(t *testing.T) {
	u, err := ParseJSON([]byte(`{"update_id":1,"message":{"message_id":2,"from":{"id":111,"first_name":"Ada"},"chat":{"id":111},"text":"hello"}}`))
	if err != nil {
		t.Fatal(err)
	}
	in := Inbound(u)
	if in.Text != "hello" {
		t.Fatalf("text = %q", in.Text)
	}
	if in.MediaKind != channel.MediaText {
		t.Fatalf("media = %q", in.MediaKind)
	}
	if in.Identity.ParticipantID != "111" {
		t.Fatalf("participant = %q", in.Identity.ParticipantID)
	}
}

func TestInboundPhotoIsImage(t *testing.T) {
	u, err := ParseJSON([]byte(`{"update_id":1,"message":{"from":{"id":1},"chat":{"id":1},"photo":[{"file_id":"p"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	in := Inbound(u)
	if in.MediaKind != channel.MediaImage {
		t.Fatalf("media = %q", in.MediaKind)
	}
}

func TestInboundVoiceIsAudio(t *testing.T) {
	u, err := ParseJSON([]byte(`{"update_id":1,"message":{"from":{"id":1},"chat":{"id":1},"voice":{"file_id":"v"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if Inbound(u).MediaKind != channel.MediaAudio {
		t.Fatalf("media = %q", Inbound(u).MediaKind)
	}
}

func TestInboundVideo(t *testing.T) {
	u, err := ParseJSON([]byte(`{"update_id":1,"message":{"from":{"id":1},"chat":{"id":1},"video":{"file_id":"vid"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if Inbound(u).MediaKind != channel.MediaVideo {
		t.Fatalf("media = %q", Inbound(u).MediaKind)
	}
}

func TestInboundCallbackAgree(t *testing.T) {
	u, err := ParseJSON([]byte(`{"update_id":9,"callback_query":{"id":"cb1","from":{"id":55,"first_name":"Ada"},"message":{"message_id":3,"chat":{"id":55},"text":"confirm"},"data":"sw:accept"}}`))
	if err != nil {
		t.Fatal(err)
	}
	in := Inbound(u)
	if in.CallbackID != "cb1" {
		t.Fatalf("callback_id = %q", in.CallbackID)
	}
	if in.Text != "I Agree" {
		t.Fatalf("text = %q", in.Text)
	}
	if in.Identity.ParticipantID != "55" || in.ChatID != "55" {
		t.Fatalf("ids participant=%q chat=%q", in.Identity.ParticipantID, in.ChatID)
	}
}
