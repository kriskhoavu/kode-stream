# Implementation Plan: PM-013 - Kanban Branch Snapshot Materialization

## Overview

Implement single-branch Kanban loading with branch snapshots and safe materialization into the current checkout branch. The selected branch controls what the board shows. The current checkout branch controls where writes land.

## Terminology Lock

Use these names in new public code, API fields, and frontend types:

- `selectedBranch`
- `currentCheckoutBranch`
- `sourceMode`
- `working_tree`
- `snapshot`
- `SourceReader`
- `GitTreeSourceReader`
- `FilesystemSourceReader`
- `BranchScanMetadata`
- `ReplaceWorkspaceBranch`
- `materialize`

Avoid:

- `branchFilter` for selected branch state.
- `checkout` for Kanban branch selection.
- `merge` or `compare` for branch snapshot loading.
- `copyAllDocs` for freestyle docs materialization.

## Phases Summary

| Phase | Name                                 | Status |
|-------|--------------------------------------|--------|
| B1    | Source Readers And Git Tree Access   | ✅     |
| B2    | Branch-Aware Scanner And Index       | ✅     |
| B3    | Branch Load And Materialization APIs | ✅     |
| B4    | Write Safety Integration             | ✅     |
| F1    | Branch API Types And Client          | ✅     |
| F2    | Single-Branch Kanban State           | ✅     |
| F3    | Snapshot Materialization UX          | ✅     |
| F4    | Final Styling And Verification       | ✅     |

## Backend Phases

### Phase B1: Source Readers And Git Tree Access

**Deliverables:**

- [x] Add `scanner.SourceReader` plus minimal `DirEntry`, `FileInfo`, and `WalkFunc` adapters.
- [x] Add `FilesystemSourceReader` backed by current filesystem operations.
- [x] Add `GitTreeSourceReader` backed by read-only Git object access.
- [x] Extend `gitadapter.GitAdapter` with branch resolve, tree read, tree walk, and ref-specific author/update methods.
- [x] Validate branch names before building refs.
- [x] Add tests proving Git tree reads do not change `git branch --show-current`.

**Result:** Added read-only Git tree access, filesystem and Git tree source readers, and branch snapshot tests that verify the current checkout branch stays unchanged.

**Verification:** `rtk go test ./internal/gitadapter ./internal/scanner`

**Draft Commit:**

```text
PM-013: Add source readers for branch snapshots

Change summary:
- Add scanner source reader abstraction
- Add filesystem and Git tree reader implementations
- Add read-only Git object access tests
```

---

### Phase B2: Branch-Aware Scanner And Index

**Deliverables:**

- [x] Refactor scanner traversal, source settings, metadata parsing, document discovery, status inference, and file counts to use `SourceReader`.
- [x] Add `ScanRequest` with branch, branch ref, commit, source mode, and editable fields.
- [x] Stamp scanned items with branch ref, commit, source mode, and editable metadata.
- [x] Add `BranchScanMetadata` to index storage.
- [x] Add `ReplaceWorkspaceBranch`, branch metadata lookup, and branch item query helpers.
- [x] Migrate existing YAML items safely when branch scan metadata is absent.
- [x] Cover scanner parity between filesystem and Git tree reads for committed content.
- [x] Cover replacing one workspace branch without deleting another branch.

**Result:** Scanner reads now run through `SourceReader` and item summaries carry branch/source metadata. Index storage can replace and query one workspace branch while preserving other branch snapshots.

**Verification:** `rtk go test ./internal/scanner ./internal/itemindex`

**Draft Commit:**

```text
PM-013: Add branch-aware scanner and index

Change summary:
- Scan through SourceReader instead of direct filesystem calls
- Persist items and scan metadata by workspace branch
- Preserve other branch snapshots during refresh
```

---

### Phase B3: Branch Load And Materialization APIs

**Deliverables:**

- [x] Add branch load service that resolves selected branch, chooses source mode, checks cache/index, scans when needed, and returns branch items.
- [x] Persist last selected branch per workspace and apply startup default priority: last selected, baseline, checkout.
- [x] Add `POST /api/workspaces/{id}/kanban/branch`.
- [x] Add materialization service for snapshot writes.
- [x] Copy whole structured item directories on first snapshot edit.
- [x] Copy only the edited file for freestyle docs and unsorted docs, unless the docs path maps to a supported item directory.
- [x] Block materialization if any target file already exists.
- [x] Refresh the current checkout branch index after successful materialization.
- [x] Add API and service tests for branch load, no-checkout behavior, materialization, and conflicts.

**Result:** Added Kanban branch loading without checkout, branch selection persistence, snapshot item reads from Git objects, and confirmed materialization that blocks existing checkout files.

**Verification:** `rtk go test ./internal/application/workspace ./internal/application/item ./internal/api`

**Draft Commit:**

```text
PM-013: Add Kanban branch load and materialization APIs

Change summary:
- Load one selected branch without checkout
- Materialize snapshot edits into the current checkout branch
- Block existing-path conflicts before writing
```

---

### Phase B4: Write Safety Integration

**Deliverables:**

