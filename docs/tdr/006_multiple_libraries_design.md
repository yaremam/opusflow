# TDR 006: Multiple Libraries

## 1. Context & Architectural Requirements

[TDR 005](005_organize_on_import_design.md) introduced organize-on-import
against a single, deploy-time `LIBRARY_ROOT` environment variable — one
destination folder, fixed until the container is redeployed with a different
value. That's inflexible for a household that wants more than one destination
(e.g. a main collection and a kids' collection kept physically separate on
disk) and couples a purely operational choice (where files live) to a
redeploy rather than something changeable from within the app.

This feature (grilled and confirmed) turns "the library root" into a real,
in-app concept: a **library** is a name plus a root folder, created by the
user, not declared in an environment variable. Multiple libraries can exist
side by side. Grilling also surfaced that `IMPORT_SOURCE_ROOTS` — the other
environment variable governing where "browse a server folder" may look — sits
on the same spectrum and should go with it: both env vars are removed, and
both the create-a-library and browse-an-import-source folder pickers browse
the container's filesystem unrestricted, starting from `/`. The one safety
rule that survives (explicitly requested): an import's source can't be the
same as, or nested inside, an existing library's root — importing a library
into itself is rejected.

Catalog browsing (Artists/Albums/Songs) is explicitly **not** scoped by
library — grilled and confirmed as unified across all of them. A library only
matters as an import destination.

The mockup (grilled and signed off before this doc — four screens:
choose-a-library, create-a-library, Libraries settings page, delete-library
confirmation) fixed the product shape: a library picker as the new first step
of every import, a name-plus-folder-browser creation form, a dedicated
settings page for viewing/deleting libraries, and the same keep-or-delete-
files prompt TDR 005 established for artist/album removal, reused verbatim
for library removal.

## 2. Alternatives Evaluated

### Alternative: relationship to `LIBRARY_ROOT`

- **Keep as an optional pre-seeded default** — Pros: scripted/non-interactive
  deploys could skip the first-run prompt. Cons: two places the destination
  could live (env var and database) that could disagree; more to explain for
  no real benefit once creating a library in-app is this cheap.
- **Replace entirely (chosen)** — Pros: one source of truth (the `libraries`
  table); nothing writes anywhere until a library actually exists. Cons: a
  fresh deploy always needs at least one in-app step before its first import
  — accepted, since that step (screen 2 of the mockup) is small.

### Alternative: catalog scoping — unified vs. per-library

- **Per-library catalogs** — Pros: matches a mental model where "Kids Music"
  and "Main Collection" are fully separate collections, never mixed in
  Artists/Albums/Songs. Cons: every catalog table, query, page, and test
  needs a library-scoping dimension and a "current library" concept/switcher
  — a large change touching nearly everything TDR 002/003 built.
- **Unified catalog, library only as import destination (chosen)** — Pros:
  a library is genuinely just "a named root folder to copy into" — every
  existing browsing page, query, and test is untouched. Cons: a track's
  originating library isn't visible while browsing (not asked for; can be
  layered on later without a schema change if it ever is, since `imports`
  already carries `library_id`).

### Alternative: filesystem access — keep allowlists vs. remove them

