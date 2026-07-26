package main

import (
	"net/url"
	"testing"
)

func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestDatabaseURLPrefersExplicitDATABASE_URL(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":      "postgres://explicit:conn@string/db",
		"POSTGRES_USER":     "ignored",
		"POSTGRES_PASSWORD": "ignored",
		"POSTGRES_DB":       "ignored",
	})

	if got := databaseURL(); got != "postgres://explicit:conn@string/db" {
		t.Fatalf("databaseURL() = %q, want the explicit DATABASE_URL unchanged", got)
	}
}

func TestDatabaseURLBuildsFromPostgresParts(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":      "",
		"POSTGRES_USER":     "opusflow",
		"POSTGRES_PASSWORD": "s3cret",
		"POSTGRES_DB":       "opusflow",
	})

	want := "postgres://opusflow:s3cret@postgres:5432/opusflow?sslmode=disable"
	if got := databaseURL(); got != want {
		t.Fatalf("databaseURL() = %q, want %q", got, want)
	}
}

func TestDatabaseURLOmitsPasswordWhenUnset(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":      "",
		"POSTGRES_PASSWORD": "",
		"POSTGRES_USER":     "opusflow",
		"POSTGRES_DB":       "opusflow",
	})

	want := "postgres://opusflow@postgres:5432/opusflow?sslmode=disable"
	if got := databaseURL(); got != want {
		t.Fatalf("databaseURL() = %q, want %q (no password segment — trust auth, nothing to leak into docker-compose.yml)", got, want)
	}

	parsed, err := url.Parse(want)
	if err != nil {
		t.Fatalf("sanity check: %v", err)
	}
	if _, ok := parsed.User.Password(); ok {
		t.Fatal("sanity check: expected the built URL to carry no password")
	}
}

func TestDatabaseURLHonorsCustomHostAndPort(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":      "",
		"POSTGRES_HOST":     "db.internal",
		"POSTGRES_PORT":     "6543",
		"POSTGRES_USER":     "opusflow",
		"POSTGRES_PASSWORD": "s3cret",
		"POSTGRES_DB":       "opusflow",
	})

	want := "postgres://opusflow:s3cret@db.internal:6543/opusflow?sslmode=disable"
	if got := databaseURL(); got != want {
		t.Fatalf("databaseURL() = %q, want %q", got, want)
	}
}

func TestDatabaseURLRoundTripsSpecialCharactersInPassword(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":      "",
		"POSTGRES_USER":     "opusflow",
		"POSTGRES_PASSWORD": "p@ss/word? #&=",
		"POSTGRES_DB":       "opusflow",
	})

	got := databaseURL()
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("databaseURL() produced an unparseable URL %q: %v", got, err)
	}
	if user := parsed.User.Username(); user != "opusflow" {
		t.Fatalf("parsed username = %q, want %q (url = %q)", user, "opusflow", got)
	}
	password, ok := parsed.User.Password()
	if !ok || password != "p@ss/word? #&=" {
		t.Fatalf("parsed password = %q (ok=%v), want %q (url = %q)", password, ok, "p@ss/word? #&=", got)
	}
}
