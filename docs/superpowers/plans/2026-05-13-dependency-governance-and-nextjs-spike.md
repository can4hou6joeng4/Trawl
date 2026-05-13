# Dependency Governance And Next.js Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清理当前 Dependabot PR 队列中的可落地依赖升级，修复明确 CI 阻塞，并为 Next.js 技术验证建立可执行边界。

**Architecture:** 本轮不做全量框架重写，先保持 Wails + Go + Vue/Vite 主线可构建、可测试、可发布。Next.js 迁移作为独立技术验证项记录输入、风险和验收标准，避免把依赖治理与架构重构混在同一变更中。

**Tech Stack:** Go 1.25, Echo, Wails, Vue 3, Vite, TypeScript, Vitest, GitHub Actions, Docker, Next.js/Node.js 技术验证文档。

---

## Current PR Findings

- Open PRs are Dependabot-only dependency updates, not feature work.
- Green, low-risk CI/action PRs can be superseded by direct workflow dependency updates on this branch.
- `#27` Echo update fails lint because `middleware.Logger()` is deprecated in newer Echo.
- `#28` Vite 8 update fails because current `@vitejs/plugin-vue@5.2.4` does not accept Vite 8 as a peer dependency.
- Old PRs `#9` and `#4` failed on stale golangci-lint action/toolchain behavior and should be re-evaluated after this branch updates CI dependencies.

## File Map

- Modify `.github/workflows/ci.yml`: update GitHub Actions dependency versions where Dependabot has already proposed compatible versions.
- Modify `.github/workflows/build.yml`: align release Node runtime with Vite 8 requirements and update release/build GitHub Actions dependencies.
- Modify `cmd/server/main.go`: replace deprecated Echo request logger middleware with `RequestLoggerWithConfig`.
- Modify `go.mod` and `go.sum`: update Go dependencies represented by open Dependabot PRs.
- Modify `frontend/package.json` and `frontend/package-lock.json`: update Vue/Vite/TypeScript toolchain dependencies represented by open Dependabot PRs.
- Create `docs/superpowers/specs/2026-05-13-nextjs-web-spike-design.md`: document the Next.js web technical spike scope and acceptance criteria.

---

### Task 1: Save The Implementation Plan

**Files:**
- Create: `docs/superpowers/plans/2026-05-13-dependency-governance-and-nextjs-spike.md`

- [x] **Step 1: Write this implementation plan**

Run: `test -f docs/superpowers/plans/2026-05-13-dependency-governance-and-nextjs-spike.md`

Expected: exit code `0`.

---

### Task 2: Update CI And Release Dependency Actions

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/build.yml`

- [x] **Step 1: Update action versions**

Set CI golangci-lint action to `golangci/golangci-lint-action@v9`.

Set release workflow actions to:
- `docker/setup-qemu-action@v4`
- `docker/login-action@v4`
- `actions/setup-dotnet@v5`
- `softprops/action-gh-release@v3`

- [x] **Step 2: Align Node versions for Vite 8**

Use Node `22` in release jobs that run frontend builds. Keep CI frontend on Node `20` because current CI already uses Node 20.20.2, satisfying Vite 8's `^20.19.0 || >=22.12.0` engine range.

- [x] **Step 3: Verify workflow syntax by inspection**

Run: `git diff -- .github/workflows/ci.yml .github/workflows/build.yml`

Expected: only action version and Node runtime changes appear.

---

### Task 3: Fix Echo Lint Blocker

**Files:**
- Modify: `cmd/server/main.go`
- Test: CI lint equivalent after dependency update

- [x] **Step 1: Replace deprecated logger middleware**

Replace:

```go
e.Use(middleware.Logger())
```

with:

```go
e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
	LogURI:    true,
	LogStatus: true,
	LogError:  true,
	LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
		logger.Info("request", "uri", v.URI, "status", v.Status, "error", v.Error)
		return nil
	},
}))
```

- [x] **Step 2: Format Go code**

Run: `gofmt -w cmd/server/main.go`

Expected: command exits `0`.

---

### Task 4: Update Go Dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [x] **Step 1: Apply dependency updates**

Run:

```bash
go get github.com/labstack/echo/v4@v4.15.2 github.com/pdfcpu/pdfcpu@v0.12.1 github.com/xuri/excelize/v2@v2.10.1 github.com/wailsapp/wails/v2@v2.12.0
```

Expected: `go.mod` and `go.sum` update without errors.

- [x] **Step 2: Tidy modules**

Run: `go mod tidy`

Expected: command exits `0`.

---

### Task 5: Update Frontend Dependencies

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`

- [x] **Step 1: Apply npm dependency updates**

Run:

```bash
npm install vue@3.5.34 vite@8.0.10 @vitejs/plugin-vue@6.0.6 vue-tsc@3.2.8 typescript@6.0.3 --save-exact
```

Expected: install succeeds with no peer dependency error.

- [x] **Step 2: Inspect package changes**

Run: `git diff -- frontend/package.json frontend/package-lock.json`

Expected: only intended Vue/Vite/TypeScript toolchain versions change.

---

### Task 6: Document Next.js Web Technical Spike

**Files:**
- Create: `docs/superpowers/specs/2026-05-13-nextjs-web-spike-design.md`

- [x] **Step 1: Write the spike design**

The document must state:
- The spike is Web-first and does not replace Wails desktop in this branch.
- It validates upload, third-party OCR call, Markdown parsing, preview, export, error handling, and secret handling.
- It defines success criteria before any migration decision.

- [x] **Step 2: Confirm document exists**

Run: `test -f docs/superpowers/specs/2026-05-13-nextjs-web-spike-design.md`

Expected: exit code `0`.

---

### Task 7: Verify The Branch

**Files:**
- Verify all modified files

- [x] **Step 1: Run Go tests**

Run: `go test ./...`

Expected: all packages pass.

- [x] **Step 2: Run Go build check**

Run: `go build ./internal/... ./cmd/...`

Expected: command exits `0`.

- [x] **Step 3: Run frontend tests**

Run: `npm test` from `frontend/`

Expected: all Vitest tests pass.

- [x] **Step 4: Run frontend build**

Run: `npm run build` from `frontend/`

Expected: Vue typecheck and Vite production build pass.

- [x] **Step 5: Check final diff and status**

Run: `git diff --stat && git status --short --branch`

Expected: modified files match this plan and no unrelated files are changed.
