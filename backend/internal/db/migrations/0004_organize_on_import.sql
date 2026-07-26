-- Replaces the "add a directory, scan its files in place" model (TDR
-- 001/002) with organize-on-import (TDR 005): opusflow now copies files
-- into LIBRARY_ROOT itself, renaming them into a canonical structure,
-- rather than reading them wherever a user's own folder layout left them.
-- Clean break, no migration of anything registered under the old model
-- (see TDR 005 §2 "relationship to the existing directory/scan model").

DROP TABLE library_scan_errors;
DROP TABLE library_directories CASCADE;

CREATE TABLE imports (
    id                BIGSERIAL PRIMARY KEY,
    source_description TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'copying',
    files_processed   INTEGER NOT NULL DEFAULT 0,
    files_total       INTEGER NOT NULL DEFAULT 0,
    error             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE import_errors (
    id         BIGSERIAL PRIMARY KEY,
    import_id  BIGINT NOT NULL REFERENCES imports(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    error      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_import_errors_import_id ON import_errors(import_id);

-- tracks.directory_id's own FK (and the table it pointed at) is gone via
-- the CASCADE above; the column survives as a plain BIGINT until renamed
-- and re-constrained here, now pointing at the batch that copied it in
-- rather than a directory it lives inside.
ALTER TABLE tracks RENAME COLUMN directory_id TO import_id;
ALTER TABLE tracks ADD CONSTRAINT tracks_import_id_fkey
    FOREIGN KEY (import_id) REFERENCES imports(id) ON DELETE CASCADE;
ALTER INDEX idx_tracks_directory_id RENAME TO idx_tracks_import_id;

-- Direct artist/album removal (AC-13) didn't exist before this feature —
-- an artist/album only ever disappeared as a side effect of removing the
-- last directory referencing it (see the now-dropped
-- deleteOrphanedCatalogEntries). Now that opusflow owns the files it
-- organizes, artists/albums can be removed directly, which needs an
-- explicit cascade through their tracks — the original 0002 FKs had none.
ALTER TABLE tracks DROP CONSTRAINT tracks_artist_id_fkey;
ALTER TABLE tracks ADD CONSTRAINT tracks_artist_id_fkey
    FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE;
ALTER TABLE tracks DROP CONSTRAINT tracks_album_id_fkey;
ALTER TABLE tracks ADD CONSTRAINT tracks_album_id_fkey
    FOREIGN KEY (album_id) REFERENCES albums(id) ON DELETE CASCADE;
ALTER TABLE albums DROP CONSTRAINT albums_artist_id_fkey;
ALTER TABLE albums ADD CONSTRAINT albums_artist_id_fkey
    FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE;
