# Desktop-Only Scope Design

## Purpose

Legal Document Extractor should become a desktop-only product. The Web, Docker, Go/Echo API, and Next.js spike paths add maintenance cost without serving the current product direction.

This design supersedes the earlier Web edition migration direction. The project should retain Wails + Vue + Go desktop behavior and remove current code, build jobs, and documentation that imply a supported browser/server edition.

## Product Boundary

The supported product surface is:

- macOS desktop application distributed as a `.dmg`.
- Windows desktop application distributed as a `.zip` or installer artifact.
- Wails-bound Vue UI for local file selection, preview, cancellation, export, trial status, and license activation.
- Go extraction core for local DOCX/PDF parsing, Baidu OCR integration, Windows OCR fallback, caching, export, and progress callbacks.

The unsupported surfaces are:

- Browser upload workflow.
- Go/Echo REST API.
- Docker or Docker Compose deployment.
- GHCR image publishing.
- Next.js Web edition or spike.
- Vue runtime fallback for non-Wails browser mode.

## Removal Scope

Remove these tracked production or spike files:

- `cmd/server/main.go`
- `Dockerfile`
- `docker-compose.yml`
- `.air.toml`
- `.dockerignore`
- `spikes/nextjs-web/`
- `docs/superpowers/specs/2026-05-13-nextjs-web-spike-design.md`
- `docs/superpowers/specs/2026-05-14-nextjs-web-spike-result.md`
- `docs/superpowers/plans/2026-05-14-nextjs-web-spike.md`
- `docs/contracts/extraction-response.md`
- `docs/superpowers/specs/2026-05-14-web-edition-refactor-design.md`
- `docs/superpowers/plans/2026-05-14-contract-and-infrastructure-baseline.md`

Keep historical release notes and older plan/spec files unless they are the newly added Web edition migration artifacts above. Historical files describe past work and should not be rewritten as if it never existed.

## Code Changes

### Frontend Service Layer

`frontend/src/services/api.ts` should expose one Wails desktop adapter. It should no longer:

- Detect Web mode.
- Export `isWebMode`.
- Instantiate `WebAdapter`.
- Reference `fetch`, `AbortController`, `VITE_API_URL`, `/api/extract`, or `/api/export`.
- Export `downloadBlob`.
- Return `File` objects from `selectFile`.

The service interface should use local file paths:

- `selectFile(): Promise<string>`
- `previewData(filePath: string, fields: string[]): Promise<ExtractResult>`
- `extractToPath(filePath: string, outputPath: string, fields: string[]): Promise<ExtractResult>`
- `exportData(records: Record[], outputPath: string): Promise<ExtractResult>`
- `scanFields(filePath: string): Promise<FieldOption[]>`

The exported `api` object should keep `api.isDesktop` as `true` for compatibility with existing components that gate desktop-only UI.

### Vue Components

`frontend/src/App.vue`, `frontend/src/components/MainDropZone.vue`, and `frontend/src/components/ConfigPanel.vue` should treat selected files as local path strings.

Remove browser-only branches:

- HTML5 Web drop handling.
- Web export/download branch.
- Web skip for output path selection.
- Web skip for opening output files.
- Activation banner condition on `api.isDesktop`.
- `File` display behavior.

Wails native file drop stays in `MainDropZone.vue`.

### CI And Release

`.github/workflows/ci.yml` should stop referencing `./cmd/...` because `cmd/server` is removed. Backend checks should cover `./internal/...` and the root Wails package.

`.github/workflows/build.yml` should remove the Docker build/push job and make the release job depend only on macOS and Windows desktop builds.

`.github/ISSUE_TEMPLATE/bug_report.yml` should remove Docker/Web as a runtime option.

## Documentation Changes

Current docs should describe only the desktop product:

- `README.md`
- `README.zh-CN.md`
- `docs/user/CONFIG_GUIDE.md`
- `docs/user/TROUBLESHOOTING.md`

They should remove Web/Docker quick start, server development instructions, Docker platform badges, Docker config instructions, and Docker troubleshooting.

The project structure should show `main.go`, `internal/app`, `internal/extractor`, `frontend`, and `build`; it should not show `cmd/server`, `Dockerfile`, or `docker-compose.yml`.

## Test Strategy

Tests should assert the absence of Web mode where practical:

- Replace `frontend/src/services/api.test.ts` with desktop-only service tests.
- Update `MainDropZone` tests to cover Wails/native desktop file handling and remove Web drop tests.
- Run frontend unit tests.
- Run Go tests across remaining packages.
- Run Go build for the Wails root and `internal` packages.
- Run frontend build.

## Acceptance Criteria

- No tracked `cmd/server`, Dockerfile, Compose file, or Next.js spike remains.
- No current README or user docs advertise Web or Docker support.
- No current CI/release workflow builds or publishes Docker images.
- Frontend service layer has no Web adapter or browser API calls.
- `rg "WebAdapter|isWebMode|VITE_API_URL|/api/extract|/api/export|spikes/nextjs-web|docker-compose|build-docker|cmd/server" README.md README.zh-CN.md docs/user .github frontend/src main.go internal` returns no active product references.
- `go test ./... -count=1` passes.
- `go build . ./internal/...` passes.
- `npm test` and `npm run build` pass in `frontend/`.
