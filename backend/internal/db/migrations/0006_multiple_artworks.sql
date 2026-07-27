-- Turns "one image per artist/album" into a real gallery (TDR 014):
-- artist_photos/album_covers hold as many images as were found/uploaded,
-- each independently removable, with exactly one marked primary per
-- entity (enforced at the application layer, same as this schema's other
-- invariants) for list views/tiles that only have room for one thumbnail.
--
-- art_status stays on artists/albums exactly as it was — it still drives
-- the background enrichment job's "does this still need automatic
-- discovery" scheduling, independent of how many images now exist.

CREATE TABLE artist_photos (
    id           BIGSERIAL PRIMARY KEY,
    artist_id    BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    thumb_path   TEXT NOT NULL,
    full_path    TEXT NOT NULL,
    source       TEXT NOT NULL,
    content_hash TEXT NOT NULL DEFAULT '',
    is_primary   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_artist_photos_artist ON artist_photos (artist_id);

CREATE TABLE album_covers (
    id           BIGSERIAL PRIMARY KEY,
    album_id     BIGINT NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    thumb_path   TEXT NOT NULL,
    full_path    TEXT NOT NULL,
    source       TEXT NOT NULL,
    picture_type TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    is_primary   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_album_covers_album ON album_covers (album_id);

-- Preserve every existing single image as that entity's first, primary
-- gallery entry (AC-9) — nobody's current artwork disappears on upgrade.
INSERT INTO artist_photos (artist_id, thumb_path, full_path, source, is_primary)
SELECT id, photo_thumb_path, photo_path, 'legacy', true FROM artists WHERE photo_path IS NOT NULL;
INSERT INTO album_covers (album_id, thumb_path, full_path, source, is_primary)
SELECT id, cover_thumb_path, cover_path, 'legacy', true FROM albums WHERE cover_path IS NOT NULL;

ALTER TABLE artists DROP COLUMN photo_thumb_path;
ALTER TABLE artists DROP COLUMN photo_path;
ALTER TABLE albums DROP COLUMN cover_thumb_path;
ALTER TABLE albums DROP COLUMN cover_path;
