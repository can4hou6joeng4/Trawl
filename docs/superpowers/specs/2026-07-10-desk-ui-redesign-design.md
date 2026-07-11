# Desk UI Redesign Design (取证台)

## Purpose

Replace the current single-column "web page" layout of the desktop frontend with the approved "Evidence Desk (取证台)" design: a two-pane workbench where the source document (left) and structured records (right) sit side by side, with bidirectional locate interactions. The approved interactive prototype lives at `designs/legal-extractor-redesign/取证台交互原型.html` (local, untracked) and is the visual + interaction source of truth.

Product scope, extraction flow, and all existing Wails bindings stay unchanged. This is a frontend re-architecture, phased so that Phase 1 ships with zero backend changes.

## UX Model

Three stages replace the stacked single column:

1. **Landing** — full-window viewfinder drop zone (drag or click to select). Trial pill in the in-app top bar.
2. **Parsing** — page-scan progress view driven by existing `extraction_progress` events; cancellable via `CancelExtraction`; errors surface as toasts using the existing `services/errors.ts` code mapping (CANCELLED is neutral).
3. **Desk** — left: source page pane with page navigation (page dots mark pages that produced records); right: one editable record card per extracted record, a fields popover (⚙ 字段 n/4), and a pinned export bar (format switch + path + export). Bidirectional locate: record card ⇄ source page.

Flow change vs today: selecting a file **auto-runs preview** (`PreviewData` with all fields) and lands on the Desk — there is no separate 预览 button. Export always goes through `ExportData(editedRecords, path)`, so manual corrections are preserved (this wiring already exists). Field toggles are a frontend concern: hidden fields are stripped from records before export.

## Phasing

- **Phase 1 — layout migration (no backend changes).** The left pane renders a neutral placeholder page (as in the prototype) with real page navigation and page-level record linking. All bindings used are the existing ones: `SelectFile`, `PreviewData`, `ExportData`, `SelectOutputPath`, `OpenFile`, `CancelExtraction`, `GetTrialStatus`, `GetMachineID`, `Activate`, `extraction_progress` events, native file drop.
- **Phase 2a — real page rendering.** Add one additive Go binding `ReadFileBase64(path)` and render actual PDF pages with pdf.js (DOCX/images get a document placeholder).
- **Phase 2b — field-level highlight link-back.** Requires the extractor to emit per-field positions (`{page, bbox}`). Baidu layout parsing already returns positional data we currently discard; the local text-layer path can start page-level only. This is the previously recorded deferred decision item and needs its own design.

## Component Mapping

| Current | Becomes |
|---|---|
| `App.vue` (stage-less) | `App.vue` stage machine (`landing / parsing / desk`) + desk grid |
| `MainDropZone.vue` | `LandingView.vue` (full-window drop zone, Wails `OnFileDrop` kept) |
| — | `ParsingView.vue` (progress, cancel; consumes `useExtractionProgress`) |
| `PreviewTable.vue` | `RecordCard.vue` list (per-record card, inline-editable fields, edited marker, collapse) |
| `ConfigPanel.vue` | `FieldsPopover.vue` (min 1 field) + `ExportBar.vue` (format popover, path, export button) |
| `ResultCard.vue` | export success bar inside `ExportBar.vue` (path + 打开文件) |
| `TrialBanner.vue` | trial pill in the in-app top bar (opens `ActivationModal`) |
| `ActivationModal.vue` | kept, restyled to desk theme |
| — | `DocPane.vue` (page card + page nav; placeholder in Phase 1, pdf.js in Phase 2a) |

`useNotification` / `useExtractionProgress` composables and `services/` (api adapter, errors mapping, types) are reused as-is. `style.css` tokens switch from the glass/Trust palette to the desk palette (near-black `#07100c`, greens `#34d879`/`#18b25d`, mono accents) — single token swap on `:root`.

Window chrome stays the OS default (no frameless mode); the in-app top bar carries brand, engine status, and the trial pill only.

## Test Strategy

- Adapt existing component tests to the new components (drop zone behaviors, activation modal, notification/progress composables largely carry over).
- New tests: stage transitions (select → parsing → desk; cancel → landing), RecordCard editing marks record edited and export payload contains corrections, FieldsPopover enforces ≥1 field and strips hidden fields from export payload, DocPane page nav bounds and record-page dots, export success bar rendering.
- `npm run build` (vue-tsc) and full Go suite must stay green; no Go changes expected in Phase 1.

## Acceptance Criteria (Phase 1)

- Desk stage fits 1024×768 with zero scrolling of the window itself (record list scrolls internally).
- Full flow works against the real backend: drop/select → auto preview → edit a field → export → success bar opens the file; cancel during parsing returns to landing with a neutral toast.
- Stable error codes still surface through `friendlyErrorMessage`; trial gating and activation unchanged.
- All frontend tests pass; CI green.