- **Keep `IMPORT_SOURCE_ROOTS` (and add an equivalent for library roots)** —
  Pros: an admin-configured allowlist bounds what's browsable/writable from
  inside the app, independent of what's mounted into the container. Cons: a
  second layer of restriction on top of what Docker already restricts
  (the container can only ever see what's mounted in) — for a
  single-operator, self-hosted app, redundant with the actual security
  boundary (the compose file's own volume mounts).
- **Remove both, browse unrestricted from `/` (chosen)** — Pros: one fewer
  concept to configure and explain; the real boundary (what's mounted into
  the container) is still fully in the deploying admin's control via
  `docker-compose.yml`. Cons: anyone with access to the app's UI can browse
  anywhere the container can see — acceptable for this app's single-operator,
  self-hosted audience, not a multi-tenant one. The one concrete risk grilled
  out explicitly — importing a library into itself — is blocked directly
  (AC-8), rather than solved by restricting browsing in general.

### Alternative: deleting a library — cascade vs. block-until-empty

- **Block deletion until the library has no tracks left** — Pros: simplest
  to implement; no new cascade/orphan-cleanup logic. Cons: with catalog
  browsing unified (no per-library filter), there'd be no way for a user to
  even find which artists/albums to remove first — a dead end in practice
  given the previous decision.
- **Cascade delete, with the keep-or-delete-files choice (chosen)** — Pros:
  one action, same explicit-every-time pattern AC-13 of TDR 005 already
  established for artists/albums — no new UX vocabulary. Cons: requires
  `imports.library_id` (so a library's tracks can be found at all) and a
  library-deletion-scoped orphaned-artist/album sweep — the general
  sweep-on-every-removal behavior TDR 005 deliberately removed in favor of
  direct deletion is reintroduced, but only as an internal step of this one,
  explicit, user-confirmed action — never an automatic side effect of
  anything else.

## 3. Structural Decision

**Env vars removed**: `LIBRARY_ROOT` and `IMPORT_SOURCE_ROOTS` both go away.
`backend/internal/library/roots.go`'s allowlist machinery (`Roots`,
`ParseRoots`, `ValidateDirectory`, `ErrOutsideRoots`) is deleted; `Browse`
becomes a plain, unscoped directory listing (any absolute path in, its
immediate subdirectories out — no containment check).

**Schema** (new migration): adds `libraries` (`id`, `name`, `root_path`,
`created_at`). `imports` gains `library_id` (FK → `libraries`,
`ON DELETE CASCADE` — deleting a library cascades to its imports, which
already cascades to their tracks and import_errors via the existing
`tracks.import_id`/`import_errors.import_id` FKs). No backfill — existing
`imports` rows aren't touched because there aren't any in a supported
deployment (no migration path from the single-`LIBRARY_ROOT` model, per
AC-3).

**`library.Store`/`Service`** gain `CreateLibrary(name, rootPath)` (validates
`rootPath` names an existing directory — the only validation left, now that
there's no allowlist to check against), `ListLibraries`, `DeleteLibrary(id,
deleteFiles)` (collects affected track paths first if `deleteFiles`, deletes
the library row, then sweeps any artist/album left with zero tracks — the
same shape TDR 001's original orphan-cleanup had, reinstated here scoped to
this one action). `BuildPlan`/`BuildPlanFromStaged`/`ValidatePlan`/
`ConfirmImport` all take a `libraryID` now, resolve its root path from the
store, and — for `BuildPlan`/`BuildPlanFromStaged` specifically — reject if
the given source directory is the same as, or nested inside, *any* existing
library's root (AC-8), before handing off to `organize`. The `organize`
package itself (`BuildPlan`/`Validate`/`Copy`) is unchanged — it already only
ever took a plain root-path string, not an env var or allowlist.

**httpserver**: new `GET /api/libraries`, `POST /api/libraries`
(`{name, rootPath}`), `DELETE /api/libraries/{id}?deleteFiles=`. The old
`GET /api/imports/roots` is removed (nothing to list — browsing has no
configured set of roots anymore). `POST /api/imports/plan`,
`POST /api/imports/plan/validate`, `POST /api/imports/upload`, and
`POST /api/imports` all gain a required `libraryId`.

**Frontend**: the Import flow gains a new first step — a library
picker/creator — before today's choose-a-source step; `SourceFolderPicker`
drops its root-tabs (there's no roots list to tab between anymore) and always
starts browsing at `/`; a new `LibrariesPage` (list + delete) is reachable
from the header nav, reusing `RemoveModal` verbatim for the delete
confirmation (it already only needs a name and two callbacks).

**`docker-compose.yml`**: since there's no more env-var-declared, per-purpose
bind-mount (one for `LIBRARY_ROOT`, one or more for `IMPORT_SOURCE_ROOTS`),
the compose file instead mounts one broad, read-write host folder (e.g.
`./data:/data`) — both library creation and import-source browsing happen
somewhere under it. The admin controls what's actually reachable by what
they put on the host side of that single mount, same as they always
controlled it by choosing which folders to bind-mount at all.

## 4. Cross-Workspace Implications

- **`backend/`**:
  - New migration: adds `libraries`; `imports.library_id` (FK, cascade).
  - `backend/internal/library/roots.go`: allowlist machinery deleted;
    `Browse` becomes an unscoped directory listing.
  - `backend/internal/library/store.go` / `service.go`: new
    `CreateLibrary`/`ListLibraries`/`DeleteLibrary`; `BuildPlan` family and
    `ConfirmImport` take `libraryID` and enforce the source/library overlap
    rule (AC-8).
  - `backend/internal/httpserver`: new `/api/libraries` endpoints;
    `/api/imports/roots` removed; the plan/validate/upload/confirm endpoints
    all require `libraryId`.
  - `backend/cmd/server/main.go`: `LIBRARY_ROOT`/`IMPORT_SOURCE_ROOTS` env
    vars no longer read.
  - `backend/internal/library/organize`: unchanged.
- **`web/`**: new library-picker step in the Import flow; `SourceFolderPicker`
  simplified (no root tabs, always starts at `/`); new `LibrariesPage.tsx` +
  nav link; `api/library.ts` gains `listLibraries`/`createLibrary`/
  `deleteLibrary` and threads `libraryId` through the existing plan/upload/
  confirm calls.
- **`mobile/`**: unchanged — out of scope, same as TDR 005.
- **`deploy/`**: `docker-compose.yml`'s `LIBRARY_ROOT`/`IMPORT_SOURCE_ROOTS`
  env vars and their per-purpose bind-mounts are replaced by one broad
  `./data:/data` mount; `docs/deploy/synology.md` updated to match.
- **`docs/`**: this backlog/TDR pair; `docs/ARCHITECTURE.md` updated once
  implementation lands (component list, schema, decision log, and the
  environment-variable description in §3 which TDR 005 itself just added).
