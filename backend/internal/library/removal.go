package library

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
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

	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("library: deleting file %q: %v", p, err)
		}
	}
	return nil
}
