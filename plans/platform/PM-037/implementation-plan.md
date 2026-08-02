# Implementation Plan: PM-037 - Focused Terminal Canvas

## Overview

Implement a polished Local Canvas MVP for the workspace -> plan -> session loop. Establish provider-neutral capabilities,
branch-aware references, placement persistence, durable safe session metadata, and verification freshness before adding
the viewport and Workbench. Every phase preserves existing terminal, Git, item, verification, and storage ownership.

The broader Canvas vision remains in the README roadmap. Groups, notes, artifacts, custom edges, multiple canvases,
Cloud Agent execution, Remote Snapshot UX, worktrees, and collaboration are not implementation deliverables for
PM-037.

## Phases Summary

| Phase | Name                                                  | Track      | Status   |
|-------|-------------------------------------------------------|------------|----------|
| B1    | Provider axes and action capabilities                 | Backend    | Complete |
| B2    | Layout, placement, and session repositories           | Backend    | Complete |
| B3    | Durable session lifecycle and branch-safe launch      | Backend    | Complete |
| B4    | Verification freshness and Canvas API                 | Backend    | Pending  |
| F1    | Canvas route and placement state                      | Frontend   | Pending  |
| F2    | Draggable semantic nodes and restored layout          | Frontend   | Pending  |
| F3    | Focused Workbench and session terminal                | Frontend   | Pending  |
| F4    | Git, verification, capability, and accessibility UX   | Frontend   | Pending  |
| I1    | Browser journey, documentation, and final integration | Full stack | Pending  |

## Backend Phases

### Phase B1: Provider Axes And Action Capabilities

**Deliverables:**

- [x] Define deployment-topology, workspace-content-provider, execution-provider, datastore, and authorization concerns
  without adding Canvas behavior keyed by deployment names.
- [x] Add action capability state, reason code, safe message, and recovery-action models.
- [x] Implement capability composition for `layout.move`, `repo.read`, `git.status`, `terminal.launch`, and
  `verification.run`.
- [x] Map current Local checkout and Local process behavior into content and execution provider adapters.
- [x] Keep current Cloud role and Remote Snapshot behavior working outside Canvas while exposing adapter-compatible
  support for future phases.
- [x] Revalidate capabilities within the existing guarded action services rather than trusting UI projections.
- [x] Add tests for available, unavailable, unsupported, forbidden, and branch-conflicted outcomes.

**Verification:** `go test ./internal/workspace ./internal/server/api ./internal/provider`

**Commit:** `PM-037: Add provider-aware action capabilities`

---

### Phase B2: Layout, Placement, And Session Repositories

**Deliverables:**

- [x] Add `internal/canvas` layout, placement, branch-aware entity-reference, validation, and repository contracts.
- [x] Implement one default layout for owner, workspace, and branch key.
- [x] Implement placement-level optimistic revisions and bounded batch updates.
- [x] Add durable safe session-record domain and repository contracts under the AI/session boundary.
- [x] Add data-dir repositories with guarded atomic writes for layouts, placements, and session records.
- [x] Add the next SQLite migration and repositories for normalized layouts, placements, and session records.
- [x] Extend `storage.RepositoryBundle`, provider composition, manual Local storage sync, backups, status fixtures, and
  cleanup.
- [x] Reject terminal data, repository content, cross-workspace references, invalid geometry, and invalid revisions.
- [x] Add repository contract, migration, sync, backup, isolation, limit, and conflict tests.

**Verification:** `go test ./internal/canvas ./internal/ai ./internal/storage`

**Commit:** `PM-037: Persist Canvas placements and session records`

---

### Phase B3: Durable Session Lifecycle And Branch-Safe Launch

**Deliverables:**

- [x] Split safe durable session lifecycle metadata from ephemeral PTY/process binding state.
- [x] Associate the current terminal manager binding with a session-record ID without persisting grants, bytes, prompts,
  arguments, buffers, environment variables, or credentials.
