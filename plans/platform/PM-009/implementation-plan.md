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
| B5    | Safe Text File Editing               | ✅     |
| F1    | Types And Content Search Clients     | ✅     |
| F2    | Tree Mode And Search State           | ✅     |
| F3    | Item And Explorer Search UI          | ✅     |
| F4    | Styling, Accessibility, And Final QA | ✅     |
| F5    | Compact Search Result UX             | ✅     |
| F6    | Unified Explorer Search              | ✅     |
| F7    | Shared Explorer Result Cards         | ✅     |

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

### Phase B5: Safe Text File Editing

**Deliverables:**

- [x] Mark loaded text, code, and structured files as editable.
- [x] Preserve stale-hash, path, binary, UTF-8, and response-size safeguards.
- [x] Allow Explorer and item Raw mode saves for supported text files.
- [x] Cover item and workspace text-file writes.

**Verification:** `rtk go test ./internal/fileaccess ./internal/workspacefiles ./internal/application/item ./internal/application/workspacefiles ./internal/api`

**Draft Commit:**
```text
PM-009: Enable safe raw text editing

- Allow Raw mode to save supported text files
- Preserve binary, size, path, and stale-hash guards
- Cover item and workspace text-file writes
```

## Frontend Phases

### Phase F1: Types And Content Search Clients

**Deliverables:**

- [x] Add tree-mode and content-search types.
- [x] Add normalized item and Explorer API methods.
- [x] Encode mode, workspace, ignored, and case options.
- [x] Add API client tests.

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

- [x] Add reusable debounced content-search state.
- [x] Add stale-response and query-reset behavior.
- [x] Add Configured Sources default and persisted All Files mode.
- [x] Add mode-specific tree cache keys and source root composition.
- [x] Add selected match context and result-opening helpers.
- [x] Add hook and pure-helper tests.

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

- [x] Add item details content-search input and results.
- [x] Add Explorer tree-mode control.
- [x] Keep Explorer Paths and Content search distinct.
- [x] Add all/current workspace content-search scope.
- [x] Add keyboard result navigation and match highlighting.
- [x] Save pending Markdown before opening another result.
- [x] Add component and integration tests.

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

- [x] Add feature-owned responsive search and mode styles.
- [x] Add stable loading, empty, error, and truncated states.
- [x] Verify keyboard, mobile, light, and dark behavior.
- [x] Run backend, frontend, dependency, and production-build checks.
- [x] Update architecture and PM-009 documents.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**
```text
PM-009: Finalize scoped content search UX

- Add responsive and accessible search styling
- Complete safety and regression verification
- Update architecture and planning documents
```

### Phase F5: Compact Search Result UX

**Deliverables:**

- [x] Keep file names and line numbers on one compact header row.
- [x] Collapse long paths to the nearest two parent directories.
- [x] Clamp snippets to two lines and 120 characters while retaining the match.
- [x] Remove repeated column metadata from every result row.
- [x] Show at most 20 results and ask users to refine broader queries.
- [x] Preserve full path and column context for assistive technology and tooltips.
- [x] Add compact-result regression tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-009: Compact content search results

- Reduce repeated metadata in narrow panels
- Clamp paths, snippets, and visible result counts
- Preserve keyboard and accessible result context
```

### Phase F6: Unified Explorer Search

**Deliverables:**

- [x] Replace the Paths and Content tabs with one search box.
- [x] Show file-name and text matches in one result list.
- [x] Infer one-workspace or all-workspace scope from the current selection.
- [x] Remove the search-scope menu, ignored-files menu, case toggle, and clear icon.
- [x] Rename tree modes to Planning folders and Entire workspace.
- [x] Preserve keyboard navigation and safe result opening.
- [x] Add simplified-search regression coverage.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-009: Simplify Explorer search controls

- Merge file-name and text search into one interaction
- Infer workspace scope from the current selection
- Remove ambiguous search controls from narrow panels
```

### Phase F7: Shared Explorer Result Cards

**Deliverables:**

- [x] Render Explorer filename matches with the shared content-search card style.
- [x] Render Explorer text matches through the same shared result component as item details.
- [x] Use matching filename, badge, location, highlight, hover, and keyboard states.
- [x] Preserve file/folder type context for filename matches.
- [x] Add Explorer card-style regression coverage.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-009: Unify Explorer search result cards

- Reuse item-detail result cards in Explorer
- Match filename and text result spacing and states
- Preserve result type and location context
```

## Post-Implementation Checklist

- [x] Confirm item search cannot read sibling item directories.
- [x] Confirm Sources mode cannot search unconfigured directories.
- [x] Confirm All Files preserves PM-008 full-tree behavior.
- [x] Confirm `.git`, outside symlinks, and binary files are excluded.
- [x] Confirm ignored preference matches tree and search behavior.
- [x] Confirm path-name search remains functional in unified results.
- [x] Confirm result selection preserves pending Markdown safely.
- [x] Run full backend and frontend suites.
- [x] Run production build and record bundle output.

## Final Verification

- Backend: 147 tests passed across 26 packages.
- Frontend: 66 tests passed across 22 files.
- Production: main JavaScript 316.59 kB (91.78 kB gzip); main CSS 70.36 kB (12.60 kB gzip).
- Dependencies: Go modules verified and the npm dependency tree resolved; npm audit was unavailable because the configured internal registry did not resolve.
- Browser automation: unavailable in this session; component tests cover keyboard controls, mode switching, highlighting, and responsive state structure.
