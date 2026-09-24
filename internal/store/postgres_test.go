package store

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"steadwell/internal/channel"
	"steadwell/internal/dbmigrate"
)

func TestPostgresCatalogUsesMigrationSeed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := dbmigrate.Apply(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cat, err := NewPostgres(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}

	u, err := cat.LandTelegramUser(ctx, "tg-full-test", "Ada", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.OrganizationSlug != channel.DefaultOrgSlug || u.PlanSlug != channel.DefaultPlanSlug {
		t.Fatalf("user %+v", u)
	}
	if !cat.FeatureEnabled(u.OrganizationID, channel.FeatureText) {
		t.Fatal("text should be enabled on free plan")
	}
	if cat.FeatureEnabled(u.OrganizationID, channel.FeatureImage) {
		t.Fatal("image should be off on free plan")
	}

	body := strings.Join(cat.PromptBodies(u.OrganizationID), "\n")
	if !strings.Contains(body, "You are Steadwell") {
		t.Fatalf("expected Steadwell prompt from DB, got %q", body)
	}
	if strings.Contains(body, "Clarihealth") {
		t.Fatal("DB prompt still says Clarihealth")
	}

	again, err := cat.LandTelegramUser(ctx, "tg-full-test", "Ada", "")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != u.ID {
		t.Fatalf("expected same user, %s vs %s", again.ID, u.ID)
	}
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://127.0.0.1:5432/steadwell?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("local postgres not up (createdb steadwell): %v", err)
	}
	return pool
}