- [x] Route file save, metadata save, status update, and revert paths through branch/source-mode checks.
- [x] Require current checkout branch for direct writes.
- [x] Require materialization confirmation for snapshot writes.
- [x] Strengthen path validation for writes, deletes, restores, reverts, and commits.
- [x] Reject traversal, symlink escape, outside-source paths, and unsupported item paths.
- [x] Ensure no Kanban branch load path calls `git checkout` or `git switch`.
- [x] Add regression tests for forbidden Git commands and plan safety boundary errors.

**Result:** Item writes now require the indexed working-tree branch to match the current checkout branch, snapshot writes require explicit materialization confirmation, and materialized files are checked against configured sources before copying.

**Verification:** `rtk go test ./internal/application/item ./internal/application/git ./internal/workspacefiles ./internal/api && rtk go test ./...`

**Draft Commit:**

```text
PM-013: Enforce branch materialization safety

Change summary:
- Guard direct writes by current checkout branch
- Validate materialized target paths before writes
- Keep destructive Git operations outside Kode Stream ownership
```

## Frontend Phases

### Phase F1: Branch API Types And Client

**Deliverables:**

- [x] Add frontend types for `BranchLoadResult`, branch mode, branch scan metadata, and materialization-aware write options.
- [x] Add API client method for `POST /api/workspaces/{id}/kanban/branch`.
- [x] Extend existing item write calls with optional `materializeConfirmed`.
- [x] Normalize branch load response arrays and optional fields.
- [x] Add focused API client tests.

**Result:** Added branch load and source mode frontend types, materialization-aware write options, and a normalized `loadKanbanBranch` API client method with focused tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/shared/api`

**Draft Commit:**

```text
PM-013: Add Kanban branch API client

Change summary:
- Add branch load and source mode frontend types
- Add materialization-aware write options
- Normalize branch-scoped item responses
```

---

### Phase F2: Single-Branch Kanban State

**Deliverables:**

- [x] Replace branch multi-select filter state with `selectedBranch`.
- [x] Load Kanban items through the branch load endpoint.
- [x] Add single branch selector with working tree and snapshot labels.
- [x] Preserve non-branch filters for source, scope, status, author, and text.
- [x] Rename Scan to `Refresh` and pass `force=true`.
- [x] Migrate old saved filter branch arrays by using only the first branch.
- [x] Add tests for default selection, branch switching, refresh, and no `switchBranch` calls.

**Result:** Kanban now loads one selected branch through the branch-load endpoint, exposes a single branch selector with working-tree/snapshot context, keeps branch out of normal facet filters, and refreshes only the active branch.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/KanbanPage.test.tsx web/src/features/kanban`

**Draft Commit:**

```text
PM-013: Add single-branch Kanban state

Change summary:
- Replace branch facet with one selected branch
- Load board items from the selected branch endpoint
- Refresh only the active branch
```

---

### Phase F3: Snapshot Materialization UX

**Deliverables:**

- [x] Add first-edit confirmation for snapshot file saves, metadata saves, status menu moves, and drag/drop moves.
- [x] Explain whole-plan copy for structured items.
- [x] Explain one-file copy for freestyle docs and unsorted docs.
- [x] Pause optimistic drag/drop until confirmation succeeds.
- [x] Reconcile successful materialized writes with returned working-tree item.
- [x] Show conflict and load errors with clear messages.
- [x] Cover confirm, cancel, conflict, structured copy, docs copy, and drag/drop paths.

**Result:** Snapshot edits now require explicit confirmation before status, drag/drop, file, or metadata writes materialize content into the current checkout branch. Cancelled confirmations skip writes, conflicts surface backend errors, and successful writes reconcile from the returned working-tree item.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/KanbanPage.test.tsx web/src/pages/KanbanPage.drag.test.tsx`

**Draft Commit:**

```text
PM-013: Add snapshot edit materialization UX

Change summary:
- Confirm before copying snapshot content into checkout
- Materialize structured plans and docs edits through existing write flows
- Preserve drag and status rollback behavior
```

---

### Phase F4: Final Styling And Verification

**Deliverables:**

- [x] Style branch selector, branch mode labels, write target text, and materialization prompts.
- [x] Verify desktop and mobile layouts do not overlap.
- [x] Rebuild frontend assets.
- [x] Run backend and frontend regression suites.
- [x] Update PM-013 docs with final naming if implementation differs.

**Result:** Added scoped selector styling for the branch context chip, completed the embedded frontend rebuild, and verified the full frontend/backend regression suite after all PM-013 changes.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build && rtk go test ./...`

**Draft Commit:**

```text
PM-013: Finish branch snapshot materialization

Change summary:
- Polish branch selector and snapshot edit states
- Verify frontend and backend regressions
- Rebuild embedded frontend assets
```

## Post-Implementation Checklist

- [x] Branch selection never calls checkout or switch.
- [x] Board always shows exactly one selected branch.
- [x] Working tree mode writes directly to filesystem.
- [x] Snapshot structured edit copies the whole item directory first.
- [x] Snapshot freestyle docs edit copies only the edited file.
- [x] Existing target files block materialization.
- [x] Refreshing one branch preserves other branch indexed items.
- [x] All writes remain inside Kode Stream safety boundary.
