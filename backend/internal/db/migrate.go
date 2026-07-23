package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies any embedded migration files not yet recorded in
// schema_migrations, in filename order. It's safe to call repeatedly —
// already-applied migrations are skipped.
func Migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		version := entry.Name()

		var applied bool
		if err := conn.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
		).Scan(&applied); err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", version, err)
		}

		if err := applyMigration(conn, version, string(sqlBytes)); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(conn *sql.DB, version, sqlText string) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for migration %s: %w", version, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(sqlText); err != nil {
		return fmt.Errorf("applying migration %s: %w", version, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("recording migration %s: %w", version, err)
	}
	return tx.Commit()
}
