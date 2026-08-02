# Implementation Plan: PM-037 - Terminal Canvas Workspace Orchestrator

## Overview

Implement a workspace-scoped spatial Canvas that persists app-owned layouts, resolves current workspace/plan/session
state, and opens selected entities in a contextual Workbench. Local data-dir and SQLite support the full flow. Cloud
uses Postgres; Agent-Backed workspaces execute through the owner Agent, while Remote Snapshot workspaces stay
commit-pinned and read-only for repository/process actions.

## Terminology Lock

All code, fields, API parameters, tests, and UI labels use:

- `Canvas Document` for the persisted app-owned spatial document.
- `Canvas Node` and `Canvas Edge` for positioned references and typed relationships.
- `Entity Reference` for links to workspace, plan, session, or artifact entities.
- `Terminal Canvas` for the product surface.
- `Terminal Workbench` for the selected session's full terminal presentation.
- `Remote Snapshot` for Agentless Cloud workspaces; never use `remote workspace` as an execution promise.
- `Agent-Backed` for Cloud workspaces that route privileged actions through an owner Cloud Agent.

Do not use `canvas` to rename the existing xterm host class unless the code refers to the Terminal Canvas feature.

## Dependencies And Delivery Gates

| Dependency                         | Required behavior                                                                 | PM-037 handling                                                                            |
|------------------------------------|-----------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------|
| PM-020 / PM-027 embedded sessions  | Bounded PTY, grants, reconnect, cancellation, and reusable terminal presentation. | Reuse and refactor without changing secrecy or process limits.                             |
| PM-032 Agent command routing       | Owner Agent accepts and streams terminal, AI, Git, and verification actions.      | Capability remains unavailable until routing exists; do not execute on the Cloud host.     |
| PM-033 storage providers and sync  | Provider-backed app state in data-dir, SQLite, and Postgres.                      | Add Canvas repository to every provider and manual-sync snapshot.                          |
| PM-034 Remote Snapshot read models | Commit-pinned workspace and indexed plan reads without an Agent.                  | Show only currently resolvable snapshot entities; no fabricated plan/session state.        |
| PM-036 E2E quality surface         | Ticket-local playbooks can be enriched into a canonical reusable browser journey. | Update PM-037 playbook during implementation, then run `wiki-enrich` before final handoff. |

Local delivery is independently testable. Agent-Backed and complete Remote Snapshot acceptance must not be reported as
finished while their required access-adapter capabilities are unavailable.

## Phases Summary

| Phase | Name                                       | Track    | Status  |
|-------|--------------------------------------------|----------|---------|
| B1    | Canvas domain and repositories             | Backend  | Pending |
| B2    | Canvas service and API contracts           | Backend  | Pending |
| B3    | Resolution, capability, and session state  | Backend  | Pending |
| F1    | Canvas route, types, and document state    | Frontend | Pending |
| F2    | Infinite viewport and semantic nodes       | Frontend | Pending |
| F3    | Terminal and entity Workbench              | Frontend | Pending |
| F4    | Mode gating, accessibility, and resilience | Frontend | Pending |
| F5    | Browser journey and final integration      | Frontend | Pending |

## Backend Phases

### Phase B1: Canvas Domain And Repositories

**Deliverables:**

- [ ] Add `internal/canvas` document, node, edge, entity-reference, viewport, preference, and validation models.
- [ ] Define Canvas repository operations for list, resolve default, get, versioned update, create copy, and delete.
- [ ] Add bounded validation for ownership, kinds, IDs, references, groups, coordinates, zoom, counts, and payload size.
- [ ] Add file-backed Canvas repository using app data `canvases.yaml` and atomic guarded writes.
- [ ] Add SQL Canvas repository and migration version 2 for SQLite and Postgres.
- [ ] Extend `storage.RepositoryBundle`, provider composition, status fixtures, and cleanup with Canvas repository support.
- [ ] Extend manual storage sync and backups so Canvas documents round-trip between Local data-dir and SQLite.
- [ ] Add repository contract, migration, version-conflict, isolation, and sync tests.

**Verification:** `go test ./internal/canvas ./internal/storage`

