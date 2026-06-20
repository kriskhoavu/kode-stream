# Implementation Plan: PM-007 - Workspace Explorer

## Overview

Add a global filesystem explorer beside Kanban. Show every registered workspace as a root, load real directories lazily, decorate indexed item folders, preview files through PM-006, and edit Markdown with the same Preview, Raw, Diff, autosave, stale-write, and revert behavior as item details.

## Terminology Lock

All code and UI text must use these names:

- `WorkspaceExplorerPage` for the global route.
- `WorkspaceTreeEntry` for real directory and file nodes.
- `WorkspaceDirectoryListing` for immediate lazy children.
- `Workspace File` for a file addressed by workspace ID and relative path.
- `ExplorerItemDecoration` for indexed metadata on a real directory.
- `ExplorerTree` for the left workspace hierarchy pane.
- `WorkspaceFileEditor` for the center Preview, Raw, and Diff pane.
- `ExplorerInspector` for the right context pane.
- `useFileEditorSession` for shared details and Explorer edit behavior.
- `expandedNodeIds` for persisted expansion state.

## Phases Summary

| Phase | Name                                 | Status |
|-------|--------------------------------------|--------|
| B1    | Workspace Path And Directory Domain  | ✅     |
| B2    | Workspace File Application Service   |        |
| B3    | Workspace File APIs                  |        |
| B4    | Backend Safety And Scale Tests       |        |
| F1    | Types, API, And Explorer Route       |        |
| F2    | Tree State And Shared Editor Session |        |
| F3    | Filesystem Explorer And Editor UI    |        |
| F4    | Styling, Performance, And QA         |        |

## Backend Phases

### Phase B1: Workspace Path And Directory Domain

**Deliverables:**

- [x] Add workspace directory listing, tree entry, save input, and revert input models.
- [x] Add workspace-root path validation with traversal, absolute, `.git`, and symlink guards.
- [x] Extract or export one shared PM-006 file classification policy.
- [x] Add one-level directory listing with directory-first natural sorting.
- [x] Add batched Git-ignore detection and `includeIgnored` behavior.
- [x] Cover hidden files, empty directories, nested paths, and special filenames.

**Verification:** `rtk go test ./internal/workspacefiles ./internal/fileaccess ./internal/security/pathguard`

**Draft Commit:**
```text
PM-007: Add guarded workspace directory access

- Add lazy workspace directory models and listing
- Protect workspace paths and Git internals
- Reuse shared file classification
```

---

### Phase B2: Workspace File Application Service

**Deliverables:**

- [ ] Add `internal/application/workspacefiles.Service`.
- [ ] Add classified workspace file reads with PM-006 limits.
- [ ] Add Markdown-only atomic writes with required expected hashes.
- [ ] Preserve file permissions where supported.
- [ ] Add selected-file diff and revert operations.
- [ ] Add audit events and targeted configured-source refresh decisions.
- [ ] Add service tests for read, save, diff, revert, audit, and refresh behavior.

**Verification:** `rtk go test ./internal/application/workspacefiles`

**Draft Commit:**
```text
PM-007: Add workspace file application service

- Read and save guarded workspace Markdown
- Add selected-file diff and revert
- Audit writes and refresh affected item sources
```

---

### Phase B3: Workspace File APIs

**Deliverables:**

- [ ] Add `GET /api/workspaces/{id}/tree`.
- [ ] Add `GET` and `PUT /api/workspaces/{id}/files`.
- [ ] Add `GET /api/workspaces/{id}/files/diff`.
- [ ] Add `POST /api/workspaces/{id}/files/revert`.
- [ ] Wire workspace file service in app startup and API construction.
- [ ] Map not-found, protected-path, unsupported, stale-hash, and Git errors.
- [ ] Keep existing item file APIs unchanged.

**Verification:** `rtk go test ./internal/api ./internal/app`

**Draft Commit:**
```text
PM-007: Add workspace tree and file APIs

- Expose lazy workspace directory listings
- Add guarded file read, save, diff, and revert routes
- Preserve existing item API contracts
```

---

### Phase B4: Backend Safety And Scale Tests

**Deliverables:**

- [ ] Cover traversal, `.git`, outside symlink, wrong type, and missing paths.
- [ ] Cover ignored entry visibility and batch Git behavior.
- [ ] Cover Markdown hash conflicts, atomic saves, permissions, and audit events.
- [ ] Cover binary and non-Markdown write rejection.
- [ ] Add a large immediate-directory test and prove deep trees remain unloaded.
- [ ] Run the complete backend suite.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-007: Add workspace explorer backend regression tests

