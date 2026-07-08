# Implementation Plan: PM-025 - Jira-First Workstream

## Overview

Establish Workstream as the board, intake, branch-context, and AI planning surface. PM-025 adds Jira-first new item intake, scaffolds Jira-backed item context, separates Workstream backend responsibilities from registered workspace ownership, and lets users start AI planning with a preset or free prompt after item creation.

## Terminology Lock

All new user-facing route and page text must use:

- `Workstream` for the main board/intake surface.
- `Workspace` for the sidebar section and registered repository contexts.
- `Board View` for the status-column view inside Workstream.
- `New Work Item` for the creation entry point.
- `Jira Intake` for pre-create Jira lookup.
- `AI Preset` for named prompt templates.

Use `/workstream` as the canonical surface route.

## Phases Summary

| Phase | Name                        | Status |
|-------|-----------------------------|--------|
| B1    | Workspace Jira Lookup       | Done   |
| B2    | Jira-Backed Item Creation   | Done   |
| B3    | AI Preset Launch Contract   | Done   |
| B4    | Workstream Branch API       | Done   |
| B5    | Workstream Backend Domain   | Done   |
| F1    | Workstream Route And Shell  | Done   |
| F2    | Jira Intake UI              | Done   |
| F3    | AI Preset Launch UI         | Done   |
| F4    | Workstream Verification     | Done   |
| F5    | Workstream Naming Alignment | Done   |
| F6    | Workstream Explorer         | Done   |

## Backend Phases

### Phase B1: Workspace Jira Lookup

**Deliverables:**

- [x] Add a workspace-scoped Jira issue lookup service method that accepts workspace ID and Jira key.
- [x] Add `GET /api/workspaces/{id}/jira/issues/{issueKey}`.
- [x] Reuse PM-019 validation, normalized issue state, cache, and recovery messages.
- [x] Add backend tests for success, missing config, invalid key, project mismatch, auth failure, forbidden, unavailable, and not found.

**Verification:** `rtk go test ./internal/jira ./internal/server/api`

**Commit:** `PM-025: Add workspace Jira issue lookup`

---

### Phase B2: Jira-Backed Item Creation

**Deliverables:**

- [x] Extend `NewItemInput` with optional Jira key and initial README context.
- [x] Update item writer to create README content when supplied instead of always writing an empty file.
- [x] Keep blank item creation behavior valid.
- [x] Add tests for Jira-backed README creation, duplicate item rejection, validation, and rescan behavior.

**Verification:** `rtk go test ./internal/item ./internal/server/api`

**Commit:** `PM-025: Add Jira-backed item creation`

---

### Phase B3: AI Preset Launch Contract

**Deliverables:**

- [x] Add built-in AI planning presets for implementation plan, technical design, and test scenarios.
- [x] Add `GET /api/ai/presets`.
- [x] Allow AI launch requests to include preset ID or free prompt while preserving existing context modes.
- [x] Add tests for preset lookup, prompt expansion, invalid preset handling, and existing launch behavior.

**Verification:** `rtk go test ./internal/ai ./internal/server/api`

**Commit:** `PM-025: Add AI planning presets`

### Phase B4: Workstream Branch API

**Deliverables:**

- [x] Add `POST /api/workspaces/{id}/workstream/branch`.
- [x] Use Workstream branch load terminology in the API handler and audit action.
- [x] Use `WorkstreamBranchLoadInput` and `WorkstreamBranchLoadResult` for the route contract.
- [x] Keep `/api/workspaces`, `internal/workspace`, `WorkspaceBranches`, and workspace file/tree APIs focused on registered repository workspaces.

**Verification:** `rtk go test ./internal/workspace ./internal/server/api && rtk npm run typecheck && rtk npm test -- --run`

---

### Phase B5: Workstream Backend Domain

**Deliverables:**

- [x] Add `internal/workstream` as the backend domain for branch-scoped board context.
- [x] Move branch snapshot loading, snapshot caching, working-tree rescan behavior, and selected branch persistence into the Workstream service.
- [x] Keep `internal/workspace` focused on registered repository lifecycle, scans, files, source settings, safety, and health.
- [x] Wire `internal/server/api` to delegate Workstream branch loads to the Workstream service.
- [x] Move branch-context tests into the Workstream package.

**Verification:** `rtk go test ./internal/workstream ./internal/workspace ./internal/server/api`

---

## Frontend Phases

### Phase F1: Workstream Route And Shell

**Deliverables:**

- [x] Use `WorkstreamPage` as the route owner and primary surface.
- [x] Make `/workstream` the canonical board and intake route.
- [x] Use Workstream in navigation, route labels, and page heading.
- [x] Update route tests and saved navigation assumptions.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

---

### Phase F2: Jira Intake UI

**Deliverables:**

- [x] Replace the New item modal with `New Work Item`.
- [x] Add Blank and From Jira modes.
- [x] Add Jira key fetch, preview, editable creation defaults, and failure states.
- [x] Create Jira-backed items through the extended API and open the created item.
- [x] Add focused component tests for lookup, success, failures, and no-write failure behavior.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-025: Add Jira-first work item intake`

---

### Phase F3: AI Preset Launch UI

**Deliverables:**

- [x] Load built-in AI presets from the API.
- [x] Add preset and free prompt controls to the post-create launch path.
- [x] Pass selected preset or prompt into embedded and external AI launch requests.
- [x] Preserve existing workspace-only and card-context launch choices.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-025: Add AI preset launch controls`

---

### Phase F4: Workstream Verification

**Deliverables:**

- [x] Verify user-facing Workstream terminology across the primary surface.
- [x] Keep Board View terminology where the status-column layout is specifically meant.
- [x] Update README and architecture notes for route names and API contracts.
- [x] Run browser verification for Workstream load, Jira intake, blank item creation, and AI launch dialog. In-app browser was unavailable; HTTP smoke verified the active surface route and `/api/ai/presets`.

**Verification:** `rtk npm run build && rtk go test ./...`

---

### Phase F5: Workstream Naming Alignment

**Deliverables:**

- [x] Use the Workstream label for the board/intake navigation item and page heading.
- [x] Keep `/workstream` as the canonical board route.
- [x] Use `features/workstream` for Workstream feature helpers.
- [x] Use `WorkstreamPage` for the main page component.
- [x] Update PM-025 docs to distinguish Workstream from registered Workspaces.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk go test ./internal/server`

---

### Phase F6: Workstream Explorer

**Deliverables:**

- [x] Use `WorkstreamExplorer` for the global file/tree surface.
- [x] Use `features/workstream-explorer` for explorer feature helpers.
- [x] Keep workspace branch/path domain helpers named Workspace where they refer to registered repositories.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run && rtk npm run build`

## Post-Implementation Checklist

- [x] Update `plans/platform/PM-025/` with implementation decisions.
- [x] Confirm Jira data is not persisted outside intended README context and existing metadata.
- [x] Confirm `/workstream` is the canonical surface route.
- [x] Confirm Workstream backend behavior is owned by `internal/workstream`.
- [x] Confirm PM-021 Jira mutation scope remains untouched.
