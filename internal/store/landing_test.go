package store

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"steadwell/internal/channel"
)

func TestLandTelegramUserUsesDefaultOrgAndFreePlan(t *testing.T) {
	ctx := context.Background()
	cat := NewSeeded()

	u, err := cat.LandTelegramUser(ctx, "111", "Ada", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.OrganizationSlug != channel.DefaultOrgSlug {
		t.Fatalf("org slug = %q, want %q", u.OrganizationSlug, channel.DefaultOrgSlug)
	}
	if u.PlanSlug != channel.DefaultPlanSlug {
		t.Fatalf("plan slug = %q, want %q", u.PlanSlug, channel.DefaultPlanSlug)
	}
	if u.ChannelSlug != channel.Telegram {
		t.Fatalf("channel = %q", u.ChannelSlug)
	}
	if u.ParticipantID != "111" {
		t.Fatalf("participant = %q", u.ParticipantID)
	}
}

func TestLandTelegramUserIsIdempotent(t *testing.T) {
	ctx := context.Background()
	cat := NewSeeded()

	a, err := cat.LandTelegramUser(ctx, "111", "Ada", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := cat.LandTelegramUser(ctx, "111", "Ada", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatalf("expected same user id, got %s vs %s", a.ID, b.ID)
	}
}

func TestLandTelegramUserUsesJoinToken(t *testing.T) {
	ctx := context.Background()
	cat := NewSeeded()
	orgID := uuid.NewString()
	token := "join-token-xyz"
	cat.orgs[orgID] = org{ID: orgID, Slug: "partner-a", Name: "Partner A", JoinToken: token}

	u, err := cat.LandTelegramUser(ctx, "222", "Bob", token)
	if err != nil {
		t.Fatal(err)
	}
	if u.OrganizationID != orgID || u.OrganizationSlug != "partner-a" {
		t.Fatalf("org = %s/%s", u.OrganizationID, u.OrganizationSlug)
	}
}
