-- Backs FindArtistIDByMusicBrainzID/FindAlbumIDByMusicBrainzID (TDR 017),
-- which the enrichment job runs on every artist/album on every pass to
-- detect duplicates — without an index that's a full table scan per row
-- processed. Partial (excluding NULL) since most rows haven't resolved a
-- MusicBrainz ID yet and would otherwise bloat the index for no benefit.

CREATE INDEX idx_artists_musicbrainz_id ON artists (musicbrainz_id) WHERE musicbrainz_id IS NOT NULL;
CREATE INDEX idx_albums_musicbrainz_id ON albums (musicbrainz_id) WHERE musicbrainz_id IS NOT NULL;
