# Scenarios: PM-042 Session Focus

## Scenario List

| # | Title           | Description                                                                    |
|---|-----------------|--------------------------------------------------------------------------------|
| 0 | Busy Canvas     | Twenty plans, two agents running; the user cannot tell which plans are active. |
| 1 | Focus mode      | Toggle Focus; only plans with an active session stay bright.                   |
| 2 | Sessions tab    | Select the workspace node; see every session on this branch in one table.      |
| 3 | Session changes | Select a session node; the Changes tab lists files since launch with diffs.    |
| 4 | Sessions page   | Open Sessions in the left nav; see agents across all workspaces; stop one.     |
| 5 | Guard rails     | Restart, moved HEAD, missing workspace, cloud tenancy, non-local content.      |

## Scenario 0: Busy Canvas

### Starting State

| # | Area      | State                                                                    |
|---|-----------|--------------------------------------------------------------------------|
| 1 | Workspace | Local checkout on `master` at `C0`.                                      |
| 2 | Plans     | Twenty plans indexed and placed.                                         |
| 3 | Sessions  | Two embedded sessions running, launched from plans A and B at `C0`.      |
| 4 | Canvas    | Session nodes are placed beside their plans; everything renders equally. |

### Visual State (Before)

```text
Canvas · master  (22 nodes, all equally bright)
├── Plan A ── Session claude · running
├── Plan B ── Session codex · running
└── Plans C..T
```

## Scenario 1: Focus Mode

### Flow 1.1: Toggle Focus

1. User clicks **Focus** in the Canvas toolbar.
2. Board computes the active set: sessions with state starting or running and `live`, plus their plans, plus edges
   between them.
3. Every other node is dimmed and non-interactive for selection but keeps its position.
4. Toolbar shows "Focus · 2 plans · 2 sessions". The choice persists per browser like the node-type facet.

### Flow 1.2: A Session Ends

1. Session codex exits.
2. Projection refresh marks it not live; Plan B and its session dim on the next render.
3. If no session is active, the toolbar shows "Focus · nothing running" and offers to turn Focus off.

### Visual State (After)

```text
Canvas · master · Focus
├── Plan A ── Session claude · running        (bright)
├── Plan B ── Session codex · exited          (dimmed)
└── Plans C..T                                 (dimmed)
```

## Scenario 2: Sessions Tab

### Flow 2.1: Open The Tab

1. User selects the workspace node.
2. Workbench shows tabs Info and **Sessions**.
3. Sessions lists this branch's records newest first: state, provider, plan, started, duration, changes count.
4. Row actions: **Open** selects and expands the session node; **Stop** cancels a live process after confirmation;
   **Changes** opens the session's Changes tab.

## Scenario 3: Session Changes

### Flow 3.1: Read Changes

1. User selects session claude and opens **Changes**.
2. Backend runs name-status and short stat against the launch commit `C0` in the workspace path and lists untracked
   files from status.
3. Tab shows "+312 −48 · 9 files · HEAD moved 2 commits" and the file list grouped by status.
4. Selecting a file loads its unified diff against `C0`.

### Flow 3.2: Two Sessions, Same Checkout

1. Sessions claude and codex both ran in the main checkout.
2. Both Changes tabs show the union of edits since their own launch commit; the tab states this in one line:
   "Changes since launch include work by any session in this checkout."

## Scenario 4: Sessions Page

### Flow 4.1: List And Stop

1. User opens **Sessions** from the left nav.
2. Page lists active sessions across all registered workspaces: workspace, branch, plan, provider, state, started,
   duration, changes count.
3. Scope toggle switches to all sessions with history.
4. **Stop** on a live row confirms and cancels through the existing endpoint; the row shows `cancelled`.
5. **Open** navigates to that workspace's Workbench with the session node selected and expanded.

## Scenario 5: Guard Rails

| Case                               | Behaviour                                                                                     |
|------------------------------------|-----------------------------------------------------------------------------------------------|
| Application restart                | Records without a binding are `interrupted` (PM-037); Focus and lists treat them as inactive. |
| Launch commit unknown              | Changes tab says "Launch commit not recorded" and shows working-tree status only.             |
| Launch commit no longer in history | Changes tab reports `launch_commit_missing` and falls back to working-tree status.            |
| Workspace path missing             | Changes are `unavailable` with reason `workspace_missing`; the row still lists.               |
| Cloud tenancy                      | The global list includes only workspaces the caller may read; changes 404 on denial.          |
| Remote Snapshot workspace          | Changes are `unsupported`; the list shows the record without a changes count.                 |
| Large diff                         | File list capped at 500 entries with a note; per-file diff capped at the existing bound.      |
