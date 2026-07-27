package organize

import (
	"os"
	"path/filepath"
	"testing"
)

func completeTrack(sourcePath string) Track {
	return Track{SourcePath: sourcePath, TrackNumber: 1, Title: "Shine On"}
}

func TestValidateAcceptsACompletePlan(t *testing.T) {
	plan := Plan{Albums: []Album{{
		Artist: "Pink Floyd", Album: "Wish You Were Here", Year: 1975,
		Tracks: []Track{completeTrack("/src/one.flac")},
	}}}

	errs := Validate(t.TempDir(), &plan)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if plan.Albums[0].Tracks[0].DestPath == "" {
		t.Fatal("DestPath was not populated by Validate")
	}
}

func TestValidateFlagsMissingRequiredFields(t *testing.T) {
	plan := Plan{Albums: []Album{{
		Artist: "", Album: "Wish You Were Here", Year: 1975,
		Tracks: []Track{{SourcePath: "/src/one.flac", TrackNumber: 0, Title: ""}},
	}}}

	errs := Validate(t.TempDir(), &plan)
	if len(errs) != 1 {
		t.Fatalf("len(errs) = %d, want 1", len(errs))
	}
	want := map[string]bool{"artist": true, "trackNumber": true, "title": true}
	got := map[string]bool{}
	for _, m := range errs[0].Missing {
		got[m] = true
	}
	for field := range want {
		if !got[field] {
			t.Fatalf("Missing = %v, want it to include %q", errs[0].Missing, field)
		}
	}
}

func TestValidateFlagsUnresolvedConflict(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Pink Floyd", "1975.Wish You Were Here", "01.Shine On.flac")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	track := completeTrack("/src/one.flac")
	track.Title = "Shine On"
	plan := Plan{Albums: []Album{{Artist: "Pink Floyd", Album: "Wish You Were Here", Year: 1975, Tracks: []Track{track}}}}

	errs := Validate(root, &plan)
	if len(errs) != 1 || !errs[0].Conflict {
		t.Fatalf("errs = %+v, want one conflict error", errs)
	}
	if !plan.Albums[0].Tracks[0].Conflict {
		t.Fatal("Validate did not refresh Track.Conflict to true")
	}
}

func TestValidateFlagsConflictWhenOnlyTheWVCCompanionExists(t *testing.T) {
	root := t.TempDir()
	// The .wv destination itself is free, but its .wvc companion's
	// destination already exists — TDR 013 AC-7 treats that the same as
	// any other conflict, since a .wvc is never overwritten silently.
	wvcDest := filepath.Join(root, "Pink Floyd", "1975.Wish You Were Here", "01.Shine On.wvc")
	if err := os.MkdirAll(filepath.Dir(wvcDest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wvcDest, []byte("existing correction file"), 0o644); err != nil {
		t.Fatal(err)
	}

	track := completeTrack("/src/one.wv")
	track.HasCorrectionFile = true
	plan := Plan{Albums: []Album{{Artist: "Pink Floyd", Album: "Wish You Were Here", Year: 1975, Tracks: []Track{track}}}}

	errs := Validate(root, &plan)
	if len(errs) != 1 || !errs[0].Conflict {
		t.Fatalf("errs = %+v, want one conflict error from the .wvc companion alone", errs)
	}
}

func TestValidateOverwriteResolvesBothWVAndWVCConflict(t *testing.T) {
	root := t.TempDir()
	wvcDest := filepath.Join(root, "Pink Floyd", "1975.Wish You Were Here", "01.Shine On.wvc")
	if err := os.MkdirAll(filepath.Dir(wvcDest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wvcDest, []byte("existing correction file"), 0o644); err != nil {
		t.Fatal(err)
	}

	track := completeTrack("/src/one.wv")
	track.HasCorrectionFile = true
	track.Overwrite = true
	plan := Plan{Albums: []Album{{Artist: "Pink Floyd", Album: "Wish You Were Here", Year: 1975, Tracks: []Track{track}}}}

	errs := Validate(root, &plan)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none — Overwrite resolves the .wvc conflict too, one decision for both files", errs)
	}
}

func TestValidateAllowsExplicitOverwriteOfAConflict(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "Pink Floyd", "1975.Wish You Were Here", "01.Shine On.flac")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	track := completeTrack("/src/one.flac")
	track.Title = "Shine On"
	track.Overwrite = true
	plan := Plan{Albums: []Album{{Artist: "Pink Floyd", Album: "Wish You Were Here", Year: 1975, Tracks: []Track{track}}}}

	errs := Validate(root, &plan)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none — Overwrite was set", errs)
	}
}
