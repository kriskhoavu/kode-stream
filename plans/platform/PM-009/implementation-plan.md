# Implementation Plan: PM-009 - Scoped Content Search

## Overview

Add bounded content search to item details and Explorer. Make configured workspace sources the default Explorer tree mode while preserving an explicit All Files mode.

## Phases Summary

| Phase | Name                                 | Status |
|-------|--------------------------------------|--------|
| B1    | Content Search Domain And Budgets    | ✅     |
| B2    | Scope Resolution And Application     | ✅     |
| B3    | Item And Explorer Search APIs        | ✅     |
| B4    | Search Safety And Scale Tests        | ✅     |
| F1    | Types And Content Search Clients     |        |
| F2    | Tree Mode And Search State           |        |
| F3    | Item And Explorer Search UI          |        |
| F4    | Styling, Accessibility, And Final QA |        |

## Backend Phases

### Phase B1: Content Search Domain And Budgets

**Deliverables:**

- [x] Add content-search request, result, response, and budget models.
- [x] Add literal UTF-8 line matching and bounded snippets.
- [x] Reuse PM-006 classification and binary detection.
- [x] Add result, file, byte, file-size, and query-length limits.
- [x] Support context cancellation.
- [x] Cover case sensitivity, Unicode, line/column, and snippets.

**Verification:** `rtk go test ./internal/workspacefiles ./internal/fileaccess`

**Draft Commit:**
```text
PM-009: Add bounded workspace content search

- Match literal text with line and column context
- Reuse classified text and binary safeguards
- Enforce predictable search budgets
```

### Phase B2: Scope Resolution And Application

**Deliverables:**

- [x] Add item-root search resolution.
- [x] Add configured-source and all-files root resolution.
- [x] Deduplicate nested canonical roots.
- [x] Batch ignore checks and skip outside symlinks.
- [x] Share one budget across workspaces and roots.
- [x] Add application service tests for every scope.

**Verification:** `rtk go test ./internal/application/contentsearch ./internal/workspacefiles`

**Draft Commit:**
```text
PM-009: Add scoped content search service

- Resolve item, configured-source, and full-root scopes
- Respect ignored and symlink boundaries
- Share bounded work across search roots
```

### Phase B3: Item And Explorer Search APIs

**Deliverables:**

- [x] Add item content-search endpoint.
- [x] Add Explorer content-search endpoint.
- [x] Validate query, mode, workspace, and case options.
- [x] Map item, workspace, safety, and cancellation errors.
- [x] Preserve PM-005 and PM-008 search contracts.
- [x] Add API regression tests.

**Verification:** `rtk go test ./internal/api ./internal/app`

**Draft Commit:**
```text
PM-009: Add scoped content search APIs

- Expose item-directory content search
- Expose Explorer mode-aware content search
- Preserve existing path and item search routes
```

### Phase B4: Search Safety And Scale Tests

**Deliverables:**

- [x] Cover sibling item and unconfigured-directory isolation.
- [x] Cover `.git`, ignored paths, outside symlinks, and binary files.
- [x] Cover large files, changing files, unreadable files, and invalid UTF-8.
- [x] Cover result, file, byte, and cancellation limits.
- [x] Run the complete backend suite.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-009: Add content search regression tests

- Protect item and Explorer scope boundaries
- Exercise binary, ignore, symlink, and budget safeguards
- Run complete backend regression coverage
```

## Frontend Phases

### Phase F1: Types And Content Search Clients

**Deliverables:**

- [ ] Add tree-mode and content-search types.
- [ ] Add normalized item and Explorer API methods.
- [ ] Encode mode, workspace, ignored, and case options.
- [ ] Add API client tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-009: Add content search frontend contracts

- Add item and Explorer content search clients
- Add Configured Sources and All Files mode types
- Normalize optional search response fields
```

### Phase F2: Tree Mode And Search State

**Deliverables:**

- [ ] Add reusable debounced content-search state.
- [ ] Add stale-response and query-reset behavior.
- [ ] Add Configured Sources default and persisted All Files mode.
- [ ] Add mode-specific tree cache keys and source root composition.
- [ ] Add selected match context and result-opening helpers.
- [ ] Add hook and pure-helper tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-009: Add scoped content search state

- Debounce and cancel stale content results
- Scope Explorer trees and searches by mode
- Preserve selected line context when opening files
```

### Phase F3: Item And Explorer Search UI

**Deliverables:**

- [ ] Add item details content-search input and results.
- [ ] Add Explorer tree-mode control.
- [ ] Keep Explorer Paths and Content search distinct.
- [ ] Add all/current workspace content-search scope.
- [ ] Add keyboard result navigation and match highlighting.
- [ ] Save pending Markdown before opening another result.
- [ ] Add component and integration tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-009: Add scoped content search interfaces

- Search within the current item directory
- Add mode-aware Explorer content search
- Open highlighted line matches safely
```

### Phase F4: Styling, Accessibility, And Final QA

**Deliverables:**

- [ ] Add feature-owned responsive search and mode styles.
- [ ] Add stable loading, empty, error, and truncated states.
- [ ] Verify keyboard, mobile, light, and dark behavior.
- [ ] Run backend, frontend, dependency, and production-build checks.
- [ ] Update architecture and PM-009 documents.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**
```text
PM-009: Finalize scoped content search UX

- Add responsive and accessible search styling
- Complete safety and regression verification
- Update architecture and planning documents
```

## Post-Implementation Checklist

- [ ] Confirm item search cannot read sibling item directories.
- [ ] Confirm Sources mode cannot search unconfigured directories.
- [ ] Confirm All Files preserves PM-008 full-tree behavior.
- [ ] Confirm `.git`, outside symlinks, and binary files are excluded.
- [ ] Confirm ignored preference matches tree and search behavior.
- [ ] Confirm path-name search remains distinct and functional.
- [ ] Confirm result selection preserves pending Markdown safely.
- [ ] Run full backend and frontend suites.
- [ ] Run production build and record bundle output.
