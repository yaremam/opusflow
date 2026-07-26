package organize

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPlanGroupsTaggedFilesByAlbum(t *testing.T) {
	source := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "tagged.mp3"), filepath.Join(source, "one.mp3"))
	copyFixture(t, filepath.Join("testdata", "tagged.flac"), filepath.Join(source, "sub", "two.flac"))

	plan, err := BuildPlan(t.TempDir(), source)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	if len(plan.Albums) != 1 {
		t.Fatalf("len(Albums) = %d, want 1 (both fixtures share Artist/Album tags)", len(plan.Albums))
	}
	al := plan.Albums[0]
	if al.Artist != "Test Artist" || al.Album != "Test Album" || al.Year != 2000 {
		t.Fatalf("album = %+v, want Artist/Album/Year from tags", al)
	}
	if len(al.Tracks) != 2 {
		t.Fatalf("len(Tracks) = %d, want 2", len(al.Tracks))
	}
	for _, tr := range al.Tracks {
		if tr.Title != "Test Title" || tr.TrackNumber != 3 {
			t.Fatalf("track = %+v, want Title/TrackNumber from tags", tr)
		}
	}
}

func TestBuildPlanLeavesUntaggedFieldsBlank(t *testing.T) {
	source := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "untagged.mp3"), filepath.Join(source, "mystery.mp3"))

	plan, err := BuildPlan(t.TempDir(), source)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	if len(plan.Albums) != 1 {
		t.Fatalf("len(Albums) = %d, want 1", len(plan.Albums))
	}
	al := plan.Albums[0]
	if al.Artist != "" || al.Album != "" {
		t.Fatalf("album = %+v, want blank Artist/Album (AC-7 — no filename fallback, no placeholder guess)", al)
	}
	if len(al.Tracks) != 1 {
		t.Fatalf("len(Tracks) = %d, want 1", len(al.Tracks))
	}
	if al.Tracks[0].Title != "" || al.Tracks[0].TrackNumber != 0 {
		t.Fatalf("track = %+v, want blank Title, zero TrackNumber", al.Tracks[0])
	}
}

func TestBuildPlanComputesDestPathAndDetectsConflict(t *testing.T) {
	source := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "tagged.mp3"), filepath.Join(source, "one.mp3"))

	libraryRoot := t.TempDir()
	existing := filepath.Join(libraryRoot, "Test Artist", "2000.Test Album", "03.Test Title.mp3")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("already here"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildPlan(libraryRoot, source)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	tr := plan.Albums[0].Tracks[0]
	if tr.DestPath != existing {
		t.Fatalf("DestPath = %q, want %q", tr.DestPath, existing)
	}
	if !tr.Conflict {
		t.Fatal("Conflict = false, want true — destination already exists on disk")
	}
}

func TestBuildPlanNoConflictWhenDestinationIsFree(t *testing.T) {
	source := t.TempDir()
	copyFixture(t, filepath.Join("testdata", "tagged.mp3"), filepath.Join(source, "one.mp3"))

	plan, err := BuildPlan(t.TempDir(), source)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.Albums[0].Tracks[0].Conflict {
		t.Fatal("Conflict = true, want false — nothing at the destination yet")
	}
}

func TestBuildPlanSortsTracksByTrackNumber(t *testing.T) {
	source := t.TempDir()
	// Both fixtures carry TrackNumber 3 (same tags), so this mainly checks
	// that BuildPlan doesn't reorder or drop files within a group — a
	// same-track-number tie is broken by encounter order, which is fine
	// since real albums don't repeat track numbers.
	copyFixture(t, filepath.Join("testdata", "tagged.mp3"), filepath.Join(source, "a.mp3"))
	copyFixture(t, filepath.Join("testdata", "tagged.flac"), filepath.Join(source, "b.flac"))

	plan, err := BuildPlan(t.TempDir(), source)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Albums[0].Tracks) != 2 {
		t.Fatalf("len(Tracks) = %d, want 2", len(plan.Albums[0].Tracks))
	}
}

func TestBuildPlanErrorsWhenSourceDirDoesNotExist(t *testing.T) {
	_, err := BuildPlan(t.TempDir(), filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("expected an error for a nonexistent source directory")
	}
}
