-- file_size_bytes backs the mobile app's format/quality indicator (TDR
-- 027) — average bitrate is derived at read time as
-- file_size_bytes * 8 / duration_seconds / 1000, one formula across every
-- supported format rather than per-format bitrate parsing. Nullable: only
-- newly-scanned tracks get it; backfilling existing libraries via a
-- rescan is out of scope here, same as any other addition that only
-- benefits newly-imported files.
ALTER TABLE tracks ADD COLUMN file_size_bytes BIGINT;
