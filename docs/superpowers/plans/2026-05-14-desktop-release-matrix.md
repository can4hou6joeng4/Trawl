# Desktop Release Matrix Hardening Plan

## Goal

Harden the desktop-only release path after removing Web/Docker/API support. The first priority is a safe public release baseline for macOS and Windows desktop users, without embedding cloud credentials into distributed binaries.

## Priorities

- P0: Publish a macOS universal DMG for Intel and Apple Silicon users.
- P0: Publish a Windows x64 installer and ZIP package with the Windows OCR bridge included.
- P0: Remove the CI path that bakes cloud OCR credentials into public artifacts.
- P1: Add Windows ARM64 native builds after the Windows OCR bridge has a verified `win-arm64` artifact and test coverage.
- P2: Add macOS Developer ID signing/notarization and Windows code signing when certificates are available.

## Changes

- `.github/workflows/build.yml`
  - Build macOS with `wails build -platform darwin/universal`.
  - Publish `legal-extractor_${VERSION}_darwin_universal.dmg`.
  - Build Windows x64 with `wails build -platform windows/amd64`.
  - Build a Windows x64 NSIS installer with `wails build -platform windows/amd64 -nsis`.
  - Publish both `legal-extractor_${VERSION}_windows_amd64.zip` and `legal-extractor_${VERSION}_windows_amd64_setup.exe`.
  - Stop writing cloud service secrets into `internal/config/baked_conf.yaml`.
- `internal/config/config.go`
  - Remove embedded config loading.
  - Keep user configuration through `config/conf.yaml` and `LEGAL_EXTRACTOR_*` environment variables.
- `build/windows/installer/project.nsi`
  - Install `bridge_bin` so the Windows OCR fallback works from installed builds.
- `release_policy_test.go`
  - Lock the release workflow against credential embedding.
  - Lock the expected macOS universal and Windows x64 installer output names.
  - Lock the Windows installer OCR bridge packaging requirement.
- README and user docs
  - Document macOS universal and Windows x64 installer packaging.
  - Mark Windows ARM64 native support as planned until the bridge and release validation are ready.

## Verification

- `go test . -run 'TestReleaseWorkflow|TestWindowsInstaller' -count=1`
- `go test ./... -count=1`
- `go build . ./internal/...`
- `npm test` in `frontend/`
- `npm run build` in `frontend/`

Local Wails artifact builds are not part of this run because the current machine does not have the `wails` CLI installed. The GitHub release workflow installs Wails during release jobs.
