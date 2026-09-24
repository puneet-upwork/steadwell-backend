package telegramflow

import (
	"context"
	"strings"
	"testing"

	"steadwell/internal/channel"
	"steadwell/internal/flows"
	"steadwell/internal/store"
	"steadwell/internal/telegramapi"
)

type stubSender struct {
	bodies []string
}

func (s *stubSender) Send(_ context.Context, _ string, reply channel.Reply) error {
	s.bodies = append(s.bodies, reply.MessageBody)
	return nil
}

func (s *stubSender) AnswerCallback(context.Context, string, string, bool) error { return nil }

type stubGen struct{}

func (stubGen) Generate(_ context.Context, system, user string) (string, error) {
	return "SYS:" + system + "|USER:" + user, nil
}

func testDeps(t *testing.T) (*flows.Deps, *store.Catalog, *stubSender) {
	t.Helper()
	cat := store.NewSeeded()
	sender := &stubSender{}
	return &flows.Deps{
		Catalog:   cat,
		Telegram:  sender,
		Generator: stubGen{},
	}, cat, sender
}

func TestTelegramTextUsesWhitelistedPrompt(t *testing.T) {
	deps, _, sender := testDeps(t)
	u, err := telegramapi.ParseJSON([]byte(`{"update_id":1,"message":{"from":{"id":99,"first_name":"Ada"},"chat":{"id":99},"text":"hi"}}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), deps, u)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatal("expected ok")
	}
	if !strings.Contains(res.Reply.MessageBody, "Care Companion") {
		t.Fatalf("reply should include assembled Steadwell prompt, got %q", res.Reply.MessageBody)
	}
	if !strings.Contains(res.Reply.MessageBody, "USER:hi") {
		t.Fatalf("reply should include user text, got %q", res.Reply.MessageBody)
	}
	if !strings.Contains(res.SystemPrompt, "You are Steadwell") {
		t.Fatalf("system_prompt should be the Steadwell stack, got %q", res.SystemPrompt)
	}
	if res.UserText != "hi" {
		t.Fatalf("user_text = %q", res.UserText)
	}
	if len(sender.bodies) != 1 {
		t.Fatalf("sent %d", len(sender.bodies))
	}
}

func TestTelegramImageRejectedOnDefaultOrg(t *testing.T) {
	deps, _, sender := testDeps(t)
	u, err := telegramapi.ParseJSON([]byte(`{"update_id":1,"message":{"from":{"id":99},"chat":{"id":99},"photo":[{"file_id":"p"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), deps, u)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FeatureBlocked {
		t.Fatal("expected feature block")
	}
	if !strings.Contains(strings.ToLower(res.Reply.MessageBody), "image") {
		t.Fatalf("blocked copy = %q", res.Reply.MessageBody)
	}
	if len(sender.bodies) != 1 {
		t.Fatalf("sent %d", len(sender.bodies))
	}
}

func TestTelegramImageAllowedAfterOrgOverride(t *testing.T) {
	deps, cat, _ := testDeps(t)
	cat.SetOrgFeature(cat.DefaultOrgID(), channel.FeatureImage, true)
	u, err := telegramapi.ParseJSON([]byte(`{"update_id":1,"message":{"from":{"id":99},"chat":{"id":99},"caption":"look","photo":[{"file_id":"p"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), deps, u)
	if err != nil {
		t.Fatal(err)
	}
	if res.FeatureBlocked {
		t.Fatal("image should be allowed after override")
	}
	if !res.OK {
		t.Fatal("expected ok")
	}
}
