# TDR 021: Upgrade Nightly CI to Node.js 24 Architecture

## 1. Context & Architectural Requirements

The repository's nightly image build workflow (`.github/workflows/nightly.yml`) tests `main` against PostgreSQL and publishes prebuilt container images to `ghcr.io`.

With GitHub Actions deprecating Node.js 20 on runner environments, `nightly.yml` must be updated to align with the rest of the repository's toolchain (Node 24 in `Dockerfile` and `android-ci.yml`).

---

## 2. Alternatives Evaluated

### Alternative A: Keep Legacy Node Environment with Exemption Flags
Set `ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION=true` in `nightly.yml`.

- **Pros**: Zero structural YAML changes.
- **Cons**: Leaves deprecated runtime in place; risks sudden pipeline breakage when GitHub Actions fully drops Node 20.

### Alternative B (Chosen): Standardize on Node.js 24 (`node-version: 24`)
Update `actions/setup-node@v4` in `nightly.yml` to specify `node-version: 24` and standardize checkout steps on `actions/checkout@v4`.

- **Pros**:
  - Full runtime parity with root `Dockerfile` (`node:24-alpine`) and mobile CI workflow.
  - Eliminates runner deprecation warnings.
- **Cons**: None.

---

## 3. Structural Decision

We select **Alternative B**.

1. **`actions/checkout`**: Use `@v4` across `changes`, `test`, and `publish` jobs.
2. **`actions/setup-node`**: Use `node-version: 24` with `cache: pnpm`.
3. **`pnpm/action-setup`**: Use `@v4` auto-detecting `packageManager` from root `package.json`.

---

## 4. Cross-Workspace Implications

- **`.github/workflows/nightly.yml`**: Updated runtime and checkout versions.
- No breaking changes to `backend/`, `web/`, or `mobile/` source code.
