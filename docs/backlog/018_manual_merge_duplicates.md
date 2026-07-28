# User Story: Manual Merge Tool for Duplicate Artists/Albums

## 1. User Value Statement

As a **household member browsing my library**, I want to **merge two
catalog rows I recognize as the same real artist or album into one,
myself, right from the entity's page**, So that **I can clean up a
duplicate immediately — including ones MusicBrainz can't resolve or
hasn't resolved yet — without waiting on automatic dedup (issue #30) or
losing anything already attached to either row**.

## 2. Strict Acceptance Criteria

- **AC-1**: Every artist and album detail page has a "Merge into…" action
  alongside the existing "Remove…" action.
- **AC-2**: Choosing "Merge into…" opens a two-step flow: first, search
  and pick which *other* row (of the same kind, artist or album) to keep;
  second, an explicit confirmation naming exactly what will move (albums/
  tracks/photos for an artist; tracks/covers for an album), that
  same-titled albums combine rather than duplicate, that files on disk
  never move, and that the action can't be undone.
- **AC-3**: The search step only offers other rows of the same kind — for
  an album, only albums by the *same* artist (a cross-artist album merge
  isn't offered; that's an artist merge instead). The row being merged
  never appears in its own search results.
- **AC-4**: Confirming reassigns everything from the source row onto the
  chosen target and removes the source, then the user lands on the
  target's page — the source's own page no longer exists.
- **AC-5**: This reuses the same `MergeArtists`/`MergeAlbums` operation
  issue #30's automatic MusicBrainz-ID dedup uses — a manual merge and an
  automatic one produce identical results, just triggered differently.
