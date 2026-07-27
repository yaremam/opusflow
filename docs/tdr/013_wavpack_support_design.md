# TDR 013: WavPack (.wv) Support

## 1. Context & Architectural Requirements

GitHub issue #18 asked for `.wv` (WavPack) support. Today it's entirely
unrecognized: `scan.DetectFormat` (`backend/internal/library/scan/format.go:26-33`)
maps extensions straight to a `duration.DurationParser`, and
`organize.BuildPlan`'s directory walk (`organize/plan.go:81-83`) silently
skips anything `DetectFormat` doesn't recognize — before tags are ever
read. A `.wv` file dropped into an import source today simply never
appears in the review plan.

Grilling this surfaced that a genuinely minimal version (detection +
duration only, tags always blank) was on the table, but the user wants
full parity with MP3/FLAC — detection, exact duration, tag read *and*
write, Genre + embedded art, and hybrid-mode `.wvc` correction files. Two
facts from research shape the design:

- `github.com/dhowden/tag` (the library `readTrackTags`/`readGenreAndArtwork`
  already use for every other format) has no APEv2/WavPack support at all —
  its `Format` enum only covers ID3v1/v2, MP4, and Vorbis
  (`tag.go:73-79`), and dispatches on magic bytes that don't include
  WavPack's `wvpk`. A `.wv` file thus needs its own tag reader *and*
  writer, independent of dhowden/tag.
- No mature Go APEv2 library exists to lean on (confirmed via research) —
  this is genuinely new ground, the same situation TDR 005 already
  accepted for M4A/OGG's write side, just with no library at all rather
  than an immature one.

## 2. Alternatives Evaluated

### Alternative: where the APEv2 reader/writer lives

- **Inline in `organize/`** (alongside `writeMP3Tags`/`writeFLACTags`) —
  Pros: no new package, everything WavPack-specific stays where the other
  format-specific tag functions already are. Cons: APEv2 parsing (a tag
  footer + item table, read *and* written, with its own binary layout) is
  a self-contained concern with no dependency on `organize`'s Plan/Track
  types — bundling it into `organize` mixes "how do I read/write an APEv2
  tag" with "how does opusflow build/copy a plan," the same distinction
  that already keeps duration parsing in its own `scan/duration` package
  rather than inline in `scan`.
- **A new `backend/internal/library/apev2` package (chosen)** — a small
  leaf package (`Read`/`Write`, no dependency on `organize` or `scan`),
  matching the existing shape of `scan/duration` (format-specific parsing,
  zero domain knowledge) and `enrich` (a self-contained concern `library`
  composes). `organize/plan.go` and `organize/copy.go` become two more
  callers, the same relationship they already have with `scan.DetectFormat`
  and `dhowden/tag`.

### Alternative: duration parsing robustness — first-block header only vs. full block scan

- **Read only the first block's header (chosen as the primary path)** —
  WavPack's block header carries a whole-file `total_samples` field
  precisely so readers don't have to scan every block, the same
  "read a header, don't decode audio" approach `wav.go`/`flac.go` already
  take. Pros: fast, matches this package's existing style exactly. Cons:
  a small class of files (encoded from a live/streaming source with an
  unknown-length input) leave `total_samples` at WavPack's documented
  "unknown" sentinel.
- **Always scan every block, summing per-block sample counts** — Pros:
  correct even for the unknown-total-samples case. Cons: means reading
  (not decoding, just header-hopping) potentially thousands of small
  blocks for an ordinary file — real cost for the overwhelmingly common
  case, to handle an edge case.
- **Decision: first-block read, falling back to a full block scan only
  when the header reports the sentinel** — gets the fast path for every
  normal file and correctness for the rare one, mirroring how `mp3.go`
  already falls back from "trust the Xing header" to "estimate from
  bitrate" only when the fast path doesn't have what it needs.

### Alternative: hybrid `.wvc` companion — a new "paired file" concept vs. folding it into the existing per-track model

- **A new first-class "paired file" concept in `Plan`** (e.g. a second
  track-like entry) — Pros: most explicit modeling of "two files, one
  logical track." Cons: a `.wvc` has no audio duration, tags, or title of
  its own to review — giving it its own `Track` entry would put a
  meaningless row in the review table and double-count in `totalTracks`.
