# Implementation Plan: PM-015 - Implementation Performance And Render Architecture Review

## Overview

Review and improve the current backend and frontend implementation in behavior-preserving phases. PM-015 targets performance, render mechanics, code conventions, design-pattern based decomposition, and future enhancement seams.

## Terminology Lock

All new code and docs should use:

- `Scan Pipeline`
- `Refresh Policy`
- `State Snapshot`
- `Render Adapter`
- `Feature Controller`
- `Provider Adapter`

Avoid:

- `big refactor`
- `cleanup only`
- `page logic`
- `magic cache`

## Phases Summary

| Phase | Name                                       | Status |
|-------|--------------------------------------------|--------|
| A1    | Baseline Review And Characterization       | Draft  |
| B1    | State Snapshot And Index Hot Paths         | Draft  |
| B2    | Scanner Pipeline And Git Metadata Provider | Draft  |
| B3    | Workspace File Refresh Policy              | Draft  |
| B4    | API Resource Handler Split                 | Draft  |
| F1    | Content Viewer Render Adapters             | Draft  |
| F2    | Explorer Controller Split                  | Draft  |
| F3    | Kanban And Item Workspace Controllers      | Draft  |
| D1    | Conventions And Architecture Finalization  | Draft  |

## Phase A1: Baseline Review And Characterization

**Deliverables:**

- [ ] Record current largest backend and frontend files and ownership risks.
- [ ] Capture baseline commands for backend tests, frontend tests, typecheck, and build.
- [ ] Add missing characterization tests before behavior-preserving refactors.
- [ ] Capture simple timing notes for `/api/state`, workspace scan, content search, and rich preview rendering on a representative local workspace.
- [ ] Update PM-015 docs with any corrected findings from the baseline.

**Verification:** `rtk go test ./... && rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Commit:** `PM-015: Capture implementation performance baseline`

---

## Backend Phases

### Phase B1: State Snapshot And Index Hot Paths

**Deliverables:**

- [ ] Add item index helpers for lightweight state metadata: item count, latest update, branch scan metadata, and revision source.
- [ ] Change `workspace.Service.State` to use the lightweight snapshot instead of marshaling all workspaces and items on every poll.
- [ ] Keep `/api/state` response fields and version semantics stable.
- [ ] Add tests proving state version changes after workspace create/update/delete, scan, metadata write, status write, source structure save/reset, and branch load.
- [ ] Review `itemindex.Get` and query hot paths for low-risk map/index helpers where behavior stays stable.

**Verification:** `rtk go test ./internal/itemindex ./internal/application/workspace ./internal/api && rtk go test ./...`

**Commit:** `PM-015: Add lightweight state snapshot`

---

### Phase B2: Scanner Pipeline And Git Metadata Provider

**Deliverables:**

- [ ] Split scanner internals into explicit stages behind `Scanner.Scan` and `Scanner.ScanWithRequest`.
- [ ] Add a `MetadataProvider` adapter for author/update timestamps with current per-item Git calls as fallback.
- [ ] Add a batch Git metadata implementation when available from existing Git commands.
- [ ] Keep `SourceReader`, branch snapshot, source settings, and PM-014 proposal behavior unchanged.
- [ ] Add focused tests for pipeline stage parity and metadata fallback.

**Verification:** `rtk go test ./internal/scanner ./internal/gitadapter ./internal/application/workspace && rtk go test ./...`

**Commit:** `PM-015: Introduce scanner pipeline metadata providers`

---

### Phase B3: Workspace File Refresh Policy

**Deliverables:**

- [ ] Add a refresh policy object for workspace file save, create, rename, revert, and future mutation paths.
- [ ] Move source path matching out of inline mutation methods.
- [ ] Preserve audit event timing and success/blocked/failed status behavior.
- [ ] Keep full workspace refresh fallback when a mutation crosses source boundaries or ownership is unclear.
- [ ] Add tests for no-refresh, source-refresh, and full-refresh decisions.

**Verification:** `rtk go test ./internal/application/workspacefiles ./internal/workspacefiles ./internal/api && rtk go test ./...`

**Commit:** `PM-015: Add workspace file refresh policy`

---

### Phase B4: API Resource Handler Split

**Deliverables:**

- [ ] Split `internal/api/api.go` handlers into resource files such as workspace, item, workspace files, git, search, navigation, and system.
- [ ] Keep route registration, HTTP methods, paths, status mapping, and response payloads unchanged.
- [ ] Keep shared helpers small and named by responsibility.
- [ ] Add or update tests around representative route groups.
- [ ] Update `ARCHITECTURE.md` dependency rules if handler ownership changes.

**Verification:** `rtk go test ./internal/api ./internal/application/... && rtk go test ./...`

**Commit:** `PM-015: Split API handlers by resource`

---

## Frontend Phases

### Phase F1: Content Viewer Render Adapters

**Deliverables:**

- [ ] Introduce renderer adapter contracts for Markdown, HTML, structured data, and source code.
- [ ] Cache Markdown output by file/content hash rather than raw content when possible.
- [ ] Cancel stale Markdown render promises and keep existing sanitization.
- [ ] Defer or chunk source highlighting for large files while preserving copy, wrap, and line-number controls.
- [ ] Add focused tests for renderer selection, large-file fallback, stale render cancellation, and cache bounds.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/features/content-viewer && rtk npm run build`