**Commit:** `PM-037: Add Canvas domain and app-state repositories`

---

### Phase B2: Canvas Service And API Contracts

**Deliverables:**

- [ ] Add Canvas service ownership and workspace-access validation.
- [ ] Implement idempotent default Canvas resolution by owner, workspace, workspace scope, and `default` document key.
- [ ] Add list, resolve, get, versioned update, create-copy, and metadata-only delete handlers under `/api/canvases`.
- [ ] Register Gin routes through the existing transport boundary.
- [ ] Add stable bad-request, forbidden, not-found, size-limit, and `canvas_version_conflict` mappings.
- [ ] Audit Canvas mutations and blocked intents without serialized document, note content, prompts, or terminal data.
- [ ] Add API tests for Local identity, Cloud ownership isolation, roles, conflicts, validation, and deletion isolation.

**Verification:** `go test ./internal/canvas ./internal/server/api`

**Commit:** `PM-037: Add Canvas service and API`

---

### Phase B3: Resolution, Capability, And Session State

**Deliverables:**

- [ ] Resolve workspace and plan references in bounded batches from registered workspaces and current item indexes.
- [ ] Resolve Remote Snapshot references only at their selected immutable commit and mark incomplete read models clearly.
- [ ] Add safe active embedded-session listing by accessible workspace without channel grants, output, input, prompts, or arguments.
- [ ] Resolve session nodes against the existing in-memory manager and return explicit ended/stale states.
- [ ] Compose Canvas capabilities from runtime role, workspace access mode, editability, owner Agent availability, and action capability.
- [ ] Route Canvas-initiated execution into existing guarded services and workspace access adapters; never execute on Cloud host.
- [ ] Return recovery guidance for offline Agent, Remote Snapshot, missing workspace/plan/session, and role denial.
- [ ] Add Local, Agent-Backed online/offline, Remote Snapshot, stale-reference, and safe-projection tests.
- [ ] Confirm existing PM-020 session limit, lease, grant, cancellation, and shutdown tests remain unchanged.

**Verification:** `go test ./internal/ai ./internal/canvas ./internal/workspace ./internal/server/api`

**Commit:** `PM-037: Resolve Canvas entities and execution capabilities`

## Frontend Phases

### Phase F1: Canvas Route, Types, And Document State

**Deliverables:**

- [ ] Add Canvas API types and methods to the existing shared API layer.
- [ ] Add `/canvas` and `canvasId` routing with lazy-loaded `CanvasPage`.
- [ ] Add the Canvas entry to Workspace navigation outside the Chrome extension surface.
- [ ] Implement resolve/load, local edits, debounced versioned save, retry, save status, and conflict pause hooks.
- [ ] Implement **Reload latest** and **Keep my layout as a copy** conflict recovery.
- [ ] Preserve active workspace selection and route behavior when switching workspaces.
- [ ] Add router, API normalization, hook, ownership/error, and conflict tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/app web/src/features/canvas`

**Commit:** `PM-037: Add Canvas route and document state`

---

### Phase F2: Infinite Viewport And Semantic Nodes

**Deliverables:**

- [ ] Add React Flow viewport with pan, zoom, fit, selection, minimap threshold, grid preference, and bounded controls.
- [ ] Add workspace, plan, session, artifact, note, and group node renderers.
- [ ] Add typed relationship edges and valid source/target connection rules.
- [ ] Implement overview, standard, and detail semantic zoom levels.
- [ ] Add deterministic initial/reset layout and preview confirmation.
- [ ] Add **Unplaced work** behavior so scans do not rearrange saved manual layouts.
- [ ] Add search-to-focus, focus mode, remove-reference, and open-full-view actions.
- [ ] Memoize nodes and test model conversion at 100, 500, and 1,000 nodes.
- [ ] Add model, layout, node, edge, selection, and empty-state tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/canvas`

**Commit:** `PM-037: Add infinite Canvas and semantic nodes`

---

### Phase F3: Terminal And Entity Workbench

**Deliverables:**

