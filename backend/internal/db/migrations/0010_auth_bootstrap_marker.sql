-- auth_bootstrap records that this install has been bootstrapped (TDR
-- 022) — written once, the moment Bootstrap first creates a token, and
-- never cleared. It exists to distinguish "no tokens yet because this is
-- a fresh install" (api_tokens empty, never bootstrapped: auth
-- middleware should let requests through) from "no tokens because
-- someone deleted them all after auth was already enabled" (api_tokens
-- also empty, but bootstrapped: auth middleware must still reject every
-- request, not reopen the API). Row count in api_tokens alone can't tell
-- these two states apart.
CREATE TABLE auth_bootstrap (
    id              BIGSERIAL PRIMARY KEY,
    bootstrapped_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
