# Docs

- [`vision.md`](vision.md) — product vision: what opusflow is, its
  capabilities, and its scope boundaries. Read this first.
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — living system architecture:
  components, interfaces, data model, and a decision-log index. Start here.
- [`backlog/`](backlog/) — one user story per feature: user value statement
  and strict acceptance criteria.
- [`tdr/`](tdr/) — Technical Design Records: one per feature, paired by
  number with its backlog entry. Context, alternatives evaluated, and the
  structural decision made.

Every feature gets a matched `backlog/NNN_<slug>.md` + `tdr/NNN_<slug>_design.md`
pair, created together via the `/new-feature` skill. See CLAUDE.md for the
full feature-development process (grilling kickoff, UI mockup sign-off, TDD).
