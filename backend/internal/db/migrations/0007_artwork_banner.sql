-- Adds a second, independent "banner" designation to the artwork gallery
-- (TDR 016), alongside the existing is_primary — a detail page's header
-- banner is a separate, explicitly-chosen image, not derived from the
-- primary image. No backfill: every existing row starts unflagged, which
-- is exactly the "no banner chosen yet" state; GetArtist/GetAlbum fall
-- back to the primary image as the displayed banner until one is set.

ALTER TABLE artist_photos ADD COLUMN is_banner BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE album_covers  ADD COLUMN is_banner BOOLEAN NOT NULL DEFAULT FALSE;
