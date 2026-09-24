package dbmigrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrateAdvisoryLock serializes Apply across processes (tests and overlapping boots).
const migrateAdvisoryLock int64 = 0x537465616477656c // "Steadwel"

var files = []string{
	"001_init.sql",
	"002_seed_default.sql",
	"003_admin_auth.sql",
	"004_org_commercial.sql",
}

// Dir walks up from cwd (and MIGRATIONS_DIR) to find migrations/.
func Dir() (string, error) {
	if d := os.Getenv("MIGRATIONS_DIR"); d != "" {
		return d, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		p := filepath.Join(dir, "migrations", "001_init.sql")
		if _, err := os.Stat(p); err == nil {
			return filepath.Join(dir, "migrations"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("migrations directory not found (set MIGRATIONS_DIR)")
}

// Apply runs SQL files in order, once each, against Postgres.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("nil pool")
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate lock conn: %w", err)
	}
	defer lockConn.Release()
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrateAdvisoryLock); err != nil {
		return fmt.Errorf("migrate lock: %w", err)
	}
	defer func() {
		_, _ = lockConn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrateAdvisoryLock)
	}()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}
	for _, name := range files {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := execSQL(ctx, pool, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}
	return nil
}

func execSQL(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, err = conn.Conn().PgConn().Exec(ctx, sql).ReadAll()
	return err
}