- [x] Add safe session-record listing by accessible workspace and branch.
- [x] Reconcile `starting` and `running` records without live bindings to `interrupted` during application startup.
- [x] Extend launch requests with expected workspace, plan branch, observed commit, and idempotency key.
- [x] Re-resolve the plan and current checkout immediately before process start.
- [x] Return `terminal_branch_mismatch` with expected/current branch and safe Git recovery context.
- [x] Ensure repeated submissions with one idempotency key return one record and start at most one process.
- [x] Preserve current PM-020 grant, lease, cancellation, session-limit, origin, and shutdown behavior.
- [x] Add matched branch, changed branch, dirty tree, stale plan, forbidden plan, launch failure, page reload, application
  restart, cancellation, placement removal, and sensitive-field tests.

**Verification:** `go test ./internal/ai ./internal/git ./internal/item ./internal/server/api`

**Commit:** `PM-037: Add durable branch-safe terminal sessions`

---

### Phase B4: Verification Freshness And Canvas API

**Deliverables:**

- [ ] Add deterministic repository fingerprinting for branch, HEAD, staged content, relevant tracked/untracked working
  content, and verification configuration.
- [ ] Capture start and completion fingerprints on verification jobs.
- [ ] Project `fresh`, `stale`, or `inconclusive` by comparing completed and current fingerprints.
- [ ] Keep verification jobs in memory for PM-037; do not introduce durable verification history.
- [ ] Add Canvas default resolve, get, placement patch, and viewport patch handlers.
- [ ] Resolve workspace, branch plan, durable session, live-binding, Git, and verification projections in bounded batches.
- [ ] Add deterministic initial placements plus unplaced projections for new plans and sessions.
- [ ] Render repository and application relationships as response-only derived connections.
- [ ] Return stale and forbidden references without deleting placements or leaking former titles.
- [ ] Add API tests for branch scoping, per-placement conflicts, viewport independence, new unplaced work, stale references,
  capability states, safe sessions, and verification freshness.

**Verification:** `go test ./internal/canvas ./internal/verification ./internal/git ./internal/server/api`

**Commit:** `PM-037: Resolve Canvas state and verification freshness`

## Frontend Phases

### Phase F1: Canvas Route And Placement State

**Deliverables:**

- [ ] Add Canvas, placement, branch-aware entity reference, action capability, safe session, and verification freshness
  API types.
- [ ] Add lazy-loaded `/canvas` routing for the active workspace and selected branch.
- [ ] Add **Canvas** to Workspace navigation outside the Chrome extension surface.
- [ ] Implement default resolve/load and branch-context switching.
- [ ] Implement optimistic node positions, dirty-node tracking, debounced placement patches, and bounded retry.
- [ ] Keep viewport saves independent from placement revisions.
- [ ] Implement affected-node conflict recovery through **Reload position** and explicit reapply to latest.
- [ ] Preserve dirty positions through transient failures and warn only when navigation risks losing them.
- [ ] Add router, API normalization, branch-change, placement-save, conflict, retry, and viewport tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/app web/src/features/canvas`

**Commit:** `PM-037: Add Canvas route and placement state`

---

### Phase F2: Draggable Semantic Nodes And Restored Layout

**Deliverables:**

- [ ] Add the React Flow viewport with visible pan, zoom, fit, selection, and reset controls.
- [ ] Add memoized workspace, plan, and session node renderers only.
- [ ] Make all three node kinds draggable and prove that moving the workspace does not move other nodes.
- [ ] Render repository and application connections without handles or edit/delete controls.
- [ ] Implement deterministic first placement, restored saved positions, **Unplaced work**, and **Place new items**.
- [ ] Implement reset-layout preview and confirmation.
- [ ] Add node search and focus by title, identifier, branch, and session state.
- [ ] Implement placement removal with explicit entity/process isolation messaging.
- [ ] Measure behavior at 25, 100, and 300 placements.
- [ ] Add movement, save, restore, unplaced, reset, removal, search, and render-limit tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/canvas`

**Commit:** `PM-037: Add draggable Canvas work nodes`

---

### Phase F3: Focused Workbench And Session Terminal

**Deliverables:**

