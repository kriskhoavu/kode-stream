# Implementation Plan: PM-024 - Import Existing Workspaces

## Overview

Implement a review-first import from a current-schema `workspaces.yaml` into Plan Manager's effective OS-specific registry. Registration is atomic; indexing runs per imported workspace and reports partial failures.

## Phases Summary

| Phase | Name                                       | Status   |
|-------|--------------------------------------------|----------|
| B1    | Import contracts and strict preview        | Complete |
| B2    | Atomic registration and scan orchestration | Pending  |
| F1    | Client contracts and dialog state          | Pending  |
| F2    | Review and result experience               | Pending  |
| V1    | Cross-track verification and documentation | Pending  |

## Backend Phases

### Phase B1: Import Contracts and Strict Preview

**Deliverables:**

- [x] Add `existing_workspace` registration mode with non-managed deletion semantics.
- [x] Add import preview, candidate, issue, summary, request, and result models.
- [x] Add bounded strict current-schema YAML parsing.
- [x] Reuse workspace, Git, source, Jira, and Knowledge validation per candidate.
- [x] Detect duplicates inside the source and against the effective registry.
- [x] Add preview HTTP endpoint and effective `registryFile` system path.
- [x] Add native YAML file selection endpoint.
- [x] Cover parser, validation, API, and OS-path contracts with focused tests.

**Verification:** `rtk go test ./internal/system ./internal/workspace/... ./internal/server/api`

**Commit:** `PM-024: Add workspace import preview`

---

### Phase B2: Atomic Registration and Scan Orchestration

**Deliverables:**

- [ ] Add registry batch-create with locked duplicate recheck.
- [ ] Replace registry files atomically with mode `0600`.
- [ ] Reread source and match selected candidate keys during import.
- [ ] Persist destination-generated identity and `existing_workspace` ownership.
- [ ] Scan every imported workspace and continue after individual failures.
- [ ] Return indexed, scan-failed, skipped, and failed outcomes per candidate.
- [ ] Record import and scan audit outcomes without leaking local secrets.
- [ ] Cover write failure, changed source, concurrent duplicate, and mixed scan results.

**Verification:** `rtk go test ./internal/workspace/... ./internal/item/... ./internal/server/api`

**Commit:** `PM-024: Import and index existing workspaces`

## Frontend Phases

### Phase F1: Client Contracts and Dialog State

**Deliverables:**

- [ ] Add typed preview and import API methods and normalizers.
- [ ] Add Existing Workspaces to the Add Workspace mode control.
- [ ] Add file selection, manual path entry, preview loading, and file-level errors.
- [ ] Model selecting, previewing, reviewing, importing, complete, and error states.
- [ ] Derive default selection from selectable candidates only.
- [ ] Clear stale preview state when the source path changes.
- [ ] Cover API payloads, state transitions, picker cancellation, and retry behavior.

**Verification:** `rtk npm test -- --run web/src/shared/api/index.test.ts web/src/pages/WorkspacesPage.test.ts && rtk npm run build`

**Commit:** `PM-024: Add existing workspace import state`

---

### Phase F2: Review and Result Experience

**Deliverables:**

- [ ] Show source and backend-resolved destination paths.
- [ ] Show candidate configuration, status, and field-level issues.
- [ ] Add per-candidate and selectable-only bulk selection.
- [ ] Require explicit confirmation before import.
- [ ] Show indexed, scan-failed, skipped, and failed result states.
- [ ] Refresh app data after successful registrations.
- [ ] Add accessible focus, busy, alert, keyboard, and responsive behavior.
- [ ] Cover mixed candidate and result rendering with component tests.

**Verification:** `rtk npm test -- --run web/src/features/workspaces web/src/pages/WorkspacesPage.test.ts && rtk npm run build`

**Commit:** `PM-024: Add workspace import review experience`

## Verification Phase

### Phase V1: Cross-Track Verification and Documentation

**Deliverables:**

- [ ] Verify macOS, Linux, Windows, environment override, and bootstrap destination resolution in automated tests.
- [ ] Verify preview performs no writes and import writes only the effective registry and derived indexes.
- [ ] Verify imported workspace deletion never removes its directory.
- [ ] Verify strict rejection of removed legacy schema fields.
- [ ] Update README, architecture, API, and implementation baseline documentation.
- [ ] Run Markdown formatting and review every planning/documentation diff.

**Verification:** `rtk go test ./... && rtk npm test -- --run && rtk npm run build && rtk git diff --check`

**Commit:** `PM-024: Verify workspace import workflow`

## Testing Strategy

- Backend unit tests cover strict parsing, candidate validation, batch integrity, and scan continuation.
- Backend API tests cover request boundaries, effective destination disclosure, and mixed outcomes.
- Frontend unit and component tests cover state, selection, complete configuration review, and results.
- Full Go, Vitest, TypeScript, and Vite checks gate completion.
- A manual browser pass verifies native selection, focus, path wrapping, responsive layout, and mixed results.

## Implementation Constraints

- Do not restore any removed backward-compatibility parser or migration.
- Do not copy source IDs, timestamps, scan state, or clone ownership.
- Do not alter the selected source YAML.
- Do not clone, fetch, pull, or otherwise use the network.
- Do not let scan failure roll back a valid registration.
- Do not construct the destination registry path in frontend code.
- Complete and commit one phase before starting the next.
