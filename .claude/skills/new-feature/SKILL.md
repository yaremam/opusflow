---
name: new-feature
description: Kick off a new feature — grill the idea, optionally mock up the UI, then scaffold a paired backlog user story + TDR design doc, following the docs/backlog and docs/tdr numbering and template conventions. Use when starting work on a new feature, screen, or endpoint.
disable-model-invocation: true
---

Kick off work on: $ARGUMENTS

1. **Grill it first.** Invoke the `grilling` skill (`mattpocock-skills:grilling`)
   on the feature idea in $ARGUMENTS before writing anything down. Don't move
   to step 2 until the grilling converges on a clear, specific feature concept
   — a vague idea produces vague, untestable acceptance criteria.

2. **Determine the next number.** Look at existing files in `docs/backlog/`
   and `docs/tdr/`, find the highest `NNN_` prefix across both directories,
   and increment by 1 (zero-padded to 3 digits, e.g. `001`). Slugify the
   feature name (lowercase, spaces/punctuation to underscores) for filenames.

3. **Mock up the UI, if the feature has one.** Any new web or mobile screen
   must get a mockup (an Artifact) before any component/handler code is
   written — minor tweaks to an existing screen (copy edits, spacing/color
   fixes) don't require this. Get explicit sign-off on the mockup before
   continuing to step 4. Skip this step entirely for backend-only features
   with no UI surface.

4. **Create `docs/backlog/NNN_<slug>.md`**:
   - `# User Story: <Title>`
   - `## 1. User Value Statement` — As a **<role>**, I want to **<capability>**,
     So that **<benefit>**.
   - `## 2. Strict Acceptance Criteria` — numbered `AC-N` bullets, specific
     and testable.

5. **Create `docs/tdr/NNN_<slug>_design.md`**:
   - `# TDR NNN: <Title>`
   - `## 1. Context & Architectural Requirements`
   - `## 2. Alternatives Evaluated` — at least two `### Alternative`
     subsections with Pros/Cons
   - `## 3. Structural Decision` — which alternative was chosen and why
   - `## 4. Cross-Workspace Implications` — which of `backend/`/`web/`/`mobile/`
     this touches, any API contract or schema changes, and how it fits the
     stack decisions in CLAUDE.md and `docs/ARCHITECTURE.md`

6. Infer the user story and TDR content from the grilled-out feature concept
   and the project's stack/conventions in CLAUDE.md. Ask the user for
   clarification only if something acceptance-criteria-relevant is still
   unresolved after grilling.

7. Report the file paths created (and the mockup, if one was made). Remind
   the user that implementation follows TDD (red-green-refactor per
   CLAUDE.md) starting from the acceptance criteria in the backlog entry.
