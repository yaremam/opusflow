package scan

// Track is one audio file discovered and processed during a directory
// scan, ready to be persisted.
type Track struct {
	DirectoryID     int64
	Path            string
	Title           string
	Artist          string
	Album           string
	TrackNumber     int
	Year            int
	Genre           string
	DurationSeconds int

	// ArtworkData is this file's embedded cover art, if any (see Tags.Artwork).
	ArtworkData []byte
}
