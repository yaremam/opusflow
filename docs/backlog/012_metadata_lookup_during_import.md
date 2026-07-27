# User Story: Metadata Lookup During Import

## 1. User Value Statement

As a **household member importing tracks with missing or wrong tags**, I
want to **look up the real artist, album, and track listing from
MusicBrainz and apply it to the files I'm reviewing**, So that **I don't
have to hand-type every title and track number when I already know
exactly which release these files are from**.

## 2. Strict Acceptance Criteria

- **AC-1**: Each album group in the review step (`step === 'review'`) has a
  "Look up metadata" button that opens a guided lookup flow scoped to that
  one album.
- **AC-2**: The flow's first step is an artist search: a text field
  (pre-filled with the album's current artist value) and an explicit
  Search action (not live-as-you-type) that lists matching MusicBrainz
  artists for the user to pick one from.
- **AC-3**: After picking an artist, the flow shows that artist's albums
  (MusicBrainz release-groups) for the user to pick one from.
- **AC-4**: After picking an album, the flow shows that release-group's
  individual releases/editions (each labeled with enough to distinguish
  them — e.g. country, date, track count) for the user to pick the one
  matching what they actually have, since track listings live on a
  specific release, not the abstract release-group.
- **AC-5**: After picking a release, the flow shows its track listing and
  how it will map onto the album's current files — matched by each file's
  current position in the review list against the release's tracks in
  track-number order. Files or tracks left over on either side (count
  mismatch) are shown as unmatched.
- **AC-6**: Nothing in the review screen changes until a single "Apply"
  action at the end of the flow. Applying sets the album's Artist/Album/
  Year and every matched track's Title/Track Number in one commit,
  overwriting any values already there (including manually-typed ones).
  Closing the flow without applying leaves the review screen untouched.
- **AC-7**: Applying triggers the same revalidation every other plan edit
  already does, so conflict/missing-field state stays accurate against the
  newly-applied values.
- **AC-8**: Every MusicBrainz call this feature makes goes through the
  existing shared rate limiter (1 request/second) and User-Agent — it must
  not be able to exceed that rate even under fast repeated use (e.g.
  quickly retrying a search).
