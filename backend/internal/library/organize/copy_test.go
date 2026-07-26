package organize

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
	"github.com/dhowden/tag"
)

// fakeStore is an in-memory Store double for asserting on what Copy reports,
// without needing a real database.
type fakeStore struct {
	tracks   []CopiedTrack
	errors   []string
	progress []int
	total    []int
}

func (s *fakeStore) InsertTrack(ctx context.Context, t CopiedTrack) error {
	s.tracks = append(s.tracks, t)
	return nil
}

func (s *fakeStore) RecordImportError(ctx context.Context, importID int64, path, message string) error {
	s.errors = append(s.errors, path+": "+message)
	return nil
}

func (s *fakeStore) UpdateImportProgress(ctx context.Context, importID int64, processed, total int) error {
	s.progress = append(s.progress, processed)
	s.total = append(s.total, total)
	return nil
}

// mp3WithArtwork copies testdata/tagged.mp3 into dir and embeds an APIC
// frame carrying data, so Copy's artwork-extraction path has something real
// to find — the checked-in fixtures deliberately don't carry artwork, since
// no other test in this package needs it.
func mp3WithArtwork(t *testing.T, dir string, data []byte) string {
	t.Helper()
	dest := filepath.Join(dir, "with_art.mp3")
	copyFixture(t, "testdata/tagged.mp3", dest)

	tg, err := id3v2.Open(dest, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("id3v2.Open: %v", err)
	}
	tg.AddAttachedPicture(id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    "image/png",
		PictureType: id3v2.PTFrontCover,
		Picture:     data,
	})
	if err := tg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	tg.Close()
	return dest
}

func onePixelPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d, 0xb0, 0x00, 0x00, 0x00,
		0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func trackFor(sourcePath, destPath, title string, trackNumber int) Track {
	return Track{SourcePath: sourcePath, DestPath: destPath, Title: title, TrackNumber: trackNumber}
}

func TestCopyWritesFileToDestPath(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Corrected Artist", "1999.Corrected Album", "05.Corrected Title.mp3")
	plan := Plan{Albums: []Album{{
		Artist: "Corrected Artist", Album: "Corrected Album", Year: 1999,
		Tracks: []Track{trackFor("testdata/tagged.mp3", dest, "Corrected Title", 5)},
	}}}

	store := &fakeStore{}
	summary := Copy(context.Background(), store, 1, plan)

	if summary.FilesProcessed != 1 || summary.FilesFailed != 0 {
		t.Fatalf("summary = %+v, want 1 processed, 0 failed", summary)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("copied file missing at %s: %v", dest, err)
	}
}

func TestCopyWritesBackCorrectedTagsToMP3(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Corrected Artist", "1999.Corrected Album", "05.Corrected Title.mp3")
	plan := Plan{Albums: []Album{{
		Artist: "Corrected Artist", Album: "Corrected Album", Year: 1999,
		Tracks: []Track{trackFor("testdata/tagged.mp3", dest, "Corrected Title", 5)},
	}}}

	store := &fakeStore{}
	Copy(context.Background(), store, 1, plan)

	f, err := os.Open(dest)
	if err != nil {
		t.Fatalf("open copied file: %v", err)
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		t.Fatalf("read back tags: %v", err)
	}
	if m.Title() != "Corrected Title" || m.Artist() != "Corrected Artist" || m.Album() != "Corrected Album" {
		t.Fatalf("tags = %+v/%+v/%+v, want corrected values", m.Title(), m.Artist(), m.Album())
	}
	if got, _ := m.Track(); got != 5 {
		t.Fatalf("track number = %d, want 5", got)
	}
	if m.Year() != 1999 {
		t.Fatalf("year = %d, want 1999", m.Year())
	}
}

