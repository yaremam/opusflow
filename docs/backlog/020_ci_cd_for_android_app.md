# User Story: CI/CD for Android App

## 1. User Value Statement

As a **mobile application developer and self-hosted user**,  
I want an **automated GitHub Actions CI/CD pipeline for the OpusFlow Android app**,  
So that **every Pull Request is validated with automated tests and every release tag automatically builds, signs, and publishes installable APK and AAB binaries to GitHub Releases**.

---

## 2. Strict Acceptance Criteria

- **AC-1 (EAS Build Configuration)**: The `mobile/` workspace shall include an `eas.json` configuration file defining build profiles for `preview` (standalone APK) and `production` (Google Play AAB bundle).
- **AC-2 (Pull Request Validation Workflow)**: The GitHub Actions workflow shall run unit tests (`pnpm --filter mobile test`) and TypeScript verification whenever a Pull Request touches files in `mobile/**` or `.github/workflows/android-ci.yml`.
- **AC-3 (Release Tag Build & Packaging)**: Pushing a version tag matching `v*.*.*` shall trigger automated compilation of both Android `.apk` and `.aab` artifacts.
- **AC-4 (GitHub Release Publishing)**: The workflow shall automatically create a GitHub Release for the pushed tag and attach the built `.apk` and `.aab` binaries as downloadable release assets.
- **AC-5 (Security & Credentials)**: Android signing key secrets and EAS access tokens shall be read securely from GitHub Repository Secrets without hardcoding credentials in repository source files.
