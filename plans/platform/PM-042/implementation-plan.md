# Implementation Plan: PM-042 - Session Focus

## Overview

Add session-changes and cross-workspace session listing to the backend, then three frontend surfaces that read them:
Focus mode on the Canvas, Sessions and Changes tabs in the Workbench, and a Sessions page. No new entity, node kind,
or launch path.

## Execution Flow

```text
B1 Git since-commit readers ─> B2 Session changes + global list + API ─┬─> F2 Workbench tabs ──┐
F1 Types, client, Focus mode ──────────────────────────────────────────┤                        ├─> F4 Focus hide + layout ─┬─> I1 Journey + docs
                                                                       └─> F3 Sessions page ───┴─> F5 Session browser ─────┘
```

Critical path is B1 -> B2 -> F2 -> F5 -> I1. F1 depends only on the design and starts with B1. F2 and F3 both need B2
live and can run in parallel. F4 and F5 were added after the first live trial: dimming did not declutter the board,
and a session list without the live terminal was not worth opening. They are independent and run in parallel. There
is no external gate.

## Terminology Lock

All code, fields, API params, and TS types must use:

- `session` (never lane, attempt, run)
- `launchCommit` in API and UI, backed by the record's `observedCommit`
- `SessionChanges`, `SessionFile`, `GlobalSessionRow`, `SessionScope`
- `focus` for the Canvas toggle (`focusActive`, `.canvas-node-dimmed`)
- `scope=active|all`

> Lock this in before writing any code.

## Phases Summary

| Phase | Name                                         | Track      | Status      |
|-------|----------------------------------------------|------------|-------------|
| B1    | Git since-commit readers                     | Backend    | Complete    |
| B2    | Session changes, global list, and API        | Backend    | Complete    |
| F1    | Types, API client, and Focus mode            | Frontend   | Complete    |
| F2    | Workbench Sessions and Changes tabs          | Frontend   | Complete    |
| F3    | Sessions page                                | Frontend   | Complete    |
| F4    | Focus hides and lays out the active set      | Frontend   | Pending     |
| F5    | Session browser with live terminal           | Frontend   | Pending     |
| I1    | Browser journey, docs, and final integration | Full stack | In progress |

## Backend Phases

### Phase B1: Git Since-Commit Readers

**Deliverables:**

- [x] `NameStatusSince`, `ShortStatSince`, `DiffFileSince`, `RevListCount`, `CommitExists` on `GitAdapter` with
  context variants, ref validation, `--` separators, and the existing byte bound on diffs.
- [x] Tests on a temporary repository: committed, staged, unstaged, untracked, rename, missing commit, bad ref.

**Verification:** `go test ./internal/git`

**Commit:** `PM-042: Read checkout changes since a commit`

---

### Phase B2: Session Changes, Global List, And API

**Deliverables:**

- [x] `ai.Service.SessionChanges` and `SessionFileDiff` with the availability rules from the design.
- [x] `ai.Service.SessionRecordsAcrossWorkspaces` with scope, per-workspace authorization, annotations, and the row cap.
- [x] `internal/server/api/sessions.go`: `GET /api/sessions`, `GET /api/sessions/{id}/changes`,
  `GET /api/sessions/{id}/diff`; route inventory updated.
- [x] Tests: changes for each availability case; global list hides denied workspaces; scope filtering; path validation.

**Verification:** `go test ./internal/ai ./internal/server/...`

**Commit:** `PM-042: Serve session changes and a cross-workspace session list`

---

## Frontend Phases

### Phase F1: Types, API Client, And Focus Mode

**Deliverables:**

- [x] Types and client methods from the frontend design.
- [x] Focus toggle in the Canvas toolbar, active-set computation memoised on the projection, `.canvas-node-dimmed`
  styling, edge filtering, persisted preference, empty label.
- [x] Tests: active set with live and non-live sessions; dimmed nodes keep positions; preference round trip.

**Verification:** `npm run typecheck && npm test -- --run canvas`

**Commit:** `PM-042: Add Focus mode to the Canvas`

---

### Phase F2: Workbench Sessions And Changes Tabs

**Deliverables:**