func TestCopyWritesBackCorrectedTagsToFLAC(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Corrected Artist", "1999.Corrected Album", "05.Corrected Title.flac")
	plan := Plan{Albums: []Album{{
		Artist: "Corrected Artist", Album: "Corrected Album", Year: 1999,
		Tracks: []Track{trackFor("testdata/tagged.flac", dest, "Corrected Title", 5)},
	}}}

	store := &fakeStore{}
	Copy(context.Background(), store, 1, plan)

	f, err := os.Open(dest)
	if err != nil {
		t.Fatalf("open copied file: %v", err)
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil {
		t.Fatalf("read back tags: %v", err)
	}
	if m.Title() != "Corrected Title" || m.Artist() != "Corrected Artist" || m.Album() != "Corrected Album" {
		t.Fatalf("tags = %+v/%+v/%+v, want corrected values", m.Title(), m.Artist(), m.Album())
	}
	if got, _ := m.Track(); got != 5 {
		t.Fatalf("track number = %d, want 5", got)
	}
	if m.Year() != 1999 {
		t.Fatalf("year = %d, want 1999", m.Year())
	}
}

func TestCopyCarriesGenreAndArtworkIntoCopiedTrack(t *testing.T) {
	srcDir := t.TempDir()
	art := onePixelPNG()
	src := mp3WithArtwork(t, srcDir, art)

	root := t.TempDir()
	dest := filepath.Join(root, "Artist", "2000.Album", "03.Title.mp3")
	plan := Plan{Albums: []Album{{
		Artist: "Artist", Album: "Album", Year: 2000,
		Tracks: []Track{trackFor(src, dest, "Title", 3)},
	}}}

	store := &fakeStore{}
	Copy(context.Background(), store, 1, plan)

	if len(store.tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(store.tracks))
	}
	got := store.tracks[0]
	if got.Genre != "Jazz" {
		t.Fatalf("Genre = %q, want %q (carried from source tags)", got.Genre, "Jazz")
	}
	if string(got.ArtworkData) != string(art) {
		t.Fatalf("ArtworkData mismatch: got %d bytes, want %d bytes matching source picture", len(got.ArtworkData), len(art))
	}
}

func TestCopyRecordsPerFileFailureAndContinues(t *testing.T) {
	root := t.TempDir()
	goodDest := filepath.Join(root, "Artist", "2000.Album", "02.Good.mp3")
	plan := Plan{Albums: []Album{{
		Artist: "Artist", Album: "Album", Year: 2000,
		Tracks: []Track{
			trackFor("testdata/does-not-exist.mp3", filepath.Join(root, "Artist", "2000.Album", "01.Missing.mp3"), "Missing", 1),
			trackFor("testdata/tagged.mp3", goodDest, "Good", 2),
		},
	}}}

	store := &fakeStore{}
	summary := Copy(context.Background(), store, 7, plan)

	if summary.FilesProcessed != 1 || summary.FilesFailed != 1 {
		t.Fatalf("summary = %+v, want 1 processed, 1 failed", summary)
	}
	if len(store.errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1", len(store.errors))
	}
	if _, err := os.Stat(goodDest); err != nil {
		t.Fatalf("second track's file should still have been copied: %v", err)
	}
	if len(store.tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1 (only the good track inserted)", len(store.tracks))
	}
}

func TestCopyReportsFinalProgressAgainstTotalTrackCount(t *testing.T) {
	root := t.TempDir()
	plan := Plan{Albums: []Album{{
		Artist: "Artist", Album: "Album", Year: 2000,
		Tracks: []Track{
			trackFor("testdata/tagged.mp3", filepath.Join(root, "Artist", "2000.Album", "01.A.mp3"), "A", 1),
			trackFor("testdata/tagged.flac", filepath.Join(root, "Artist", "2000.Album", "02.B.flac"), "B", 2),
		},
	}}}

	store := &fakeStore{}
	Copy(context.Background(), store, 1, plan)

	if len(store.progress) == 0 {
		t.Fatal("expected at least one progress update")
	}
	lastProcessed := store.progress[len(store.progress)-1]
	lastTotal := store.total[len(store.total)-1]
	if lastProcessed != 2 || lastTotal != 2 {
		t.Fatalf("final progress = %d/%d, want 2/2", lastProcessed, lastTotal)
	}
}
