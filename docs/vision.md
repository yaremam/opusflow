# opusflow Vision

A private, self-hosted music platform — an alternative to Roon that avoids
proprietary protocols wherever that promise is actually achievable. It
unifies a local music library with connected streaming accounts into one
library, plays that music anywhere in the house, and helps you discover and
keep up with the artists you care about.

This document captures product intent — what opusflow is for and its
boundaries. It doesn't lock in build order: each capability below becomes one
or more features, each grilled and scoped on its own via the `/new-feature`
skill before implementation.

## Household model

Single household, multiple profiles — one Netflix-style install per
household, not a multi-tenant service. One shared local library and one set
of linked streaming accounts (Spotify, Apple Music) per household; each
profile has its own listening history, follows, recommendations, and
currently-playing state layered on top.

Multi-tenancy (independent households/accounts on a shared instance) is kept
in mind architecturally — config should stay generic rather than
household-specific — but isn't built now.

## Library

The backend is a media-server core with direct filesystem access, similar to
Plex/Roon/Sonarr: it runs in a Docker container, and local folders (on the
host, or a mounted NAS share) are added as volume mounts and configured as
library paths inside the app. Local files and streaming-service catalogs
appear in one unified library and metadata model regardless of source.

## Streaming integrations

- **Spotify** — first integration, via the official Web Playback SDK.
  Playback happens under the user's own Premium subscription; opusflow never
  touches or re-serves the raw audio.
- **Apple Music** — second, via official MusicKit, same constraint.
- **YouTube Music** — explicitly out: no official API exists: every
  "YouTube Music API" is a reverse-engineered, ToS-violating client that can
  break without warning. Not a dependency anything else relies on.

## Playback & networked output

Playing on other devices in the house is core to the vision, not a
deferred/later feature — but the "no proprietary protocol" promise only
holds for audio opusflow actually controls:

- **Local files**: opusflow decodes the audio itself, so it can stream it to
  any networked output device over a protocol it controls — e.g. adopting
  [Snapcast](https://github.com/badaix/snapcast) rather than inventing a new
  wire protocol. Genuinely proprietary-protocol-free, end to end.
- **Streaming-service tracks**: their SDKs play audio directly on the device
  running the SDK; opusflow has no legitimate way to intercept and
  re-broadcast that stream. Getting a Spotify/Apple Music track onto another
  room's speaker means handing playback off through *their* mechanism —
  Spotify Connect (control an already Connect-enabled speaker) or AirPlay —
  which are the proprietary protocols this project is otherwise avoiding.

## Discovery & recommendations

Starts as an **aggregator**: surfacing similarity/recommendation data other
services already compute (Spotify's own recommendations, Last.fm
scrobble-based similar artists, MusicBrainz relationships), unified across
sources rather than siloed per streaming service.

Long-term vision: opusflow builds its **own** recommendation engine from
real cross-source listening data (local + Spotify + Apple Music combined) —
something no single streaming service can see. This is a deliberate later
evolution, not part of the initial aggregator build.

## Artist following

Two separate capabilities, each with its own data source:

- **New releases** — sourced from the streaming/metadata APIs already in
  play (Spotify, Apple Music, MusicBrainz).
- **Events (tours/concerts)** — requires a dedicated events API (e.g.
  Bandsintown, Songkick), unrelated to anything else the product touches.

Delivered via an in-app feed and mobile push notifications — timely enough
that an event isn't missed by only checking when the library happens to be
open.

## Scope

**In scope now**: music only — local files, Spotify, Apple Music.

**Explicitly future, not core**: podcasts/audiobooks (both Spotify's and
Apple's APIs surface these alongside music, but the UX — episode feeds,
resume position, subscriptions — is a different product surface opusflow
doesn't take on by accident just because the API access is already there).

## Distribution intent

Designed for eventual sharing/self-hosting by others — config via env
vars/settings, no assumptions baked in that only make sense for one specific
household's setup — without doing any actual multi-tenant, packaging, or
licensing work yet. Same posture as the household-model note above: kept in
mind, not built.
