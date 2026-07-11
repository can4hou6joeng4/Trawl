# Desk UI Redesign Implementation Plan (Phase 1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the Vue frontend as the approved "Evidence Desk (取证台)" two-pane workbench with zero backend changes, per `docs/superpowers/specs/2026-07-10-desk-ui-redesign-design.md`.

**Architecture:** A three-stage App (`landing / parsing / desk`), a left source-page pane (placeholder page in this phase) and a right structured-records pane, reusing all existing Wails bindings, composables, and the error-code mapping layer. Visual/interaction source of truth: `designs/legal-extractor-redesign/取证台交互原型.html`.

**Tech Stack:** Vue 3, Vite, Vitest, existing Wails v2 bindings only.

---

## File Map

- Modify `frontend/src/style.css`: swap `:root` tokens to the desk palette (near-black bg, `#34d879`/`#18b25d` greens, mono accents); keep `.glass-panel`/`.btn` utility names working during migration.
- Modify `frontend/src/App.vue`: stage machine + desk grid; auto-preview on file select; export via `exportData` with hidden fields stripped.
- Add `frontend/src/components/LandingView.vue`: full-window viewfinder drop zone (absorbs `MainDropZone.vue` Wails drop logic).
- Add `frontend/src/components/ParsingView.vue`: scan progress + cancel (consumes `useExtractionProgress`).
- Add `frontend/src/components/DocPane.vue`: placeholder page card, page nav (buttons / arrow keys / page dots with record markers).
- Add `frontend/src/components/RecordCard.vue`: editable per-record card with collapse, edited marker, locate button (absorbs `PreviewTable.vue`).
- Add `frontend/src/components/FieldsPopover.vue`: field toggles, min 1 (absorbs field selection from `ConfigPanel.vue`).
- Add `frontend/src/components/ExportBar.vue`: format popover, output path, export button, success bar (absorbs rest of `ConfigPanel.vue` + `ResultCard.vue`).
- Modify `frontend/src/components/ActivationModal.vue`: restyle to desk theme (logic unchanged).
- Delete `frontend/src/components/MainDropZone.vue`, `PreviewTable.vue`, `ConfigPanel.vue`, `ResultCard.vue`, `TrialBanner.vue` (+ their test files, replaced by new ones).
- Add tests for each new component; adapt `App`-level flow assumptions in existing tests.

---

## Tasks

### 1. Theme + skeleton
- [ ] Swap `style.css` tokens to desk palette; verify existing components still render (temporary coexistence).
- [ ] Introduce stage state in `App.vue` (`landing | parsing | desk`) with the in-app top bar (brand, engine status, trial pill).

### 2. Landing + parsing
- [ ] `LandingView.vue` with Wails `OnFileDrop` + click-to-select; supported-format hints; error toasts via `friendlyErrorMessage`.
- [ ] Auto-run `previewData` on file selected → stage `parsing`; `ParsingView.vue` with progress/ETA/cancel; cancel → landing + neutral toast; failure → landing + error toast.

### 3. Desk
- [ ] `DocPane.vue` placeholder page + page nav (◀▶, arrow keys, page dots; dots mark record pages).
- [ ] `RecordCard.vue` editable fields (text/mono/textarea), edited marker, collapse, locate → sets DocPane page; page highlight-click → expand + flash card (page-level in this phase).
- [ ] `FieldsPopover.vue` (≥1 field) controlling visible rows; `ExportBar.vue` format popover + path (`selectOutputPath`) + export via `exportData` with hidden fields stripped; success bar with `openFile`.

### 4. Trial + cleanup
- [ ] Trial pill in top bar; restyled `ActivationModal.vue`; activated state.
- [ ] Delete replaced components/tests; remove dead styles.

### 5. Verification
- [ ] New/adapted Vitest suites pass; `npm run build` green; `go test ./...` untouched and green.
- [ ] Manual `wails dev` pass of the full flow at 1024×768 (zero window scroll on desk stage).
