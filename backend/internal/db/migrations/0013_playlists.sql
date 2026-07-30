-- playlists (TDR 028) — household-shared, no per-user ownership: no
-- identity/profile system exists yet, matching every other collection
-- in this app (libraries, paired devices).
CREATE TABLE playlists (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- playlist_tracks is addressed by its own id (not track_id) so a track can
-- appear more than once in the same playlist (AC-6, matching addToQueue's
-- own no-dedup rule) with each occurrence still individually removable/
-- reorderable. ON DELETE CASCADE on both FKs: deleting a playlist drops
-- its rows, and deleting a track from the library (directly or via its
-- artist/album) removes it from every playlist automatically.
CREATE TABLE playlist_tracks (
    id          BIGSERIAL PRIMARY KEY,
    playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id    BIGINT NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    position    INT NOT NULL,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX playlist_tracks_playlist_id_position_idx ON playlist_tracks(playlist_id, position);
