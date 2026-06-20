# Implementation Plan: PM-008 - Explorer Productivity Enhancements

## Overview

Add bounded path search, Git tree decorations, guarded Markdown and directory creation, and safe rename operations to the PM-007 Workspace Explorer.

## Phases Summary

| Phase | Name                             | Status |
|-------|----------------------------------|--------|
| B1    | Guarded Create And Rename Domain | ✅     |
| B2    | Path Search And Git State Domain | ✅     |
| B3    | Application Service And APIs     | ✅     |
| B4    | Backend Safety And Scale Tests   | ✅     |
| F1    | Types And API Clients            | ✅     |
| F2    | Search, Git, And Mutation State  | ✅     |
| F3    | Explorer Productivity UI         | ✅     |
| F4    | Responsive Styling And Final QA  | ✅     |

## Backend Phases

### Phase B1: Guarded Create And Rename Domain

**Deliverables:**

- [x] Add create, rename, and mutation result models.
- [x] Add single-segment name validation.
- [x] Add exclusive Markdown creation and one-directory creation.
- [x] Add guarded same-workspace rename without overwrite.
- [x] Return precise parent cache invalidation paths.
- [x] Cover traversal, `.git`, symlink, format, collision, and root safeguards.

**Verification:** `rtk go test ./internal/workspacefiles ./internal/security/pathguard`

**Draft Commit:**
```text
PM-008: Add guarded workspace path mutations

- Create Markdown files and directories safely
- Rename workspace paths without overwrite
- Cover protected and collision cases
```

### Phase B2: Path Search And Git State Domain

**Deliverables:**

- [x] Add bounded search models and traversal.
- [x] Batch ignored-path checks by directory.
- [x] Skip `.git` and outside symlinks.
- [x] Add deterministic result ordering and truncation.
- [x] Normalize Git changes into path state responses.
- [x] Cover unloaded, ignored, deep, large, and renamed paths.

**Verification:** `rtk go test ./internal/workspacefiles ./internal/gitadapter`

**Draft Commit:**
```text
PM-008: Add workspace path search and Git state

- Search bounded unloaded workspace paths
- Respect ignored and protected entries
- Normalize Git state for Explorer rows
```

### Phase B3: Application Service And APIs

**Deliverables:**

- [x] Add search, create, rename, and Git state service methods.
- [x] Add audit events for successful and blocked mutations.
- [x] Refresh configured sources only when affected.
- [x] Add all proposed HTTP routes and error mapping.
- [x] Preserve PM-007 route contracts.
- [x] Add application and API regression tests.

**Verification:** `rtk go test ./internal/application/workspacefiles ./internal/api ./internal/app`

**Draft Commit:**
```text
PM-008: Add Explorer productivity APIs

- Expose path search and Git decorations
- Add guarded create and rename routes
- Audit mutations and refresh affected sources
```

### Phase B4: Backend Safety And Scale Tests

**Deliverables:**

- [x] Cover search result and visited-entry limits.
- [x] Cover ignored and outside-symlink traversal.
- [x] Cover exclusive create and no-overwrite rename behavior.
- [x] Cover audit and targeted refresh results.
- [x] Run the complete backend suite.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-008: Add Explorer productivity regression tests

- Exercise search limits and ignore rules
- Protect create and rename safeguards
- Run complete backend regression coverage
```

## Frontend Phases

### Phase F1: Types And API Clients

**Deliverables:**

- [x] Add search, Git state, create, rename, and mutation result types.
- [x] Add normalized API client methods.
- [x] Add API contract tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-008: Add Explorer productivity contracts

- Add path search and Git state clients
- Add create and rename clients
- Normalize optional response collections
```

### Phase F2: Search, Git, And Mutation State

**Deliverables:**

- [x] Add debounced cancellable search state.
- [x] Add Git path state normalization and loading.
- [x] Add create and rename mutation state.
- [x] Add targeted directory cache invalidation.
- [x] Add expand-to-result behavior.
- [x] Add helper and hook tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-008: Add Explorer productivity state

- Search unloaded workspace paths
- Cache Git decorations by path
- Refresh targeted directories after mutations
```

### Phase F3: Explorer Productivity UI

**Deliverables:**

- [x] Add search results with keyboard navigation.
- [x] Add accessible Git state markers to tree rows.
- [x] Add create file and directory dialogs.
- [x] Add confirmed rename dialog.
- [x] Save pending Markdown before rename.
- [x] Add component and integration tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-008: Add Explorer productivity controls

- Add repository-wide path search UI
- Show Git state on workspace rows
- Create and rename guarded workspace paths
```

### Phase F4: Responsive Styling And Final QA

**Deliverables:**

- [x] Add feature-owned responsive search, marker, menu, and dialog styles.
- [x] Verify keyboard, mobile, light, and dark states.
- [x] Run backend, frontend, dependency, and production-build checks.
- [x] Update architecture and PM-008 documents.

**QA record:** 133 Go tests and 56 frontend tests pass. TypeScript, dependency integrity, production build, and post-build Go tests pass. The initial JavaScript bundle is 310.40 kB (90.01 kB gzip). Explorer remains lazy-loaded at 25.16 kB (7.95 kB gzip). Responsive and theme states are covered by feature CSS and component behavior. The in-app browser was unavailable, so no live screenshots were captured.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**
```text
PM-008: Finalize Explorer productivity UX

- Add responsive productivity styling
- Complete accessibility and regression checks
- Update architecture and planning documents
```

## Post-Implementation Checklist

- [x] Confirm no operation can expose or modify `.git`.
- [x] Confirm search and rename cannot follow outside symlinks.
- [x] Confirm create and rename never overwrite a destination.
- [x] Confirm ignored mode controls search and tree behavior consistently.
- [x] Confirm Git status uses one call per workspace refresh.
- [x] Confirm item editor and PM-007 Explorer regressions pass.
- [x] Run full backend and frontend suites.
- [x] Run the production build and record bundle output.
