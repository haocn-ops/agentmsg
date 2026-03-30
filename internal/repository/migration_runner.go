package repository

import (
	"context"
	"embed"
	"errors"
	"sort"

	"github.com/jmoiron/sqlx"
)

const migrationLockKey int64 = 73194201

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

func RunMigrations(ctx context.Context, db *PostgresDB) error {
	if db == nil || db.db == nil {
		return errors.New("database is not configured")
	}

	if err := ensureMigrationTable(ctx, db.db); err != nil {
		return err
	}

	if _, err := db.db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return err
	}
	defer func() {
		_, _ = db.db.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockKey)
	}()

	entries, err := embeddedMigrations.ReadDir("migrations")
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := migrationApplied(ctx, db.db, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		content, err := embeddedMigrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		tx, err := db.db.BeginTxx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	return err
}

func migrationApplied(ctx context.Context, db *sqlx.DB, filename string) (bool, error) {
	var count int
	if err := db.GetContext(ctx, &count, `SELECT COUNT(1) FROM schema_migrations WHERE filename = $1`, filename); err != nil {
		return false, err
	}
	return count > 0, nil
}
