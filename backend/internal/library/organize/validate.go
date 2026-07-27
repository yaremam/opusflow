package organize

import "path/filepath"

// ValidationError reports one track that isn't ready to copy: it's missing
// one or more required fields, has an unresolved destination conflict, or
// both. AlbumIndex/TrackIndex index into the Plan passed to Validate, so
// the caller can map an error straight back to the row it came from.
type ValidationError struct {
	AlbumIndex int      `json:"albumIndex"`
	TrackIndex int      `json:"trackIndex"`
	Missing    []string `json:"missing,omitempty"`
	Conflict   bool     `json:"conflict,omitempty"`
}

// Validate recomputes every track's destination path and on-disk conflict
// status from plan's current field values — mutating plan in place so its
// DestPath/Conflict always reflect what confirming right now would
// actually do, never a stale snapshot from when the plan was first built
// (TDR 005 §3: the server, not the client, is the source of truth for
// both). It returns one ValidationError per track that isn't ready:
// missing a required field (artist, album, year, track number, or title —
// AC-7's "nothing is guessed"), or an unresolved conflict (Conflict true
// and the reviewer hasn't set Overwrite).
func Validate(libraryRoot string, plan *Plan) []ValidationError {
	var errs []ValidationError

	for ai := range plan.Albums {
		al := &plan.Albums[ai]
		for ti := range al.Tracks {
			tr := &al.Tracks[ti]

			missing := missingFields(*al, *tr)

			tr.DestPath = destPath(libraryRoot, al.Artist, al.Album, al.Year, tr.TrackNumber, tr.Title, filepath.Ext(tr.SourcePath))
			tr.Conflict = destExists(tr.DestPath) || (tr.HasCorrectionFile && destExists(correctionPath(tr.DestPath)))

			unresolvedConflict := tr.Conflict && !tr.Overwrite
			if len(missing) > 0 || unresolvedConflict {
				errs = append(errs, ValidationError{
					AlbumIndex: ai,
					TrackIndex: ti,
					Missing:    missing,
					Conflict:   unresolvedConflict,
				})
			}
		}
	}
	return errs
}

func missingFields(al Album, tr Track) []string {
	var missing []string
	if al.Artist == "" {
		missing = append(missing, "artist")
	}
	if al.Album == "" {
		missing = append(missing, "album")
	}
	if al.Year == 0 {
		missing = append(missing, "year")
	}
	if tr.TrackNumber == 0 {
		missing = append(missing, "trackNumber")
	}
	if tr.Title == "" {
		missing = append(missing, "title")
	}
	return missing
}
