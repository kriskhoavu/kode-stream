# Implementation Plan: PM-025 - Jira-First Workspace

## Overview

Rename the Kanban surface to Workspace, add Jira-first new item intake, scaffold Jira-backed item context, and let users start AI planning with a preset or free prompt after item creation.

## Terminology Lock

All new user-facing route and page text must use:

- `Workspace` for the main board/intake surface.
- `Board View` for the Kanban-style view inside Workspace.
- `New Work Item` for the creation entry point.
- `Jira Intake` for pre-create Jira lookup.
- `AI Preset` for named prompt templates.

Do not keep `/kanban` route compatibility.

## Phases Summary

| Phase | Name                          | Status |
|-------|-------------------------------|--------|
| B1    | Workspace Jira Lookup         | Done   |
| B2    | Jira-Backed Item Creation     | Done   |
| B3    | AI Preset Launch Contract     | Done   |
| F1    | Workspace Route Rename        | Done   |
| F2    | Jira Intake UI                | Done   |
| F3    | AI Preset Launch UI           | Done   |
| F4    | Verification And Copy Cleanup | Done   |

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

## Frontend Phases

### Phase F1: Workspace Route Rename

**Deliverables:**

- [x] Rename `KanbanPage` surface and visible labels to Workspace.
- [x] Make `/workspace` the canonical board route.
- [x] Remove `/kanban` route handling, route labels, and navigation copy.
- [x] Update route tests and saved navigation assumptions.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-025: Rename Kanban surface to Workspace`

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

### Phase F4: Verification And Copy Cleanup

**Deliverables:**

- [x] Remove remaining user-facing Kanban page identity where it refers to the old surface.
- [x] Keep Board View terminology where the Kanban-style layout is specifically meant.
- [x] Update README or architecture notes if route names or API contracts changed.
- [x] Run browser verification for Workspace load, Jira intake, blank item creation, and AI launch dialog. In-app browser was unavailable; HTTP smoke verified `/workspace` and `/api/ai/presets`.

**Verification:** `rtk npm run build && rtk go test ./...`

**Commit:** `PM-025: Verify Jira-first Workspace`

## Post-Implementation Checklist

- [x] Update `plans/platform/PM-025/` with implementation decisions.
- [x] Confirm Jira data is not persisted outside intended README context and existing metadata.
- [x] Confirm `/kanban` is removed from client routing without preserving old focused-item state.
- [x] Confirm PM-021 Jira mutation scope remains untouched.
