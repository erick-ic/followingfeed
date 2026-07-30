package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ApplyAll executes every embedded migration that has not yet been recorded.
func ApplyAll(ctx context.Context, db *sql.DB) error {
	all, err := All()
	if err != nil {
		return err
	}
	for _, migration := range all {
		if err = apply(ctx, db, migration); err != nil {
			return err
		}
	}
	return nil
}

func apply(ctx context.Context, db *sql.DB, migration Migration) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(128) NOT NULL PRIMARY KEY,
		applied_at BIGINT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations WHERE version = ?", migration.Version).Scan(&count); err != nil {
		return fmt.Errorf("query migration %s: %w", migration.Version, err)
	}
	if count > 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", migration.Version, err)
	}
	defer tx.Rollback()
	for _, statement := range strings.Split(migration.Script, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Version, err)
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", migration.Version, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.Version, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.Version, err)
	}
	return nil
}