**Commit:** `PM-015: Add content viewer render adapters`

---

### Phase F2: Explorer Controller Split

**Deliverables:**

- [ ] Split `useWorkspaceExplorer` into tree cache, selection, Git state, branch refresh, and decoration controllers.
- [ ] Extract `ExplorerTree`, `ExplorerEditor`, search result, inspector, and path dialog components where it reduces page state coupling.
- [ ] Preserve route sync, keyboard behavior, source/all modes, and branch switch behavior.
- [ ] Add tests for cache invalidation, expand-to-path, source mode rows, and search selection.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/features/workspace-explorer web/src/pages/WorkspaceExplorerPage.test.tsx`

**Commit:** `PM-015: Split workspace explorer controllers`

---

### Phase F3: Kanban And Item Workspace Controllers

**Deliverables:**

- [ ] Extract Kanban branch loading, refresh, scan, pending status move, and saved-filter state into controller hooks.
- [ ] Extract Kanban board, drawer, branch picker, and new item dialog components behind stable props.
- [ ] Extract Item Workspace item/file loading, metadata draft, Git panel, content search, and layout resize state into focused hooks/components.
- [ ] Keep current class names and user-facing copy unless tests require a correction.
- [ ] Add focused tests for controller behavior and keep existing page tests passing.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/KanbanPage.test.tsx web/src/pages/ItemWorkspacePage.test.ts web/src/features/kanban web/src/features/file-editor && rtk npm run build`

**Commit:** `PM-015: Extract Kanban and item workspace controllers`

---

## Phase D1: Conventions And Architecture Finalization

**Deliverables:**

- [ ] Update `ARCHITECTURE.md` with final backend handler, scanner, refresh policy, frontend controller, and renderer ownership.
- [ ] Document code conventions for feature folders, hooks, adapters, error handling, tests, and performance-sensitive paths.
- [ ] Run Markdown formatting for PM-015 docs and any architecture docs touched.
- [ ] Run full backend and frontend verification.
- [ ] Update PM-015 plan docs with implementation outcomes and any deferred follow-up tickets.

**Verification:** `rtk go test ./... && rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Commit:** `PM-015: Finalize performance architecture conventions`

## Post-Implementation Checklist

- [ ] Existing API route contracts remain stable.
- [ ] Existing PM-013 branch snapshot and materialization behavior remains stable.
- [ ] Existing PM-014 Source Items proposal and reset behavior remains stable.
- [ ] Large Markdown/source preview no longer blocks common navigation and editing actions.
- [ ] `/api/state` polling cost is no longer proportional to the full item payload.
- [ ] New backend and frontend ownership rules are documented in `ARCHITECTURE.md`.
