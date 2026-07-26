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
- `opusflow-data` — everything opusflow reads from or writes to lives
  somewhere under this one folder: organized libraries (created from
  within the app itself — see step 3) and anything you want to import
  *from*. If you have existing audio files to bring in, copy them onto a
  subfolder here via **File Station** (or however you normally move files
  onto the NAS); skip this if you'll only ever use "upload from device".
  Note the full path DSM shows for it, typically `/volume1/opusflow-data`.

## 2. Create the project

**Container Manager → Project → Create**:

1. **Project name**: `opusflow`.
2. **Path**: select the `opusflow` shared folder from step 1.
3. **Source**: choose **Create docker-compose.yml** — this opens a text
   editor in the wizard.
4. Paste in the full contents of
   [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) (open it on
   any machine, copy it, paste here — no download/upload step needed).
5. Before continuing, edit these lines directly in the pasted text — none of
   the three have a default, so leaving any of them unedited makes
   **Action → Build** fail with a message naming exactly which one is missing:
   - `- ${OPUSFLOW_DATA_DIR:?...}:/data` → replace the whole `${...}` token
     with the real host path from step 1, e.g. `- /volume1/opusflow-data:/data`.
     Everything opusflow reads or writes — organized libraries and anything
     you import from — lives somewhere under this one mount.
   - `- ${OPUSFLOW_PGDATA_DIR:?...}:/var/lib/postgresql/data` → replace with
     where the Postgres database itself lives, e.g.
     `- /volume1/docker/opusflow/pgdata:/var/lib/postgresql/data`.
   - `- ${OPUSFLOW_ARTWORK_DIR:?...}:/artwork` → replace with where fetched
     artist photos and album covers are cached, e.g.
     `- /volume1/docker/opusflow/artwork:/artwork`.
   - `- "8090:8080"` → change the `8090` if that port is already taken on
     your NAS (DSM itself claims `5000`/`5001`, and some packages default to
     `8080`). This is the port mapping — the app answers on this host port.
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
- Visit `http://<your-nas-ip>:<port>` in a browser for the app itself. Nothing
  exists yet — open the **Libraries** page and create your first library
  (give it a name, then browse to a folder under `/data` for it to live in),
  then use the **Import** page to browse a source folder under `/data` (or
  upload from your device) and copy files into it. Check
  **Container Manager → Project → opusflow → Logs** on the `app` container
  if an import seems stuck for a large batch of files.

## Updating to a newer nightly

**Container Manager → Project → opusflow → Action → Build** re-pulls
`ghcr.io/yaremam/opusflow:nightly` and recreates the `app` container against
whatever is currently published — your library and Postgres data (on their
Docker volumes) are untouched. Check `/health`'s `revision` before and after
to confirm the update actually landed.

## Changing settings later

**Container Manager → Project → opusflow → Action → Edit Project** reopens
the same compose text editor from step 2 — edit the port or folder path
lines there, then **Action → Build** to apply the change.

## Troubleshooting

- **Build fails with "required variable ... is missing a value"**: one of the
  three `${OPUSFLOW_..._DIR:?...}` volume lines (see step 5) still has its
  placeholder token instead of a real host path — edit it and **Action →
  Build** again.
- **Port already in use**: edit the `"8090:8080"` line (see above), then
  **Action → Build**.
- **`app` container keeps restarting**: check its logs — the most common
  cause is `postgres` not yet healthy on first boot (the `app` service waits
  for it automatically).
- **Import or Libraries can't see your files**: confirm the `:/data` bind-mount's
  host path actually exists on the NAS and contains your folders — a typo
  there silently mounts an empty directory, and the folder browser will
  just look empty.
