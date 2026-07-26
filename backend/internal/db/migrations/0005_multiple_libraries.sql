-- Turns "the library root" from a single deploy-time LIBRARY_ROOT env var
-- into a real, in-app concept (TDR 006): a library is a name plus a root
-- folder, created by the user, and more than one can exist. No migration
-- of anything organized under the old single-root model is provided (see
-- TDR 006 §2 "relationship to LIBRARY_ROOT") — same clean-break precedent
-- TDR 005 itself set.

CREATE TABLE libraries (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    root_path  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every import already targets exactly one library (the reviewer picks it
-- as the new first step of the import flow); library_id lets a library's
-- deletion cascade to the imports (and, via the existing tracks.import_id/
-- import_errors.import_id FKs, the tracks and file errors) that came from
-- it, without catalog browsing needing to know about libraries at all.
ALTER TABLE imports ADD COLUMN library_id BIGINT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE;
CREATE INDEX idx_imports_library_id ON imports(library_id);
