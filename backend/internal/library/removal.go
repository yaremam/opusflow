package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// DeleteArtist removes an artist and, via ON DELETE CASCADE, every album
// and track attributed to them. If deleteFiles is true, each track's file
// on disk is removed too, after the database rows are gone — AC-13 always
// requires an explicit choice here, never a silent default either way.
func (s *Store) DeleteArtist(ctx context.Context, id int64, deleteFiles bool) error {
	return s.deleteWithFiles(ctx, deleteFiles,
		`SELECT path FROM tracks WHERE artist_id = $1`,
		`DELETE FROM artists WHERE id = $1`,
		id, ErrArtistNotFound)
}

// DeleteAlbum removes an album and, via ON DELETE CASCADE, every track on
// it. See DeleteArtist for the deleteFiles behavior.
func (s *Store) DeleteAlbum(ctx context.Context, id int64, deleteFiles bool) error {
	return s.deleteWithFiles(ctx, deleteFiles,
		`SELECT path FROM tracks WHERE album_id = $1`,
		`DELETE FROM albums WHERE id = $1`,
		id, ErrAlbumNotFound)
}

// DeleteLibrary removes a library and, via ON DELETE CASCADE, every import
// (and, via the existing tracks.import_id/import_errors.import_id FKs,
// every track and file error) that came from it. Unlike DeleteArtist/
// DeleteAlbum, this can leave an artist/album with zero tracks behind — a
// library's tracks are never its own dedicated rows, they're shared catalog
// rows also reachable from any other library — so this sweeps those
// orphans away too, inside the same transaction. deleteFiles behaves the
// same as DeleteArtist/DeleteAlbum: an explicit, required choice, never a
// silent default.
func (s *Store) DeleteLibrary(ctx context.Context, id int64, deleteFiles bool) error {
	var paths []string
	if deleteFiles {
		rows, err := s.db.QueryContext(ctx, `
			SELECT t.path FROM tracks t JOIN imports i ON i.id = t.import_id WHERE i.library_id = $1
		`, id)
		if err != nil {
			return fmt.Errorf("listing track paths: %w", err)
		}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				return err
			}
			paths = append(paths, p)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM libraries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting library: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLibraryNotFound
	}

	if err := deleteOrphanedCatalogEntries(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing library delete: %w", err)
	}

	removeFilesAndEmptyDirs(paths)
	return nil
}

// deleteOrphanedCatalogEntries removes any artist/album row left with zero
// tracks — called only from DeleteLibrary, inside its transaction, never
// automatically from anywhere else (unlike TDR 001's original scan-removal
// behavior, deliberately not reinstated in general by TDR 005). A global
// sweep rather than one scoped to the deleted library: correct either way,
// since the condition ("no track left in the whole library referencing this
// row") is the same, and it's simpler than threading the set of affected
// artist/album IDs out of the cascade delete.
func deleteOrphanedCatalogEntries(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM albums WHERE id NOT IN (SELECT DISTINCT album_id FROM tracks)
	`); err != nil {
		return fmt.Errorf("deleting orphaned albums: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM artists WHERE id NOT IN (SELECT DISTINCT artist_id FROM tracks)
	`); err != nil {
		return fmt.Errorf("deleting orphaned artists: %w", err)
	}
	return nil
}

// deleteWithFiles is the shared shape behind DeleteArtist/DeleteAlbum: list
// affected tracks' paths (if their files are to be removed too), delete the
// row (cascading to its tracks), then — only once the database has
// committed to the deletion — remove those files from disk. A file that's
// already gone or otherwise fails to delete is logged, not returned: the
// catalog deletion already succeeded, which is the part callers depend on.
func (s *Store) deleteWithFiles(ctx context.Context, deleteFiles bool, pathQuery, deleteQuery string, id int64, notFound error) error {
	var paths []string
	if deleteFiles {
		rows, err := s.db.QueryContext(ctx, pathQuery, id)
		if err != nil {
			return fmt.Errorf("listing track paths: %w", err)
		}
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				return err
			}
			paths = append(paths, p)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}

	res, err := s.db.ExecContext(ctx, deleteQuery, id)
	if err != nil {
		return fmt.Errorf("deleting: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}

	removeFilesAndEmptyDirs(paths)
	return nil
}

// removeFilesAndEmptyDirs removes each file, then cleans up the
// <Artist>/<Year>.<Album>/ and <Artist>/ directories organize's canonical
// layout (TDR 005) always creates around a track — exactly two levels
// above it, never further, so a library's root is never at risk even if
// walked all the way up (GitHub issue #7: a deleted artist's/album's
// folders used to survive because only the file itself was ever removed).
// A directory that isn't actually empty (a sibling album/track still using
// it) is left alone — not an error, the common case for DeleteAlbum.
func removeFilesAndEmptyDirs(paths []string) {
	albumDirs := map[string]bool{}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("library: deleting file %q: %v", p, err)
			continue
		}
		albumDirs[filepath.Dir(p)] = true
	}

	artistDirs := map[string]bool{}
	for dir := range albumDirs {
		if removeIfEmpty(dir) {
			artistDirs[filepath.Dir(dir)] = true
		}
	}
	for dir := range artistDirs {
		removeIfEmpty(dir)
	}
}

// removeIfEmpty removes dir if (and only if) it currently has no entries,
// reporting whether it did. A non-empty or already-gone directory is not
// logged as an error — both are expected outcomes here, not failures.
func removeIfEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("library: checking directory %q: %v", dir, err)
		}
		return false
	}
	if len(entries) > 0 {
		return false
	}
	if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("library: removing empty directory %q: %v", dir, err)
		return false
	}
	return true
}
