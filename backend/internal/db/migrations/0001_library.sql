CREATE TABLE library_directories (
    id              BIGSERIAL PRIMARY KEY,
    root            TEXT NOT NULL,
    path            TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'scanning',
    files_processed INTEGER NOT NULL DEFAULT 0,
    files_total     INTEGER NOT NULL DEFAULT 0,
    error           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tracks (
    id               BIGSERIAL PRIMARY KEY,
    directory_id     BIGINT NOT NULL REFERENCES library_directories(id) ON DELETE CASCADE,
    path             TEXT NOT NULL,
    title            TEXT NOT NULL,
    artist           TEXT NOT NULL DEFAULT '',
    album            TEXT NOT NULL DEFAULT '',
    track_number     INTEGER NOT NULL DEFAULT 0,
    year             INTEGER NOT NULL DEFAULT 0,
    genre            TEXT NOT NULL DEFAULT '',
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tracks_directory_id ON tracks(directory_id);

CREATE TABLE library_scan_errors (
    id           BIGSERIAL PRIMARY KEY,
    directory_id BIGINT NOT NULL REFERENCES library_directories(id) ON DELETE CASCADE,
    path         TEXT NOT NULL,
    error        TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_scan_errors_directory_id ON library_scan_errors(directory_id);
