# Implementation Plan: PM-003 - Technical Architecture Refactoring

## Overview

Refactor Plan Manager architecture in small behavior-preserving phases. Each phase must keep current workflows, routes, API payloads, storage files, and UI unchanged.

## Phases Summary

| Phase | Name                                    | Status |
|-------|-----------------------------------------|--------|
| A1    | Characterization And Baseline           | Done   |
| B1    | Backend Application Services            | Done   |
| B2    | Scanner And Path Guard Separation       | Done   |
| B3    | Backend Performance Improvements        | Done   |
| F1    | Frontend App State And API Modules      |        |
| F2    | Frontend Feature Hooks And Components   |        |
| F3    | Frontend Styles And Render Performance  |        |
| D1    | Architecture Documentation Finalization |        |

## Phase A1: Characterization And Baseline

**Deliverables:**

- [x] Add backend characterization tests for key API response shapes and error statuses.
- [x] Add scanner fixture tests for structured sources, configured sources, freestyle docs, and item YAML precedence.
- [x] Add frontend helper tests for route parsing, filters, diff parsing, source parsing, and source settings inference.
- [x] Record current build, test, and bundle baseline.

**Verification:** `rtk go test ./... && rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Draft Commit:**
```text
PM-003: Add architecture refactor characterization tests

- Cover current backend API and scanner behavior
- Cover current frontend helper behavior
- Record build and test baseline before refactoring
```

---

## Phase B1: Backend Application Services

**Deliverables:**

- [x] Add workspace, item, and Git application services.
- [x] Move orchestration from API handlers into services.
- [x] Keep all existing HTTP routes and DTOs unchanged.
- [x] Add service tests with fake dependencies.
- [x] Keep `internal/app` wiring simple and explicit.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-003: Extract backend application services

- Move workspace, item, and Git orchestration out of HTTP handlers
- Keep API contracts unchanged
- Add service-level tests for core workflows
```

---

## Phase B2: Scanner And Path Guard Separation

**Deliverables:**

- [x] Split scanner traversal, source settings matching, metadata parsing, and item assembly behind the existing scanner facade.
- [x] Add `internal/security/pathguard` for safe joins, symlink checks, source scope checks, and selected Git path validation.
- [x] Replace duplicated path helpers one caller at a time.
- [x] Keep all existing scanner and write guard tests passing.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-003: Separate scanner internals and path guards

- Split scanner responsibilities behind the existing Scan facade
- Consolidate safe path validation
- Preserve current scan and write behavior
```

---

## Phase B3: Backend Performance Improvements

**Deliverables:**

- [x] Add targeted item refresh for metadata and status writes where safe.
- [x] Add scanner cache or skip logic for unchanged source roots.
- [x] Batch Git metadata lookup if scan tests show stable output.
- [x] Keep full workspace scan fallback for uncertain cases.
- [x] Add regression tests around cache invalidation.

**Implementation Note:** PM-003 keeps the full workspace scan as the correctness path. The performance change removes the duplicate post-write scan by reusing the scan data already produced during refresh, and it lists Git branches once per scan instead of once per item identifier.

**Verification:** `rtk go test ./...`

**Draft Commit:**
```text
PM-003: Improve backend scan performance

- Add targeted refresh for safe write paths
- Avoid repeated work for unchanged source roots
- Keep full scan fallback for correctness
```

---

## Phase F1: Frontend App State And API Modules

**Deliverables:**

- [ ] Move route parsing and path generation into `web/src/app/router.ts`.
- [ ] Move workspace, theme, refresh, and stale-state logic into app hooks.
- [ ] Split API client by resource while preserving the exported `api` facade.
- [ ] Add tests for app state helpers and API normalization.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-003: Extract frontend app state and API modules

- Move routing and stale-state logic out of App
- Split API client by resource behind the same facade
- Add focused frontend tests
```

---

## Phase F2: Frontend Feature Hooks And Components

**Deliverables:**

- [ ] Extract Kanban filters, drawer behavior, and card actions into hooks and components.
- [ ] Extract item workspace autosave, metadata, diff, file tree, and Git behavior into hooks and components.
- [ ] Extract workspace forms and source settings editor into smaller modules.
- [ ] Preserve rendered class names and markup structure unless tests prove no visual change.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Draft Commit:**
```text
PM-003: Split frontend feature modules

- Extract Kanban, item workspace, and workspace feature hooks
- Split large pages into focused components
- Preserve current UI behavior and class names
```

---

## Phase F3: Frontend Styles And Render Performance

**Deliverables:**

- [ ] Move CSS into app, shared, and feature-owned files without visual changes.
- [ ] Memoize expensive selectors and derived data where tests cover behavior.
- [ ] Lazy render hidden heavy panels where state behavior remains unchanged.
- [ ] Run screenshot or manual visual checks for key workflows.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Draft Commit:**
```text
PM-003: Organize frontend styles and render work

- Move styles to feature-owned files
- Reduce repeated derived data work
- Preserve existing UI appearance
```

---

## Phase D1: Architecture Documentation Finalization

**Deliverables:**

- [ ] Update top-level `ARCHITECTURE.md` to reflect the final package structure.
- [ ] Update PM-003 docs with any implementation decisions made during migration.
- [ ] Add developer notes for package dependency rules and frontend module ownership.
- [ ] Confirm all acceptance criteria are met.

**Verification:** `rtk go test ./... && rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

**Draft Commit:**
```text
PM-003: Finalize architecture documentation

- Update architecture docs for the refactored structure
- Document package ownership and dependency rules
- Confirm PM-003 acceptance criteria
```

## Migration Controls

- One phase per pull request or commit.
- No feature changes inside PM-003 phases.
- No visual changes without a separate ticket.
- Stop and add tests when a behavior difference is found.
- Prefer facade-preserving moves before package renames.
- Keep full scan fallback until targeted refresh has enough coverage.
