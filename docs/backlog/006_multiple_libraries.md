# User Story: Multiple Libraries

## 1. User Value Statement

As a **household member organizing music with opusflow**, I want **to create
one or more named libraries, each with its own root folder that I pick myself
from within the app, and choose which library an import copies into**, So
that **I'm not locked into a single deploy-time destination folder — I can
keep, say, a main collection and a kids' collection separate on disk, and set
each one up without editing environment variables or redeploying**.

## 2. Strict Acceptance Criteria

### Libraries replace `LIBRARY_ROOT`

- **AC-1**: The `LIBRARY_ROOT` environment variable is removed. A library
  (name + root folder path) is created and managed entirely within the app;
  no library exists until a household member creates one.
- **AC-2**: Artist/Album/Song browsing is unaffected and stays unified across
  every library — a library is a destination an import targets, not a filter
  on what's browsable. No existing catalog page changes shape.
- **AC-3**: No migration is provided for anything organized under the old,
  single-`LIBRARY_ROOT` model — a fresh database is the only supported state
  for this change, consistent with how [005](005_organize_on_import.md)
  itself shipped.

### Choosing/creating a library during import

- **AC-4**: Starting an import first asks which library to copy into: a list
  of existing libraries, or an option to create a new one. This is shown on
  every import, not only the first.
- **AC-5**: Creating a library asks for a name and a root folder, the latter
  chosen via a folder browser rather than typed free-form.
- **AC-6**: Once a library is chosen or created, the import flow continues
  exactly as it does today (choose a source, review plan, copy, done),
  computing destinations against the chosen library's root.

### Filesystem browsing (both directions)

- **AC-7**: The `IMPORT_SOURCE_ROOTS` environment variable is also removed.
  Both the "browse a server folder" import-source picker and the
  create-a-library folder picker browse the container's filesystem starting
  from `/`, with no configured allowlist.
- **AC-8**: An import is rejected if its chosen source path is the same as,
  or nested inside, any existing library's root — prevents importing a
  library into itself.

### Managing libraries

- **AC-9**: A dedicated Libraries page (reachable from the main navigation)
  lists every library — name, root path, and a track count — and is the only
  place a library can be deleted from.
- **AC-10**: Deleting a library always asks, in the moment, whether to also
  delete the files it copied in or only remove it (and everything imported
  into it) from the catalog — the same keep-or-delete-files choice
  [005](005_organize_on_import.md) established for removing an artist/album,
  with no silent default either way.
- **AC-11**: Deleting a library removes every track that was imported into
  it, and any artist/album left with zero tracks as a result.

## 3. Out of Scope

- Renaming a library, or changing an existing library's root path, once
  created.
- Scoping catalog browsing (Artists/Albums/Songs) to a single library — see
  AC-2.
- Any restriction on where a library's root or an import's source may live
  beyond AC-8's single overlap rule — the admin controls what's reachable by
  what they mount into the container (see the design doc's volume-mount
  discussion).