- [ ] Add a focused Workbench shell with right-panel and narrow-window overlay presentations.
- [ ] Show workspace Git summary, plan actions, live session terminal, and ended/interrupted lifecycle detail.
- [ ] Reuse the existing xterm/session presentation without introducing a second process, subscriber, or terminal owner.
- [ ] Add plan launch with one idempotency key per pending user submission.
- [ ] Show expected/current branch and recovery actions for `terminal_branch_mismatch`.
- [ ] Add a new session to unplaced work or accept its deterministic position near the plan.
- [ ] Preserve the live process when selecting another node; follow existing reconnect rules on return.
- [ ] Show active unplaced sessions so hidden processes remain discoverable.
- [ ] Separate **Remove from Canvas** from **Cancel process** and retain confirmations.
- [ ] Add matching-branch, mismatch, double-submit, selection switch, reload, interruption, cancellation, and isolation tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/ai-session web/src/features/canvas`

**Commit:** `PM-037: Add branch-safe Canvas terminal Workbench`

---

### Phase F4: Git, Verification, Capability, And Accessibility UX

**Deliverables:**

- [ ] Show branch, HEAD summary, clean/dirty/conflicted state, and changed-file count on the workspace node and panel.
- [ ] Show verification result status separately from current/stale/inconclusive freshness.
- [ ] Remove current-success emphasis from stale passed results and show verified/current revision summaries.
- [ ] Refresh freshness after branch, commit, staged, unstaged, untracked, or verification configuration changes.
- [ ] Drive every action from action capability state and reason code, not deployment, access-mode, provider, or datastore
  names.
- [ ] Add safe stale-reference recovery and never display cached titles for forbidden entities.
- [ ] Add keyboard node search/selection/movement, accessible names, focus return, and polite status regions.
- [ ] Add non-color state cues, reduced-motion behavior, both-theme contrast, and narrow-window recovery.
- [ ] Add Git, freshness, capability, stale/forbidden, keyboard, reduced-motion, responsive, and retry tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/canvas web/src/pages/CanvasPage.test.tsx`

**Commit:** `PM-037: Complete Canvas status and accessible UX`

## Integration Phase

### Phase I1: Browser Journey, Documentation, And Final Integration

**Deliverables:**

- [ ] Update the PM-037 playbook to match implemented labels and selectors.
- [ ] Run the Local journey in a fresh Playwright MCP context when runtime inputs are supplied.
- [ ] Verify independent workspace, plan, and session movement plus restored layout.
- [ ] Verify matching-branch launch, double-submit protection, branch-mismatch rejection, and session interruption semantics.
- [ ] Verify Git state and passed-result staleness after a controlled repository mutation.
- [ ] Record passed, failed, blocked, and skipped steps in `automation/results/latest.md` with safe evidence references.
- [ ] Run `wiki-enrich` and verify the durable Canvas journey documents delivered Local behavior and future capability
  boundaries accurately.
- [ ] Update architecture, storage, terminal, verification, and user-facing README documentation with delivered behavior.
- [ ] Confirm deferred graph, Cloud Agent, and Remote Snapshot features are not described as implemented.
- [ ] Run full backend tests, frontend tests, production build, Markdown formatting, and plan consistency checks.

**Verification:** `go test ./... && npm run build && npm test -- --run`

**Commit:** `PM-037: Verify and document focused Terminal Canvas`

## Post-Implementation Checklist

- [ ] Every phase is verified and committed separately with its listed PM-037 subject.
- [ ] Canvas stores placements and presentation only, never copied entity state.
- [ ] Workspace, plan, and session nodes are all draggable; moving a workspace moves no other node.
- [ ] Placement conflicts are scoped to affected nodes and viewport saves are independent.
- [ ] Current branch is revalidated on the server immediately before terminal process start.
- [ ] Repeated launch submissions start at most one process.
- [ ] Durable session records contain no prompt, argument, environment, grant, output, input, buffer, credential, or file
  content.
- [ ] Application restart marks records without live bindings interrupted and never offers false reconnect.
- [ ] Verification becomes stale after relevant repository or configuration change and inconclusive after during-run
  change.
- [ ] Action behavior uses capability states and reason codes, not mode or datastore checks.
- [ ] Layout removal and reset never mutate workspaces, plans, sessions, processes, Git, or verification entities.
- [ ] Groups, notes, artifacts, custom edges, multiple canvases, Agent execution, snapshot UX, and collaboration remain
  deferred.
- [ ] Existing Workstream, Item Workspace, terminal dock, Git, and verification workflows still pass.
- [ ] `plan.e2e-runbook` remains true and durable wiki journey coverage is verified before final handoff.
