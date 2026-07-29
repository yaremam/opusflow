package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrTokenNotFound is returned by Delete when id doesn't match any
// existing token.
var ErrTokenNotFound = errors.New("token not found")

// Token is one row of api_tokens (TDR 022) — the plaintext token itself
// is never stored or returned again after creation; TokenHash is what's
// persisted and compared against.
type Token struct {
	ID         int64
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// Store is Postgres persistence for api_tokens.
type Store struct {
	db *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{db: conn}
}

// Count is how the bootstrap step and the auth middleware both tell
// whether any token exists yet — an empty table means enforcement hasn't
// started (see Bootstrap).
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting tokens: %w", err)
	}
	return n, nil
}

// HasBootstrapped reports whether Bootstrap has ever created the first
// token for this install — distinct from Count() > 0, which only says
// whether a token currently exists (see auth_bootstrap's migration
// comment for why the distinction matters to Middleware).
func (s *Store) HasBootstrapped(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM auth_bootstrap)`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking bootstrap marker: %w", err)
	}
	return exists, nil
}

// MarkBootstrapped records that Bootstrap has run — called exactly once,
// by Bootstrap itself.
func (s *Store) MarkBootstrapped(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO auth_bootstrap DEFAULT VALUES`)
	if err != nil {
		return fmt.Errorf("recording bootstrap marker: %w", err)
	}
	return nil
}

// Create stores a new named token by its hash (see HashToken) — the
// caller is responsible for returning the plaintext to the user exactly
// once, since nothing here ever stores or reproduces it again.
func (s *Store) Create(ctx context.Context, name, tokenHash string) (Token, error) {
	var t Token
	t.Name = name
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO api_tokens (name, token_hash) VALUES ($1, $2)
		RETURNING id, created_at
	`, name, tokenHash).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return Token{}, fmt.Errorf("creating token: %w", err)
	}
	return t, nil
}

// List returns every token, newest first — never the hash or original
// plaintext, just what's needed for the "Paired Devices" list (AC-5).
func (s *Store) List(ctx context.Context) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, created_at, last_used_at FROM api_tokens ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing tokens: %w", err)
	}
	defer rows.Close()

	tokens := []Token{}
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scanning token: %w", err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tokens, nil
}

// Delete revokes a token — any request bearing it starts failing
// immediately afterward (AC-5).
func (s *Store) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deleting token: %w", err)
	}
	if n == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// ValidateAndTouch reports whether tokenHash matches a stored token, and
// if so records that it was just used (surfaced in the "Paired Devices"
// list so a household admin can tell a stale pairing from an active one).
func (s *Store) ValidateAndTouch(ctx context.Context, tokenHash string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE api_tokens SET last_used_at = now() WHERE token_hash = $1
	`, tokenHash)
	if err != nil {
		return false, fmt.Errorf("validating token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("validating token: %w", err)
	}
	return n > 0, nil
}
