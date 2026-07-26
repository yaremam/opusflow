package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"
)

func randomSuffix() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// testDB opens a connection to a real Postgres instance for integration
// testing, skipping the test if DATABASE_URL isn't configured. Each test
// gets its own freshly-migrated, empty schema via a per-test schema search
// path so tests don't interfere with each other.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration test")
	}

	conn, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	schema := "test_" + randomSuffix()
	if _, err := conn.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec("DROP SCHEMA " + schema + " CASCADE")
	})
	if _, err := conn.Exec("SET search_path TO " + schema); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	return conn
}

func TestMigrateCreatesSchema(t *testing.T) {
	conn := testDB(t)

	if err := Migrate(conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, table := range []string{"imports", "tracks", "import_errors", "schema_migrations"} {
		var exists bool
		err := conn.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("checking table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s was not created by Migrate", table)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	conn := testDB(t)

	if err := Migrate(conn); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(conn); err != nil {
		t.Fatalf("second Migrate (should be a no-op): %v", err)
	}
}
