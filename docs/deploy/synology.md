# Self-hosting on Synology (DSM Container Manager)

Runs opusflow from the prebuilt nightly image (`ghcr.io/yaremam/opusflow`) —
no repo clone, no Go/Node toolchain on the NAS. Everything needed lives in
one file, [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) —
no separate `.env` to manage. You edit a handful of lines marked "EDIT THIS"
directly in that file before deploying.

**Prerequisites**: DSM 7.2+, the **Container Manager** package installed
(Package Center → search "Container Manager" → Install).

## 1. Create shared folders

**Control Panel → Shared Folder → Create**:

- `opusflow` — holds the project (Container Manager needs a path to store
  its resolved compose file under, even when you paste the YAML directly).
- `music` — if you don't already have one. Copy your audio files onto it via
  **File Station** (or however you normally move files onto the NAS). Note
  the full path DSM shows for it, typically `/volume1/music`.

## 2. Create the project

**Container Manager → Project → Create**:

1. **Project name**: `opusflow`.
2. **Path**: select the `opusflow` shared folder from step 1.
3. **Source**: choose **Create docker-compose.yml** — this opens a text
   editor in the wizard.
4. Paste in the full contents of
   [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) (open it on
   any machine, copy it, paste here — no download/upload step needed).
5. Before continuing, edit these lines directly in the pasted text:
   - `- ./music-1:/music/1:ro` → the real host path from step 1, e.g.
     `- /volume1/music:/music/1:ro`. This is the volume mapping: whatever you
     put on the left is what opusflow scans.
   - `- "8090:8080"` → change the `8090` if that port is already taken on
     your NAS (DSM itself claims `5000`/`5001`, and some packages default to
     `8080`). This is the port mapping — the app answers on this host port.
   - `POSTGRES_PASSWORD: change-me-please` **and** the matching
     `change-me-please` inside `DATABASE_URL` a few lines up — change both to
     the same new value. They must match exactly or the app can't reach its
     own database.
   - More than one music folder? Uncomment one of the `#   - /volume1/...`
     lines under `volumes:`, point it at the extra folder, and extend
     `LIBRARY_ROOTS: /music/1` to `/music/1,/music/2` (add `/music/3` etc. as
     needed) — the list must match the mounted paths in order.
6. Skip the optional "Web Station portal" step unless you already use one.
7. Confirm. Container Manager pulls the `app` and `postgres` images and
   starts both containers — first pull takes a few minutes depending on your
   NAS's connection.

## 3. Verify it's running

Once the project shows both containers as **Running**:

- Visit `http://<your-nas-ip>:<port>/health` (default port `8090`) — expect
  `{"status":"ok","revision":"<git sha>"}`. The `revision` field tells you
  exactly which nightly build is running; include it if you ever report a
  problem.
- Visit `http://<your-nas-ip>:<port>` in a browser for the app itself. Your
  music library won't appear until the backend finishes its first scan —
  check **Container Manager → Project → opusflow → Logs** on the `app`
  container if it's been more than a few minutes for a large library.

## Updating to a newer nightly

**Container Manager → Project → opusflow → Action → Build** re-pulls
`ghcr.io/yaremam/opusflow:nightly` and recreates the `app` container against
whatever is currently published — your library and Postgres data (on their
Docker volumes) are untouched. Check `/health`'s `revision` before and after
to confirm the update actually landed.

## Changing settings later

**Container Manager → Project → opusflow → Action → Edit Project** reopens
the same compose text editor from step 2 — edit the port, folder path, or
password lines there, then **Action → Build** to apply the change.

## Troubleshooting

- **Port already in use**: edit the `"8090:8080"` line (see above), then
  **Action → Build**.
- **`app` container keeps restarting**: check its logs — the most common
  cause is `postgres` not yet healthy on first boot (the `app` service waits
  for it automatically), or the two `change-me-please` values no longer
  matching after an edit.
- **Library stays empty**: confirm the music bind-mount's host path (left
  side of the `:/music/1:ro` line) actually exists on the NAS and contains
  audio files — a typo there silently mounts nothing.
