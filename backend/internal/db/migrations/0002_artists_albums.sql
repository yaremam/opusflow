CREATE TABLE artists (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name)
);

CREATE TABLE albums (
    id         BIGSERIAL PRIMARY KEY,
    title      TEXT NOT NULL,
    artist_id  BIGINT NOT NULL REFERENCES artists(id),
    year       INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (title, artist_id)
);
CREATE INDEX idx_albums_artist_id ON albums(artist_id);

ALTER TABLE tracks ADD COLUMN artist_id BIGINT NOT NULL REFERENCES artists(id);
ALTER TABLE tracks ADD COLUMN album_id BIGINT NOT NULL REFERENCES albums(id);
CREATE INDEX idx_tracks_artist_id ON tracks(artist_id);
CREATE INDEX idx_tracks_album_id ON tracks(album_id);
