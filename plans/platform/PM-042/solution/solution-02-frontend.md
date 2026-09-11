# Frontend Design: PM-042

## Overview

Three surfaces read one session list. Focus mode is a Canvas toolbar toggle. The Workbench gains a Sessions tab on the
workspace node and a Changes tab on a session node. A new Sessions page lists sessions across workspaces. No new node
kind, section, or launch button.

## Types

| Type               | Purpose                                                                                                                       |         |     |     |     |                 |
|--------------------|-------------------------------------------------------------------------------------------------------------------------------|---------|-----|-----|-----|-----------------|
| `SessionChanges`   | Mirrors the backend value: launchCommit, headCommit, headMoved, files, additions, deletions, truncated, availability, reason. |         |     |     |     |                 |
| `SessionFile`      | path, status `'A'                                                                                                             | 'M'     | 'D' | 'R' | 'C' | '?'`, oldPath?. |
| `GlobalSessionRow` | record, workspaceName, checkoutBranch, planTitle?, changesCount?.                                                             |         |     |     |     |                 |
| `SessionScope`     | `'active'                                                                                                                     | 'all'`. |     |     |     |                 |

## API Client

`listSessions(scope)`, `sessionChanges(id)`, `sessionFileDiff(id, path)`. Cancel reuses `cancelEmbeddedAISession`.

## Components

```text
features/canvas/
├── CanvasBoard.tsx            # Focus toggle in the toolbar; dimmed class on non-active nodes; edge filtering
├── CanvasWorkbench.tsx        # workspace node: Info | Sessions; session node: Terminal | Changes
├── SessionsTable.tsx          # shared table: state, provider, plan, started, duration, changes; Open, Stop, Changes
├── SessionChangesPanel.tsx    # header line, grouped file list, per-file diff <pre>, truncation and availability notes
└── useSessionChanges.ts       # loads changes for a session; refreshes while the session is live (5s), request-token guarded
features/sessions/
├── SessionsPage.tsx           # scope toggle, workspace filter, SessionsTable, empty state
└── useGlobalSessions.ts       # loads /api/sessions; polls every 5s while any row is live; cancels on unmount
pages/SessionsPage.tsx         # route shell
App.tsx                        # route 'sessions', left-nav entry "Sessions" between Workbench and Knowledge
```

## Board: Focus Mode

| Element     | Behaviour                                                                                                   |
|-------------|-------------------------------------------------------------------------------------------------------------|
| Toggle      | Toolbar button "Focus" with `aria-pressed`; label shows "Focus · n plans · m sessions" when on.             |
| Active set  | Session nodes with state starting or running and `live`, their plans, and edges among them.                 |
| Dimming     | Non-active nodes get `.canvas-node-dimmed` (opacity, no pointer events for selection); positions unchanged. |
| Sections    | Unchanged; a section with no active member is dimmed with its members.                                      |
| Persistence | Browser preference beside the node-type facet.                                                              |
| Empty       | When nothing is active: "Focus · nothing running" and a one-click "Turn off".                               |
| Cost        | The active set is one pass over `projection.nodes`, memoised on the projection; no extra fetch.             |

## Workbench

| Selection      | Tabs              | Content                                                                                                                                                                                                                         |
|----------------|-------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Workspace node | Info, Sessions    | Sessions: `SessionsTable` for this workspace and branch from the existing session-records endpoint. Open selects and expands the session node; Stop confirms then cancels; Changes selects the session node on its Changes tab. |
| Session node   | Terminal, Changes | Terminal: existing embedded terminal. Changes: `SessionChangesPanel`.                                                                                                                                                           |

The Workbench currently hides itself for a session node; it now opens with the two tabs above. The terminal stays in
the node and is not duplicated in the panel; the Terminal tab holds the session summary and Stop.

## Sessions Page

| Element | Behaviour                                                                                                                                                                     |
|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Scope   | Segmented control Active / All; default Active; persisted per browser.                                                                                                        |
| Filters | Workspace multi-select; provider multi-select; text search over plan title and intent.                                                                                        |
| Table   | Workspace, branch, plan, provider, state, started, duration, changes; sorted newest first.                                                                                    |
| Actions | Open navigates to the Workbench for that workspace with the session selected and expanded; Stop confirms and cancels; Changes opens a side drawer with `SessionChangesPanel`. |
| Live    | Polls every 5s while any row is live; a row that stops shows its exit code inline.                                                                                            |
| Empty   | "No sessions running. Start one from a plan on the Workbench." with a link.                                                                                                   |

## Accessibility

- Focus toggle is a button with `aria-pressed`; dimmed nodes stay in the accessibility tree with `aria-hidden` false
  but are marked "inactive" in their label.
- Tables use `<table>` with header cells; row actions are buttons with explicit labels including the plan title.
- Stop confirmations are dialogs with focus trap and Cancel default.

## Design Decisions

| Decision                                   | Rationale                                                                          |
|--------------------------------------------|------------------------------------------------------------------------------------|
| One `SessionsTable` for Workbench and page | Same columns and actions; the only difference is the data source.                  |
| Dim, do not hide                           | Keeps positions and avoids React Flow re-layout, which is what made sections slow. |
| Workbench opens for a session node         | Changes need a home; the terminal remains in the node to keep one process surface. |
| Poll at 5s only while live                 | Matches existing Canvas behaviour; nothing polls when nothing runs.                |
