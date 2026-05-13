# Next.js Web Technical Spike Design

## Purpose

This spike evaluates whether the Web edition of Legal Document Extractor can move from the current Go/Echo backend plus Vue frontend to a smaller Next.js + Node.js + TypeScript + Tailwind CSS stack.

This is not a commitment to replace the Wails desktop application. The current Go code still owns native desktop bindings, local DOCX/PDF parsing, Windows OCR fallback, export helpers, cancellation, and release packaging. The spike only tests whether the Web path can be simplified when PDF parsing is delegated to third-party OCR APIs.

## Scope

The spike should implement one narrow Web flow:

1. Upload a `.pdf` or `.docx` file from a browser.
2. Validate file type and size before remote processing.
3. Call the configured third-party OCR/layout parsing API from a server-side route handler.
4. Parse returned Markdown into the same logical records used by the current extractor: defendant, ID number, requests, facts, and page.
5. Render an editable preview table.
6. Export edited records to CSV and JSON in-browser or through a server route.
7. Return clear errors for missing token, unsupported file type, remote timeout, remote non-200 response, empty OCR result, and cancellation/abort.

The spike should not include Wails packaging, native file dialogs, Windows OCR bridge integration, license/trial enforcement, Docker publishing, or a full migration of existing Vue components.

## Proposed Stack

- Next.js App Router with TypeScript.
- Route handlers for upload/OCR calls to keep API tokens server-side.
- Tailwind CSS for the spike UI.
- Shared pure TypeScript parser functions for Markdown-to-record conversion.
- Vitest or Next-compatible unit tests for parser and route helpers.
- Playwright only if the spike reaches browser-level workflow verification.

## Data Flow

Browser upload sends multipart form data to a Next.js route handler. The handler validates the file, reads API configuration from environment variables, calls the OCR provider, normalizes the provider response into Markdown pages, and passes those pages through a pure parser module. The browser receives structured records and renders the preview table. Export operates on edited records so users can correct OCR output before downloading.

Secrets must never be exposed to client components. OCR API tokens belong in server-only environment variables. Client components should only receive structured success/error payloads.

## Acceptance Criteria

The spike is successful only if all of these are true:

- A developer can run the Web flow locally with documented environment variables.
- PDF upload through the browser produces the same essential fields as the current Web API for representative samples.
- Missing or invalid OCR credentials produce a deterministic user-facing error.
- Large or slow OCR calls do not freeze the UI; users see progress or a clear pending state.
- Parser logic is isolated from framework code and covered by unit tests.
- The implementation does not require Go for the Web-only happy path.
- Remaining gaps versus the Go/Wails app are explicitly listed before any migration decision.

## Decision Rules

Proceed toward Web migration only if the spike reduces operational complexity without losing the product capabilities required for the Web edition. Keep Go for desktop and local extraction if native behavior remains part of the product goal.

Reject or pause migration if the spike recreates a substantial backend service in Node.js without reducing complexity, weakens file privacy/security, or requires duplicating the current extractor logic without better maintainability.

## Follow-Up Implementation Plan

If this spike is approved later, create a separate branch and implementation plan. Do not mix the spike implementation with dependency maintenance, CI fixes, or release workflow cleanup.