- [x] `SessionsTable` and `SessionChangesPanel` components; `useSessionChanges` hook.
- [x] Workspace node: Info and Sessions tabs; session node: Terminal and Changes tabs; Workbench opens for a session
  node.
- [x] Open, Stop, and Changes actions wired through the page.
- [x] Tests: table renders states and actions; Changes panel renders each availability case and a file diff; Stop
  confirms before cancelling.

**Verification:** `npm run typecheck && npm test -- --run canvas`

**Commit:** `PM-042: List sessions and their changes in the Workbench`

---

### Phase F3: Sessions Page

**Deliverables:**

- [x] `features/sessions/SessionsPage` with scope, filters, `SessionsTable`, changes drawer, empty state.
- [x] `useGlobalSessions` polling only while any row is live.
- [x] Route `sessions` and left-nav entry in `App.tsx`; Open navigates to the right workspace and node.
- [x] Tests: scope toggle changes the query; Open navigates; polling stops when nothing is live.

**Verification:** `npm run typecheck && npm test -- --run sessions`

**Commit:** `PM-042: Add the cross-workspace Sessions page`

---

### Phase F4: Focus Hides And Lays Out The Active Set

**Deliverables:**

- [ ] Focus removes inactive nodes, sections, and edges from the rendered graph instead of dimming them.
- [ ] Virtual focus layout: active plans in a left column, each plan's sessions expanded in a row to its right, rows
  spaced by the tallest node; positions override only while Focus is on; nodes not draggable in Focus; saved layout
  untouched.
- [ ] Fit the viewport to the focus layout on toggle on; restore the saved viewport on toggle off.
- [ ] Remove `.canvas-node-dimmed` and the dimmed data flag.
- [ ] Tests: inactive nodes absent from the graph; layout positions per the rule; toggling off restores saved
  positions; nodes not draggable in Focus.

**Verification:** `npm run typecheck && npm test -- --run canvas`

**Commit:** `PM-042: Make Focus hide the inactive board and lay out the active set`

---

### Phase F5: Session Browser With Live Terminal

**Deliverables:**

- [ ] `features/sessions/SessionBrowser.tsx`: list of grouped cards with state glyphs, text filter, Live-only toggle,
  keyboard selection, detail pane with Terminal and Changes sub-tabs and Stop.
- [ ] Workbench Sessions tab renders the browser and widens the panel to a browser width while active.
- [ ] Sessions page renders the browser at page width grouped by workspace then plan.
- [ ] Retire `SessionsTable` in both features and the Sessions page drawer; keep `SessionChangesPanel` as the shared
  Changes view (one copy, under `features/sessions/`).
- [ ] Tests: grouping and glyphs per state; filter and Live-only; keyboard selection; terminal attaches only for the
  selected session; Stop confirms; Workbench panel widens on the tab; page renders the browser.

**Verification:** `npm run typecheck && npm test -- --run`

**Commit:** `PM-042: Browse sessions with a live terminal in the Workbench and the Sessions page`

---

## Integration Phase

### Phase I1: Browser Journey, Docs, And Final Integration

**Deliverables:**

- [ ] Run [Playbook 1](automation/scenario-01-session-focus-journey.md) against a local build; record the result.
      Not run in this phase — the orchestrator runs the browser journey against a live app and records the result in
      `automation/results/latest.md`.
- [x] `docs/domain/terminal/session-focus.md`: what changes are, what they are not, and the availability rules.
- [x] README section under Canvas; PM-037 roadmap row updated.
- [x] `make verify` green.

**Verification:** `make verify`

**Commit:** `PM-042: Document session focus and record the browser journey`

---

## Post-Implementation Checklist

- [ ] Update `plans/platform/PM-042/` docs to reflect any naming changes
- [ ] Verify no terminology drift: `grep -rn "lane" --include="*.go" --include="*.ts" --include="*.tsx" internal web/src`
- [ ] Run `wiki-enrich` for the journey runbook since `plan.e2e-runbook` is true

## Testing Strategy

- Backend: temp-repository tests for the Git readers; service tests with fake registry and repository; handler tests
  for authorization and validation.
- Frontend: component tests for the table and changes panel; hook tests with fake timers for polling; page test for
  navigation.
- Journey: the playbook covers Focus, the Sessions tab, Changes with a real edit, and Stop from the Sessions page.