- [ ] Extract a reusable `TerminalWorkbench` presentation from `EmbeddedTerminalDock` without duplicating session ownership.
- [ ] Preserve floating, side-panel, maximized, minimized, resize, reconnect, cancellation, and focus behaviors.
- [ ] Add Canvas Workbench shell with right, bottom, and narrow-viewport overlay presentations.
- [ ] Compose existing plan Markdown, files, metadata, diff, Jira, Git, and verification features for selected plan nodes.
- [ ] Launch workspace or plan AI sessions through existing composer controls and add the resulting session node relationship.
- [ ] Keep active terminal transports alive while selecting other Canvas nodes.
- [ ] Add stale/ended session recovery and remove-reference actions.
- [ ] Add tests proving one session has one active terminal host and moving between dock/Workbench does not relaunch.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/ai-session web/src/features/canvas web/src/pages/ItemWorkspacePage.test.ts`

**Commit:** `PM-037: Add Canvas Terminal Workbench`

---

### Phase F4: Mode Gating, Accessibility, And Resilience

**Deliverables:**

- [ ] Drive every node and Workbench action from effective capabilities rather than runtime-name checks.
- [ ] Add Local data-dir/SQLite labels only where storage context helps recovery, not as normal workflow noise.
- [ ] Add Agent-Backed online/offline execution labels and recovery guidance.
- [ ] Add Remote Snapshot ref/commit labels, read-only Workbench state, and Agent/local handoff guidance.
- [ ] Add accessible node names, keyboard search/navigation, visible toolbar controls, focus return, and live status regions.
- [ ] Add reduced-motion behavior, non-color state cues, both-theme contrast, and responsive Workbench layouts.
- [ ] Retain unsaved edits across transient API failures and warn only when navigation risks losing them.
- [ ] Add role, Agent state, Remote Snapshot, keyboard, reduced-motion, responsive, and retry tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/canvas web/src/pages/CanvasPage.test.tsx`

**Commit:** `PM-037: Complete Canvas modes and accessible UX`

---

### Phase F5: Browser Journey And Final Integration

**Deliverables:**

- [ ] Update `automation/scenario-01-orchestrate-plan-terminal.md` to match implemented visible labels and states.
- [ ] Run the Local browser playbook in a fresh Playwright MCP context when runtime inputs are supplied.
- [ ] Exercise Agent-Backed and Remote Snapshot sections only when suitable Cloud runtime inputs exist.
- [ ] Record passed, failed, blocked, and skipped results in `automation/results/latest.md` with evidence references.
- [ ] Run `wiki-enrich` so durable reusable Canvas coverage is synthesized under `wiki/e2e-testing/`.
- [ ] Verify canonical E2E coverage describes Local, offline Agent, and Agentless capability boundaries.
- [ ] Update architecture, storage, Cloud mode, and user-facing README documentation with actual delivered behavior.
- [ ] Run full backend, frontend, production build, formatting, and plan consistency checks.

**Verification:** `go test ./... && npm run build && npm test -- --run`

**Commit:** `PM-037: Verify and document Terminal Canvas`

## Post-Implementation Checklist

- [ ] Every phase is committed separately with its listed PM-037 commit subject.
- [ ] Canvas content remains app-owned and outside registered repositories.
- [ ] Local `datadir` and Local SQLite behavior match, including manual storage sync.
- [ ] Cloud uses Postgres; no Cloud data-dir behavior is introduced.
- [ ] Agent-Backed commands execute only through the owner Agent and never on the Cloud host.
- [ ] Remote Snapshot has no terminal, AI, Git mutation, file mutation, runtime, or verification execution.
- [ ] Terminal grants, bytes, prompts, arguments, credentials, repository content, and diffs are absent from Canvas storage/logs.
- [ ] Canvas delete and node removal do not delete source entities.
- [ ] Version conflicts never silently overwrite a newer Canvas document.
- [ ] Existing Workstream, Item Workspace, Knowledge Graph, external terminal, and embedded dock workflows still pass.
- [ ] `plan.e2e-runbook` remains true and durable wiki journey coverage is verified before handoff.
- [ ] Planning documents contain no stale terminology, endpoint, type, or file references.
