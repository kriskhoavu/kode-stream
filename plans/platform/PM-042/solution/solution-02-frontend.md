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

The first cut dimmed inactive nodes in place. Trying it showed that dimming leaves the board just as crowded; Focus
now removes everything inactive and lays out what remains.

| Element      | Behaviour                                                                                                                                                                                                                                                                                                                                   |
|--------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Toggle       | Toolbar button "Focus" with `aria-pressed`; label shows "Focus · n plans · m sessions" when on.                                                                                                                                                                                                                                             |
| Active set   | Session nodes with state starting or running and `live`, their plans, and edges among them.                                                                                                                                                                                                                                                 |
| Hiding       | Every node, section, and edge outside the active set is removed from the rendered graph.                                                                                                                                                                                                                                                    |
| Focus layout | Active plans stack in one column on the left; each plan's sessions sit in a row to its right, expanded so the terminal is visible; rows are spaced by the tallest node. The layout is virtual: it overrides positions only while Focus is on, nodes are not draggable, and the saved layout is untouched and restored when Focus turns off. |
| Fit          | Turning Focus on fits the viewport to the focus layout; turning it off restores the saved viewport.                                                                                                                                                                                                                                         |
| Persistence  | Browser preference for the toggle only; never the virtual positions.                                                                                                                                                                                                                                                                        |
| Empty        | When nothing is active: "Focus · nothing running" and a one-click "Turn off".                                                                                                                                                                                                                                                               |
| Cost         | The active set and the layout are one memoised pass over the projection; no extra fetch.                                                                                                                                                                                                                                                    |

## Workbench

| Selection      | Tabs              | Content                                                                                                                                                                                                      |
|----------------|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Workspace node | Info, Sessions    | Sessions: the Session Browser (below) for this workspace and branch. While this tab is active the panel widens to about 60vw, still resizable, so the live terminal is usable without leaving the Workbench. |
| Session node   | Terminal, Changes | Terminal: session summary and Stop; the live terminal stays in the node. Changes: `SessionChangesPanel`.                                                                                                     |

## Session Browser

One component serves the Workbench Sessions tab and the Sessions page. It follows the shape Orca uses for agents: a
list of cards on the left, the live terminal of the selected one on the right.

| Element     | Behaviour                                                                                                                                                                                                                                                |
|-------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| List        | Cards grouped by plan (Workbench) or by workspace then plan (Sessions page). Each card: state glyph, title (plan title, else intent, else provider), provider, age since start, and for finished sessions the exit code. Sorted live first, then newest. |
| State glyph | Spinner for starting or running, green dot for exited with code 0, red dot for a non-zero exit or interrupted, grey dot for cancelled. A text label always sits beside the glyph.                                                                        |
| Filter      | One text filter over title, provider, plan; a "Live only" toggle, on by default when any session is live.                                                                                                                                                |
| Detail      | Header with title, plan, branch, launch commit, started, duration; sub-tabs Terminal and Changes; Stop when live. Terminal reuses the Canvas session terminal component with `visible` true; Changes reuses `SessionChangesPanel`.                       |
| Selection   | Arrow keys move the selection, Enter opens, the selected card is `aria-current`; remembered per browser per workspace.                                                                                                                                   |
| Empty       | "No sessions yet. Start one from a plan on the Workbench." with a link on the Sessions page.                                                                                                                                                             |
| Cost        | One list fetch, polled every 5 s while any row is live; the terminal attaches only for the selected session.                                                                                                                                             |

`SessionsTable` is retired; both surfaces render the browser.

## Sessions Page

| Element | Behaviour                                                                                                                       |
|---------|---------------------------------------------------------------------------------------------------------------------------------|
| Scope   | Segmented control Active / All; default Active; persisted per browser.                                                          |
| Filters | The Session Browser's own text filter (title, provider, plan) and its Live-only toggle, on by default when any session is live. |
| Browser | The Session Browser at page width, grouped by workspace then plan; the selected session's terminal is live in the detail pane.  |
| Actions | Open on the board navigates to the Workbench with the session selected; Stop confirms and cancels; Changes is a detail sub-tab. |
| Live    | Polls every 5s while any row is live; a row that stops shows its exit code inline.                                              |
| Empty   | "No sessions running. Start one from a plan on the Workbench." with a link.                                                     |

## Accessibility

- Focus toggle is a button with `aria-pressed`; dimmed nodes stay in the accessibility tree with `aria-hidden` false
  but are marked "inactive" in their label.
- The card list uses a roving `tabindex`: the selected card if it is visible, else the first visible card, so the
  list stays keyboard-reachable when a filter hides the selection; the selected card also carries `aria-current`.
  Each card pairs a state glyph with a text label rather than colour alone.
- Stop confirmations are dialogs with focus trap and Cancel default.

## Design Decisions

| Decision                                   | Rationale                                                                                                  |
|--------------------------------------------|------------------------------------------------------------------------------------------------------------|
| One Session Browser for Workbench and page | Same cards, filter and detail pane; only the grouping (plan vs. workspace-plan) and the row source differ. |
| Dim, do not hide                           | Keeps positions and avoids React Flow re-layout, which is what made sections slow.                         |
| Workbench opens for a session node         | Changes need a home; the terminal remains in the node to keep one process surface.                         |
| Poll at 5s only while live                 | Matches existing Canvas behaviour; nothing polls when nothing runs.                                        |
