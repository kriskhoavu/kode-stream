# Implementation Plan: PM-006 - Rich Content Viewer

## Overview

Replace direct page-level Markdown rendering with one secure, extensible content viewer. Add bounded backend file classification, then add Markdown, HTML, source code, JSON, YAML, and KaTeX renderers without changing existing editing or navigation workflows.

## Terminology Lock

All code and UI text must use these names:

- `ContentViewer` for the shared preview component.
- `FileKind` for backend and frontend format groups.
- `ViewerMode` for rendered, structured, and source modes.
- `StructuredDataView` for JSON and YAML trees.
- `SourceCodeView` for highlighted source.
- `HTML Preview` for the sandboxed standalone HTML mode.

## Phases Summary

| Phase | Name                                 | Status |
|-------|--------------------------------------|--------|
| B1    | File Classification And Limits       | ✅     |
| B2    | Viewer API Metadata                  | ✅     |
| B3    | Guarded Read Integration             | ✅     |
| B4    | Backend Regression Tests             | ✅     |
| F1    | Viewer Types And Rendering Pipeline  | ✅     |
| F2    | Structured And Source Views          | ✅     |
| F3    | Shared Viewer Integration            | ✅     |
| F4    | Security, Performance, And Visual QA | ✅     |

## Backend Phases

### Phase B1: File Classification And Limits

**Deliverables:**

- [x] Add `FileKind` constants.
- [x] Move extension and special-filename mapping into `internal/fileaccess/classify.go`.
- [x] Add syntax language mapping for common code formats.
- [x] Add binary detection and named size thresholds.
- [x] Add table tests for classification and boundary behavior.

**Verification:** `rtk go test ./internal/fileaccess`

**Draft Commit:**
```text
PM-006: Add viewer file classification

- Classify supported text formats
- Add binary and preview size guards
- Cover extension and boundary behavior
```

---

### Phase B2: Viewer API Metadata

**Deliverables:**

- [x] Add `kind`, `sizeBytes`, `truncated`, and `editable` to `models.FileContent`.
- [x] Keep current JSON fields and route shape stable.
- [x] Add frontend-compatible constants and response fixtures.
- [x] Document backward compatibility in model contract tests.

**Verification:** `rtk go test ./internal/models ./internal/api`

**Draft Commit:**
```text
PM-006: Add viewer metadata to file responses

- Extend file content with viewer metadata
- Preserve existing response fields
- Add API compatibility coverage
```

---

### Phase B3: Guarded Read Integration

**Deliverables:**

- [x] Apply classification and bounded reads in `fileaccess.Read`.
- [x] Reject binary content without returning binary bytes.
- [x] Preserve full Markdown content and hash behavior within limits.
- [x] Keep `WriteMarkdown` rules and stale-write checks unchanged.
- [x] Map unsupported content to a clear API error.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-006: Integrate guarded viewer file reads

- Return classified text content
- Bound large preview responses
- Preserve Markdown write behavior
```

---

### Phase B4: Backend Regression Tests

**Deliverables:**

- [x] Cover Markdown, HTML, JSON, YAML, code, text, and unsupported files.
- [x] Cover binary, invalid UTF-8, symlink, and maximum-size cases.
- [x] Prove existing file tree, read, save, hash, and API routes remain stable.
- [x] Run the complete backend suite.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-006: Add viewer backend regression tests

- Cover supported and unsupported file reads
- Cover size and binary safeguards
- Protect existing Markdown workflows
```

## Frontend Phases

### Phase F1: Viewer Types And Rendering Pipeline

**Deliverables:**

- [x] Add `FileKind`, `ViewerMode`, and extended `FileContent` types.
- [x] Add renderer dependencies with a reviewed lockfile diff.
- [x] Build the sanitized GFM, KaTeX, and fenced-code pipeline.
- [x] Build the sanitized sandboxed HTML preview.
- [x] Add malicious-content and rendering tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-006: Add secure rich content renderers

- Add viewer types and lazy renderer dependencies
- Render sanitized Markdown and KaTeX
- Add sandboxed HTML preview tests
```

---

### Phase F2: Structured And Source Views

**Deliverables:**

- [x] Add JSON and safe YAML parsing.
- [x] Add bounded, accessible structured tree nodes.
- [x] Add source syntax highlighting and plain-text fallback.
- [x] Add copy, line number, and wrapping controls.
- [x] Add parse-error and large-file fallback states.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-006: Add structured data and source views

- Add JSON and YAML tree rendering
- Add highlighted source controls
- Add parse and large-file fallbacks
```

---

### Phase F3: Shared Viewer Integration

**Deliverables:**

- [x] Add the `ContentViewer` orchestrator and local error boundary.
- [x] Integrate it into `ItemWorkspacePage`.
- [x] Integrate it into the Kanban preview drawer.
- [x] Remove direct `marked.parse()` and duplicate preview output.
- [x] Preserve Preview, Raw, Diff, autosave, and file selection behavior.
- [x] Add shared viewer integration tests and run both page regression suites.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-006: Integrate the shared content viewer

- Use one viewer in item and Kanban previews
- Preserve raw editing and diff workflows
- Add shared integration coverage
```

---

### Phase F4: Security, Performance, And Visual QA

**Deliverables:**

- [x] Move viewer styles into a feature-owned stylesheet.
- [x] Lazy-load heavy format adapters and language definitions.
- [x] Add memoization and bounded rendering for large content.
- [x] Verify sanitization rules and iframe CSP with automated tests.
- [x] Add responsive and theme-aware styles for the drawer and full workspace. Screenshot verification was unavailable because the in-app browser could not start.
- [x] Compare production bundle output and run the full build.
- [x] Update architecture and PM-006 documents with final decisions.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**
```text
PM-006: Finalize rich viewer performance and styling

- Add responsive viewer styles
- Lazy-load heavy renderers
- Complete security and visual verification
```

## Migration Strategy

1. Add API metadata without changing routes or removing fields.
2. Build renderers behind tests before changing either page.
3. Integrate the item workspace first and compare current Markdown behavior.
4. Integrate the Kanban drawer through the same component.
5. Remove direct `marked` usage only after both integrations pass.
6. Add lazy loading and size guards after functional parity is covered.
7. Keep source fallback available throughout the migration.

## Rollback Strategy

- Each phase is independently revertible.
- The old page tabs remain intact during integration.
- If a rich adapter fails, route the file to escaped source view.
- Backend fields are additive, so reverting frontend integration does not break the API.
- Do not remove `marked` until the shared Markdown renderer passes both integration suites.

## Post-Implementation Checklist

- [x] Update `plans/platform/PM-006/` with final package and dependency names.
- [x] Update architecture documentation with `features/content-viewer` ownership.
- [x] Confirm no direct `marked.parse()` remains in page components.
- [x] Limit `dangerouslySetInnerHTML` to sanitized Markdown and escaped highlighter output.
- [x] Confirm iframe sandbox has no script or same-origin permission.
- [x] Run backend and frontend full test suites.
- [x] Run production build and compare chunk sizes.
- [x] Record that browser screenshot verification was unavailable in this session.
