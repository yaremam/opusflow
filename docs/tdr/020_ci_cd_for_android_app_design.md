# TDR 020: CI/CD for Android App Architecture

## 1. Context & Architectural Requirements

With the creation of the Android Companion App (`mobile/`), OpusFlow requires an automated continuous integration and release delivery pipeline.

Architectural requirements:
- PR validation: Fast feedback on code changes touching `mobile/`.
- Automated Android build execution: Generate installable APK binaries for self-hosted users and AAB bundles for Play Store submission.
- GitHub Release publishing: Automatically attach compiled Android binaries to tag releases (`v*.*.*`).
- Zero hardcoded secrets: Rely on GitHub Repository Secrets for EAS credentials and signing keystores.

---

## 2. Alternatives Evaluated

### Alternative A: Raw Native Gradle Build on GitHub Actions Runner (`./gradlew assembleRelease`)
Run `npx expo prebuild` to generate native Android folders, setup Java JDK 17, and invoke `./gradlew`.

- **Pros**:
  - Direct control over Android SDK version and Gradle tasks on GitHub runners.
- **Cons**:
  - Requires maintaining complex Gradle build scripts and Android SDK dependencies.
  - High risk of environment drift between local Expo developer environment and CI runner.

### Alternative B (Chosen): Expo Application Services (EAS Build CLI) + GitHub Actions
Configure `mobile/eas.json` and invoke `eas-cli` within GitHub Actions workflows.

- **Pros**:
  - Official, standardized build toolchain designed specifically for Expo React Native apps.
  - Handles native Android build environment, keystore management, and artifact generation seamlessly.
  - Generates both standalone `.apk` for direct installs and `.aab` for Play Store distribution.
- **Cons**:
  - Requires `EAS_TOKEN` repository secret for remote EAS builds.

---

## 3. Structural Decision

We select **Alternative B**.

1. **EAS Configuration (`mobile/eas.json`)**:
   - `preview`: Produces `apk` build type for direct side-loading.
   - `production`: Produces `app-bundle` (`.aab`) build type.
2. **GitHub Actions Workflow (`.github/workflows/android-ci.yml`)**:
   - **`test` Job**: Runs on PRs to `mobile/**`. Executes `pnpm install`, `pnpm --filter mobile test`, and TypeScript checks.
   - **`build-and-release` Job**: Runs on tag push `v*.*.*`. Installs EAS CLI, runs EAS Android build for APK and AAB, and publishes assets to GitHub Releases via `softprops/action-gh-release`.

---

## 4. Cross-Workspace Implications

- **`mobile/`**: Contains `eas.json` build profiles. No impact on Go `backend/` or React `web/` workspaces.
- **`.github/workflows/`**: Adds `android-ci.yml` alongside existing repository CI workflows.
