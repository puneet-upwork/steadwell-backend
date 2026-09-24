package dbmigrate

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func TestApplyCreatesSteadwellSeed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := Apply(ctx, pool); err != nil {
		t.Fatalf("apply: %v", err)
	}

	var slug, key, body string
	if err := pool.QueryRow(ctx, `SELECT slug FROM organizations WHERE slug = 'steadwell'`).Scan(&slug); err != nil {
		t.Fatalf("organizations seed: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT key FROM prompt_modules WHERE key = 'prompt'`).Scan(&key); err != nil {
		t.Fatalf("prompt module: %v", err)
	}
	err := pool.QueryRow(ctx, `
		SELECT pv.body
		FROM organization_prompt_stack s
		JOIN organizations o ON o.id = s.organization_id
		JOIN prompt_versions pv ON pv.id = s.version_id
		WHERE o.slug = 'steadwell' AND s.enabled
		ORDER BY s.sort_order
		LIMIT 1`).Scan(&body)
	if err != nil {
		t.Fatalf("prompt stack: %v", err)
	}
	if body == "" {
		t.Fatal("expected seeded prompt body")
	}

	var adminEmail string
	if err := pool.QueryRow(ctx, `SELECT email FROM admin_users WHERE email = 'admin@steadwell.local'`).Scan(&adminEmail); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
}

func TestApplyIsSafeConcurrently(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	errCh := make(chan error, 2)
	for range 2 {
		go func() {
			errCh <- Apply(ctx, pool)
		}()
	}
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent apply: %v", err)
		}
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