- Cover workspace path and write safeguards
- Exercise ignored and large directory behavior
- Protect existing backend workflows
```

## Frontend Phases

### Phase F1: Types, API, And Explorer Route

**Deliverables:**

- [ ] Add workspace listing, entry, save, diff, and revert types.
- [ ] Add API client methods with response normalization.
- [ ] Add `/explorer` and workspace/path query helpers.
- [ ] Add Explorer to desktop and mobile navigation beside Kanban.
- [ ] Add explicit Open Kanban behavior for workspace roots.
- [ ] Add API and route regression tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-007: Add workspace explorer contracts and route

- Add directory and workspace file API clients
- Add global explorer route and navigation
- Cover query paths and response normalization
```

---

### Phase F2: Tree State And Shared Editor Session

**Deliverables:**

- [ ] Add global workspace roots and all-workspace item decoration loading.
- [ ] Add lazy directory cache keyed by workspace, path, and ignored mode.
- [ ] Add pure visible-tree flattening, filtering, and item decoration helpers.
- [ ] Add route-backed selection and local expansion persistence.
- [ ] Add ignored-file preference and cache invalidation on refresh.
- [ ] Extract `useFileEditorSession` from item details without behavior changes.
- [ ] Adapt item details to the shared editor session.
- [ ] Add roving focus and tree keyboard behavior.
- [ ] Evaluate and record the virtualization dependency decision.
- [ ] Add helper, hook, editor, and item detail regression tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-007: Add explorer state and shared file editor

- Cache and flatten lazy workspace directories
- Persist selection and keyboard navigation
- Share autosave and conflict behavior with item details
```

---

### Phase F3: Filesystem Explorer And Editor UI

**Deliverables:**

- [ ] Add `WorkspaceExplorerPage`, toolbar, tree, and workspace/directory/file rows.
- [ ] Add enriched item directory rows with Open details.
- [ ] Add `WorkspaceFileEditor` with breadcrumbs and Preview, Raw, and Diff.
- [ ] Reuse PM-006 for previews and shared session for Markdown autosave.
- [ ] Add stale-content recovery, immediate save before selection, and revert confirmation.
- [ ] Add file, item, Git, warning, and health inspector sections.
- [ ] Add Open Kanban, copy path, reveal, collapse, ignored toggle, and refresh actions.
- [ ] Add resizable and collapsible pane behavior.
- [ ] Add component and integration tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-007: Add filesystem workspace explorer

- Render real workspace directory trees
- Preview and edit Markdown with detail-mode behavior
- Add item decorations and context inspector
```

---

### Phase F4: Styling, Performance, And QA

**Deliverables:**

- [ ] Add feature-owned responsive explorer styles.
- [ ] Add stable row, tab, editor, and pane dimensions for all states.
- [ ] Add virtualization only when justified by F2 measurements.
- [ ] Verify ignored toggle, search, persistence, autosave, stale hash, diff, and revert.
- [ ] Verify desktop, tablet, mobile, light, and dark layouts.
- [ ] Run full frontend, backend, production build, and dependency checks.
- [ ] Update architecture and PM-007 documentation.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**
```text
PM-007: Finalize workspace explorer UX

- Add responsive filesystem explorer styling
- Optimize lazy trees and file editor behavior
- Complete visual and regression verification
```

## Migration Strategy

1. Add guarded directory listing without frontend exposure.
2. Add read-only workspace file APIs and safety tests.
3. Add Markdown save, diff, revert, audit, and refresh behavior.
4. Add frontend route, roots, and lazy directory state.
5. Extract shared editor behavior behind item detail regression tests.
6. Add Explorer Preview and read-only formats.
7. Enable Markdown Raw, Diff, autosave, and revert through the shared session.
8. Add item decorations, inspector, persistence, keyboard behavior, and optional virtualization.
9. Keep existing Kanban and item detail routes available throughout migration.

## Rollback Strategy

- Workspace tree and file routes are additive.
- Removing Explorer navigation restores the previous workflow.
- Existing item file routes remain unchanged.
- Item details keep the same external props and behavior after editor extraction.
- Explorer preferences are local and can be ignored or cleared.
- No managed workspace schema or item index migration is required.

## Post-Implementation Checklist

- [ ] Update `plans/platform/PM-007/` with final package and dependency names.
- [ ] Update architecture documentation with workspace-root file safety and editor sharing.
- [ ] Confirm `.git`, traversal, and outside symlinks are inaccessible.
- [ ] Confirm directory expansion never recursively walks unloaded descendants.
- [ ] Confirm non-Markdown files remain read-only.
- [ ] Confirm item details and Explorer share autosave, stale-write, diff, and revert behavior.
- [ ] Confirm Git mutation controls remain outside Explorer.
- [ ] Run backend and frontend full test suites.
- [ ] Run production build and compare initial and Explorer chunks.
- [ ] Record desktop, tablet, and mobile browser verification.
