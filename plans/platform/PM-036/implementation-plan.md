# Implementation Plan: PM-036 - E2E Quality Panels

## Overview

Implement safe E2E runbook discovery and result parsing, workspace-context AI launch, and a shared E2E Quality panel for plans and E2E Knowledge pages.

## Terminology Lock

All code, fields, API params, and TS types must use:

- `E2ERunbook` for the read model
- `E2EQualityPanel` for the shared UI
- `e2e-testing` for the required AI capability
- `latestResult` for parsed Markdown outcome data

> Lock this in before writing any code.

## Backend Phases

## Phase Summary

| Phase | Name                                    | Status  |
|-------|-----------------------------------------|---------|
| B1    | E2E runbook read model and APIs         | Pending |
| B2    | Workspace-context AI session launch     | Pending |
| F1    | Types, API client, and shared E2E panel | Pending |
| F2    | Plan and Knowledge integration          | Pending |
| F3    | Tests and visual polish                 | Pending |

## Backend Phases

### Phase B1: E2E Runbook Read Model and APIs

**Deliverables:**

- [ ] Add E2E runbook and latest-result models without persistence.
- [ ] Discover eligible plan runbooks and source-reference-matched canonical pages.
- [ ] Parse safe `automation/results/latest.md` files and expose plan/Knowledge read routes.
- [ ] Add focused service and API tests for ordering, missing results, malformed content, and unsafe paths.

**Verification:** `go test ./internal/item ./internal/knowledge ./internal/server/api`

**Commit:** `PM-036: Add E2E runbook discovery`

---

### Phase B2: Workspace-Context AI Session Launch

**Deliverables:**

- [ ] Add validated workspace-context external and embedded session routes.
- [ ] Reuse provider capability discovery with workspace scope.
- [ ] Preserve all item-context launch behavior and tests.

**Verification:** `go test ./internal/ai ./internal/server/api`

**Commit:** `PM-036: Support workspace E2E sessions`

---

## Frontend Phases

### Phase F1: Types, API Client, and Shared Panel

**Deliverables:**

- [ ] Add E2E runbook and latest-result frontend types plus API client methods.
- [ ] Build `E2EQualityPanel` with cards, result metadata, diagnostics, and refresh.
- [ ] Add constrained E2E mode to the AI Session dialog/control.

**Verification:** `npm --prefix web run test -- --runInBand E2EQualityPanel AISessionLaunchDialog`

**Commit:** `PM-036: Add shared E2E quality panel`

---

### Phase F2: Plan and Knowledge Integration

**Deliverables:**

- [ ] Integrate the panel below the existing plan Quality controls.
- [ ] Integrate the panel into Knowledge Reader only for E2E pages.
- [ ] Connect workspace-context launch and result refresh behavior.

**Verification:** `npm --prefix web run test -- --runInBand ItemWorkspacePage KnowledgePage KnowledgeReader`

**Commit:** `PM-036: Surface E2E quality coverage`

---

### Phase F3: Tests and Visual Polish

**Deliverables:**

- [ ] Add ordering, no-coverage, not-run, blocked, and skill-unavailable test cases.
- [ ] Add dark/light visual regression coverage for the shared panel.
- [ ] Verify runtime verification and repository automation remain unchanged.

**Verification:** `npm --prefix web run test && npm --prefix web run build`

**Commit:** `PM-036: Verify E2E quality experience`

---

## Post-Implementation Checklist

- [ ] Update `plans/platform/PM-036/` docs to reflect any naming changes
- [ ] Run `e2e-testing` against the final local playbook when runtime inputs are available.
- [ ] Run `wiki-enrich PM-036` and verify the canonical E2E journey includes this plan’s `sourceRef`.
- [ ] PR description references planning docs
