# User Story: Mobile Playback UI — Queue View & Quality Indicator

## 1. User Value Statement

As a **household member listening on the mobile app**,
I want **to see and manage what's coming up next, and know what quality I'm actually listening to**,
So that **the top-right queue button does something, and "Streaming" — which tells me nothing — is replaced with real format/quality information**.

## 2. Strict Acceptance Criteria

- AC-1: The Player screen's top-right button opens a Queue view showing the full current queue, the now-playing track highlighted.
- AC-2: Tapping a track in the queue view jumps playback to it.
- AC-3: Each queued track (other than the now-playing one) can be removed from the queue view.
- AC-4: Tracks in the queue view can be reordered via drag-and-drop.
- AC-5: The Player screen's "Streaming" badge is replaced with a format + audio-quality indicator (e.g. "FLAC · 940 kbps"), computed as average bitrate (file size in bits ÷ duration) — one formula across every supported format. Whether a track is streaming vs. playing from local storage is still visible (via the existing download-icon convention), just no longer through this badge.
