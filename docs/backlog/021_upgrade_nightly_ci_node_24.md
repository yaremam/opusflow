# User Story: Upgrade Nightly CI to Node.js 24

## 1. User Value Statement

As a **repository maintainer**,  
I want the nightly CI build pipeline (`.github/workflows/nightly.yml`) to use Node.js 24 and standard action versions,  
So that **the CI workflow runs without Node 20 deprecation warnings and maintains 100% environment parity with our multi-stage Dockerfile and mobile CI pipeline**.

---

## 2. Strict Acceptance Criteria

- **AC-1 (Node.js Runtime Standardized)**: `.github/workflows/nightly.yml` shall configure `actions/setup-node@v4` with `node-version: 24` and `cache: pnpm`.
- **AC-2 (Action Version Parity)**: All `actions/checkout` steps in `nightly.yml` shall use official release version `@v4`.
- **AC-3 (Clean Workflow Execution)**: The nightly CI workflow shall execute without Node 20 deprecation warnings or version mismatch errors.
