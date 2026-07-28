package library

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// uploadStagingRoot is the fixed parent directory every "upload from this
// device" staging directory (backend/internal/httpserver's
// handleUploadImport) is created under. Being fixed and server-controlled
// (never client-supplied) lets ConfirmImport's post-copy cleanup recognize
// which of a confirmed plan's SourcePaths came from a staged upload — safe,
// and necessary, to delete — versus a browsed filesystem import, whose
// source directory must never be touched.
var uploadStagingRoot = filepath.Join(os.TempDir(), "opusflow-uploads")

// uploadStagingMaxAge bounds how long an unconfirmed upload's staged files
// stick around. The normal cleanup path is cleanupStagedSources, which runs
// once a confirmed plan's copy has read everything it needs — but a
// reviewer who never confirms (closes the tab, picks a different library)
// leaves nothing to trigger that. NewUploadStagingDir sweeps anything past
// this age out on every new upload instead of running a separate
// background job for what should be a rare case.
const uploadStagingMaxAge = 24 * time.Hour

// NewUploadStagingDir creates a fresh, empty directory under
// uploadStagingRoot for handleUploadImport to stage one upload's files
// into. The caller must not delete it itself on success — once the staged
// plan is handed back to the client for review, the files have to survive
// until ConfirmImport's copy has actually read them (see
// cleanupStagedSources); the caller should only remove it directly if the
// upload fails before a plan is ever returned.
func NewUploadStagingDir() (string, error) {
	if err := os.MkdirAll(uploadStagingRoot, 0o755); err != nil {
		return "", err
	}
	sweepStaleUploadStagingDirs()
	return os.MkdirTemp(uploadStagingRoot, "upload-")
}

// sweepStaleUploadStagingDirs removes any direct child of uploadStagingRoot
// last modified more than uploadStagingMaxAge ago — an upload whose plan
// was never confirmed. Best-effort: errors are ignored since a stale dir
// left behind by a failed sweep is caught on the next upload.
func sweepStaleUploadStagingDirs() {
	entries, err := os.ReadDir(uploadStagingRoot)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-uploadStagingMaxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		os.RemoveAll(filepath.Join(uploadStagingRoot, e.Name()))
	}
}

// cleanupStagedSources removes every staged-upload directory a confirmed
// plan's tracks were read from, now that Copy has finished with them.
// Tracks whose SourcePath isn't under uploadStagingRoot at all (a browsed
// filesystem import, the far more common case) are left completely alone.
func cleanupStagedSources(plan organize.Plan) {
	seen := map[string]bool{}
	for _, al := range plan.Albums {
		for _, tr := range al.Tracks {
			dir := stagingDirFor(tr.SourcePath)
			if dir == "" || seen[dir] {
				continue
			}
			seen[dir] = true
			os.RemoveAll(dir)
		}
	}
}

// stagingDirFor returns the direct child of uploadStagingRoot that
// sourcePath lives under, or "" if sourcePath isn't under uploadStagingRoot
// at all.
func stagingDirFor(sourcePath string) string {
	rel, err := filepath.Rel(uploadStagingRoot, sourcePath)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return ""
	}
	first, _, _ := strings.Cut(rel, string(filepath.Separator))
	return filepath.Join(uploadStagingRoot, first)
}
