CREATE TYPE enrich_status AS ENUM ('pending', 'found', 'not_found', 'failed');

ALTER TABLE artists ADD COLUMN musicbrainz_id TEXT;

ALTER TABLE artists ADD COLUMN photo_thumb_path TEXT;
ALTER TABLE artists ADD COLUMN photo_path TEXT;
ALTER TABLE artists ADD COLUMN art_status enrich_status NOT NULL DEFAULT 'pending';

ALTER TABLE artists ADD COLUMN formed_year INTEGER NOT NULL DEFAULT 0;
ALTER TABLE artists ADD COLUMN country TEXT NOT NULL DEFAULT '';
ALTER TABLE artists ADD COLUMN genres TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE artists ADD COLUMN facts_status enrich_status NOT NULL DEFAULT 'pending';

ALTER TABLE artists ADD COLUMN bio TEXT NOT NULL DEFAULT '';
ALTER TABLE artists ADD COLUMN bio_source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE artists ADD COLUMN bio_status enrich_status NOT NULL DEFAULT 'pending';

ALTER TABLE albums ADD COLUMN musicbrainz_id TEXT;

ALTER TABLE albums ADD COLUMN cover_thumb_path TEXT;
ALTER TABLE albums ADD COLUMN cover_path TEXT;
ALTER TABLE albums ADD COLUMN art_status enrich_status NOT NULL DEFAULT 'pending';

ALTER TABLE albums ADD COLUMN label TEXT NOT NULL DEFAULT '';
ALTER TABLE albums ADD COLUMN country TEXT NOT NULL DEFAULT '';
ALTER TABLE albums ADD COLUMN genres TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE albums ADD COLUMN facts_status enrich_status NOT NULL DEFAULT 'pending';

ALTER TABLE albums ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE albums ADD COLUMN description_source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE albums ADD COLUMN description_status enrich_status NOT NULL DEFAULT 'pending';

-- Unknown Artist/Album rows (empty name) are permanently skipped (AC-3):
-- seed them as not_found up front so the enrichment job's WHERE clause
-- never has to special-case an empty name — it just never sees a pending
-- row for them.
UPDATE artists SET art_status = 'not_found', facts_status = 'not_found', bio_status = 'not_found' WHERE name = '';
UPDATE albums SET art_status = 'not_found', facts_status = 'not_found', description_status = 'not_found' WHERE title = '';

CREATE INDEX idx_artists_enrich_pending ON artists (id) WHERE art_status IN ('pending', 'failed') OR facts_status IN ('pending', 'failed') OR bio_status IN ('pending', 'failed');
CREATE INDEX idx_albums_enrich_pending ON albums (id) WHERE art_status IN ('pending', 'failed') OR facts_status IN ('pending', 'failed') OR description_status IN ('pending', 'failed');