- **Fold it into the existing `Track` as a flag + derived paths
  (chosen)** — `Track` gains `HasCorrectionFile bool`; the companion's
  source and destination paths are always mechanically derived (swap the
  extension) rather than stored, so there's exactly one new field on the
  wire. Its conflict status folds into the *same* track's existing
  `Conflict`/`Overwrite` (AC-7's conflict-check applies to whichever of
  the two paths already exists; accepting the overwrite resolves both
  together) — one decision for the reviewer, not two, since a `.wvc`
  without its `.wv` (or vice versa) isn't something a person would ever
  want to resolve independently.

## 3. Structural Decision

**`backend/internal/library/scan/format.go`**: add `".wv": duration.WavPack`
to `durationParsersByExt`. `.wvc` is deliberately **not** added — it must
never be independently detected as its own track (AC-9); it's only ever
noticed as a side effect of finding a `.wv`.

**`backend/internal/library/scan/duration/wavpack.go`**: new `WavPack`
parser (§2) — reads the first block header (`wvpk` magic, sample rate
index, total samples), computing duration directly; falls back to a
block-hopping scan (reading each block's header, summing sample counts,
never decoding audio) when total samples is the documented "unknown"
sentinel.

**`backend/internal/library/apev2/`** (new package): `Read(f *os.File)
(Tags, error)` and `Write(path string, t Tags) error`, where `Tags` holds
Artist/Album/Title/Track/Year (the fields the review screen edits) plus
Genre and Artwork (read-only, matching how Genre/embedded art are treated
for every other format today). `Read` returns a zero `Tags` with no error
when no APEv2 tag is present, mirroring `dhowden/tag`'s
`ErrNoTagsFound`-isn't-an-error convention `readTrackTags`/
`readGenreAndArtwork` already depend on. `Write` parses whatever tag is
already there, replaces only the five text fields it's given (preserving
Genre/Artwork/any other existing item untouched, the same
overwrite-named-fields-only approach `setVorbisComment` already uses for
FLAC), and rewrites the tag footer at the end of the file — creating one
fresh if none existed.

**`organize/plan.go`**: `Track` gains `HasCorrectionFile bool
\`json:"hasCorrectionFile"\``. `readTrackTags` branches on extension: `.wv`
goes through `apev2.Read` instead of `dhowden/tag`; every other format is
unchanged. `BuildPlan`'s walk checks for a sibling `<name>.wvc` next to any
`.wv` it finds (case-insensitive) and sets `HasCorrectionFile` accordingly.

**`organize/validate.go`**: `Validate`'s conflict check becomes "does
`DestPath` exist, or (`HasCorrectionFile` and does its derived `.wvc`
destination exist)" — one derived-path helper, no new `ValidationError`
shape.

**`organize/copy.go`**: `readGenreAndArtwork` branches the same way as
`readTrackTags` (`.wv` → `apev2.Read`). `copyTrack`'s tag-write switch
gains a `.wv` case calling `apev2.Write`. After the main file copies
successfully, if `HasCorrectionFile`, `copyBytes` runs again for the
derived `.wvc` source/destination pair — a failure here fails the whole
track (recorded via the same `RecordImportError` path), not a silent
partial success.

**Frontend**: `PlanTrack` (`web/src/api/library.ts`) gains
`hasCorrectionFile: boolean`. `ImportPage.tsx`'s track row renders a small
icon (title/tooltip: "Includes a .wvc correction file") next to the
existing status dot when set — no other layout change.

**Test fixtures**: no WavPack encoder is available in this environment
(confirmed — no `apt`/`pip` access, `libwavpack1` is installed but not the
CLI encoder). Hand-constructed synthetic block/tag bytes in Go test files
are the plan, the same approach this package's own `wav_test.go` already
takes for WAV rather than shipping a real recorded fixture.

## 4. Cross-Workspace Implications

- **`backend/`**: new `scan/duration/wavpack.go` +
  `scan/duration/wavpack_test.go`; new `apev2/` package (`apev2.go`,
  `apev2_test.go`); `scan/format.go` (new map entry);
  `organize/plan.go`/`validate.go`/`copy.go` (branches + the new
  `HasCorrectionFile` field + derived-path helper); new
  `organize/testdata` fixtures (`tagged.wv`/`untagged.wv`, and a
  `tagged.wv` + `tagged.wvc` pair for the companion-file tests).
- **`web/`**: `web/src/api/library.ts` (`hasCorrectionFile` field),
  `web/src/pages/ImportPage.tsx`/`.css` (the small companion-file icon).
- **`mobile/`**: out of scope, unchanged.
- **Schema**: none — `.wv` tracks flow through the exact same
  `CopiedTrack`/`InsertTrack` shape every other format already uses.
