# User Story: In-Browser Audio Player

## 1. User Value Statement

As a **household member browsing their music library in the web app**, I
want to **play a song directly in the browser, with playback continuing
as I browse to other pages, an automatically-advancing queue, and full
transport controls**, So that **I can actually listen to my library from
opusflow instead of it being a catalog I have to open files from
elsewhere**.

## 2. Strict Acceptance Criteria

- **AC-1**: A play button is available on every playable row in the
  Songs list page and in an album's track table (AlbumDetailPage).
- **AC-2**: Clicking a track's play button starts playback and shows a
  persistent mini-player bar (album art, title, artist, transport
  controls) that stays visible and keeps playing while navigating to any
  other page in the app.
- **AC-3**: Clicking a track queues it plus every track after it in the
  list it was clicked from, in that list's current order. When the
  playing track finishes, the next track in the queue starts
  automatically.
- **AC-4**: The mini-player provides play/pause, a seekable progress bar
  (current time / total duration), skip to next/previous track in the
  queue, and a volume control.
- **AC-5**: A "Queue" control opens a panel listing every upcoming track
  in the queue. Tracks can be dragged to reorder the upcoming queue,
  removed individually, or clicked to jump straight to that track.
- **AC-6**: A track whose file is WavPack (`.wv`) has its play button
  disabled with a short explanation, since no browser can decode that
  format — clicking it does nothing rather than attempting playback and
  failing.
- **AC-7**: Playback is served from a new backend endpoint that supports
  HTTP range requests, so seeking to an unbuffered position starts
  playing from there immediately rather than re-downloading the file
  from the start.
- **AC-8**: Playback state (current track, queue, position) is
  in-memory only — reloading the page or closing the tab stops playback
  and clears the queue; nothing is persisted or resumed automatically.

## 3. Explicitly Out of Scope This Pass

- Shuffle and repeat modes.
- Mobile app playback (this is `web/` only).
- Playing a WavPack track at all (transcoding is a separate, much larger
  effort — not assumed as part of this feature).
- Persisting/resuming playback state across a reload or tab close.
