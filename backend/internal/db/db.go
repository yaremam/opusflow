// Package db provides the application's Postgres connection and schema
// migrations.
package db

import (
	"database/sql"

	_ "github.com/lib/pq"
)

// Open opens a Postgres connection pool and verifies it's reachable.
func Open(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Connect opens a Postgres connection pool without verifying it's
// reachable yet — sql.Open only validates its DSN and connects lazily on
// first use. Callers that need to keep serving other work (like a health
// endpoint) while Postgres is still coming up should use this instead of
// Open, and confirm connectivity separately (e.g. via Migrate, which will
// surface a connection failure on its first query).
func Connect(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}
