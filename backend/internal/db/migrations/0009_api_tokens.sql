-- api_tokens backs the real pairing-token auth (TDR 022) replacing the
-- previous fully client-side fake. token_hash is SHA-256 (auth.HashToken)
-- rather than a per-record-salted password hash (bcrypt/argon2): tokens
-- are pre-generated with crypto/rand at high entropy, not user-chosen, so
-- there's no dictionary/rainbow-table risk a salt would defend against —
-- and a plain hash lets validation do a direct indexed lookup instead of
-- comparing an incoming token against every stored row.
CREATE TABLE api_tokens (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ
);
