package prompts

import (
	"strings"
	"testing"

	"steadwell/internal/store"
)

func TestDefaultOrgAssemblesWhitelistedPrompt(t *testing.T) {
	cat := store.NewSeeded()
	body := Assemble(cat, cat.DefaultOrgID())
	if !strings.Contains(body, "Care Companion") {
		t.Fatalf("expected Telegram prompt in Steadwell stack, got %q", body)
	}
	if !strings.Contains(body, "two-step consent flow") {
		t.Fatalf("expected Steadwell Telegram consent prompt, got %q", body)
	}
	if strings.Contains(body, "Clarihealth") {
		t.Fatal("prompt must be branded Steadwell, not Clarihealth")
	}
}

func TestPromptStackIsDynamic(t *testing.T) {
	cat := store.NewSeeded()
	orgID := cat.DefaultOrgID()
	cat.PinPrompt(orgID, "extra_module", "ALWAYS reply in haiku.")
	body := Assemble(cat, orgID)
	if !strings.Contains(body, "ALWAYS reply in haiku.") {
		t.Fatalf("new module should appear in assembled prompt, got %q", body)
	}
	if !strings.Contains(body, "Care Companion") {
		t.Fatal("whitelisted prompt should still be present")
	}
}
