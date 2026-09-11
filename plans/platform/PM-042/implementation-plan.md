# Implementation Plan: PM-042 - Session Focus

## Overview

Add session-changes and cross-workspace session listing to the backend, then three frontend surfaces that read them:
Focus mode on the Canvas, Sessions and Changes tabs in the Workbench, and a Sessions page. No new entity, node kind,
or launch path.

## Execution Flow

```text
B1 Git since-commit readers ─> B2 Session changes + global list + API ─┬─> F2 Workbench tabs ──┐
F1 Types, client, Focus mode ──────────────────────────────────────────┤                        ├─> I1 Journey + docs
                                                                       └─> F3 Sessions page ───┘
```

Critical path is B1 -> B2 -> F2 -> I1. F1 depends only on the design and starts with B1. F2 and F3 both need B2 live
and can run in parallel. There is no external gate.

## Terminology Lock

All code, fields, API params, and TS types must use:

- `session` (never lane, attempt, run)
- `launchCommit` in API and UI, backed by the record's `observedCommit`
- `SessionChanges`, `SessionFile`, `GlobalSessionRow`, `SessionScope`
- `focus` for the Canvas toggle (`focusActive`, `.canvas-node-dimmed`)
- `scope=active|all`

> Lock this in before writing any code.

## Phases Summary

| Phase | Name                                         | Track      | Status   |
|-------|----------------------------------------------|------------|----------|
| B1    | Git since-commit readers                     | Backend    | Pending  |
| B2    | Session changes, global list, and API        | Backend    | Pending  |
| F1    | Types, API client, and Focus mode            | Frontend   | Complete |
| F2    | Workbench Sessions and Changes tabs          | Frontend   | Pending  |
| F3    | Sessions page                                | Frontend   | Pending  |
| I1    | Browser journey, docs, and final integration | Full stack | Pending  |

## Backend Phases

### Phase B1: Git Since-Commit Readers

**Deliverables:**

- [ ] `NameStatusSince`, `ShortStatSince`, `DiffFileSince`, `RevListCount`, `CommitExists` on `GitAdapter` with
  context variants, ref validation, `--` separators, and the existing byte bound on diffs.
- [ ] Tests on a temporary repository: committed, staged, unstaged, untracked, rename, missing commit, bad ref.

**Verification:** `go test ./internal/git`

**Commit:** `PM-042: Read checkout changes since a commit`

---

### Phase B2: Session Changes, Global List, And API

**Deliverables:**

- [ ] `ai.Service.SessionChanges` and `SessionFileDiff` with the availability rules from the design.
- [ ] `ai.Service.SessionRecordsAcrossWorkspaces` with scope, per-workspace authorization, annotations, and the row cap.
- [ ] `internal/server/api/sessions.go`: `GET /api/sessions`, `GET /api/sessions/{id}/changes`,
  `GET /api/sessions/{id}/diff`; route inventory updated.
- [ ] Tests: changes for each availability case; global list hides denied workspaces; scope filtering; path validation.

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

- [ ] `SessionsTable` and `SessionChangesPanel` components; `useSessionChanges` hook.
- [ ] Workspace node: Info and Sessions tabs; session node: Terminal and Changes tabs; Workbench opens for a session
  node.
- [ ] Open, Stop, and Changes actions wired through the page.
- [ ] Tests: table renders states and actions; Changes panel renders each availability case and a file diff; Stop
  confirms before cancelling.

**Verification:** `npm run typecheck && npm test -- --run canvas`

**Commit:** `PM-042: List sessions and their changes in the Workbench`

---

### Phase F3: Sessions Page

**Deliverables:**

- [ ] `features/sessions/SessionsPage` with scope, filters, `SessionsTable`, changes drawer, empty state.
- [ ] `useGlobalSessions` polling only while any row is live.
- [ ] Route `sessions` and left-nav entry in `App.tsx`; Open navigates to the right workspace and node.
- [ ] Tests: scope toggle changes the query; Open navigates; polling stops when nothing is live.

**Verification:** `npm run typecheck && npm test -- --run sessions`

**Commit:** `PM-042: Add the cross-workspace Sessions page`

---

## Integration Phase

### Phase I1: Browser Journey, Docs, And Final Integration

**Deliverables:**

- [ ] Run [Playbook 1](automation/scenario-01-session-focus-journey.md) against a local build; record the result.
- [ ] `docs/domain/terminal/session-focus.md`: what changes are, what they are not, and the availability rules.
- [ ] README section under Canvas; PM-037 roadmap row updated.
- [ ] `make verify` green.

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
