# Frontend Design: Focused Terminal Canvas

## Overview

Add a lazy-loaded `/canvas` route that renders one branch-scoped workspace Canvas with draggable workspace, plan, and
session nodes. The page uses the existing `@xyflow/react` dependency and Knowledge Graph interaction patterns, but it is
not a general graph editor. It presents current workspace state, launches branch-safe sessions, shows Git and
verification freshness, and links to existing detailed pages.

The frontend never derives support from deployment or workspace mode names. It renders the current action capability
state returned for each resolved entity and handles capability changes without discarding layout.

## Route And Context

| Route or Control                 | Behavior                                                                                     |
|----------------------------------|----------------------------------------------------------------------------------------------|
| `/canvas`                        | Resolve the default layout for the active workspace and selected branch.                     |
| Workspace navigation: **Canvas** | Open Canvas without changing workspace or branch selection.                                  |
| Branch control                   | Use the existing selected branch context; changing branch resolves a different layout.       |
| **Open full view**               | Navigate to the existing Workspace, Workstream, Item Workspace, or verification destination. |
| Node search                      | Find and focus a visible workspace, plan, or session node.                                   |

PM-037 does not add Canvas to the Chrome extension surface. It does not introduce canvas IDs, copies, lists, or
cross-workspace routing.

## Frontend Data Model

### API Types

| Type                    | Responsibility                                                                         |
|-------------------------|----------------------------------------------------------------------------------------|
| `CanvasLayout`          | Layout identity, workspace, branch context, viewport, and metadata version.            |
| `CanvasPlacement`       | Node ID, entity reference, absolute position, collapsed state, and placement revision. |
| `CanvasEntityRef`       | Workspace, branch-aware plan, or durable session identity.                             |
| `ResolvedCanvasNode`    | Current title, status, resolution, capabilities, and recovery guidance.                |
| `ActionCapability`      | Action state, reason code, message, and recovery actions.                              |
| `SessionRecord`         | Durable safe lifecycle metadata plus current live-binding availability.                |
| `VerificationFreshness` | `fresh`, `stale`, or `inconclusive` with verified/current revision summaries.          |
| `PlacementPatch`        | Changed coordinates and expected placement revision.                                   |

### Local Interaction State

| State                                       | Owner                           | Persistence                              |
|---------------------------------------------|---------------------------------|------------------------------------------|
| Layout and placement revisions              | Server query state              | App-state repository                     |
| Optimistic positions and dirty node IDs     | Canvas state hook               | Memory until acknowledged                |
| Selected node                               | Canvas page                     | Memory                                   |
| Node search query                           | Search control                  | Memory                                   |
| Inspector open state                        | Canvas page                     | Memory                                   |
| Terminal channel, grant, and xterm instance | Existing terminal-session layer | Memory only                              |
| Current capabilities and freshness          | Query projection                | Refetched; never written with placements |

## Node Types

### Workspace Node

Displays workspace name, selected branch, HEAD summary, dirty/conflict status, and the most important action
capabilities. Selecting it opens the Workspace panel with Git status and links to full workspace controls.

Dragging a Workspace node moves only that node. It does not translate plan or session nodes and does not change
workspace membership, branch selection, or repository state.

### Plan Node

Displays plan identifier, title, plan status, branch, observed commit, verification freshness, and live-session count.
Selecting it opens the Plan panel with launch, verification, and full-view actions.

The branch label remains visible at the normal detail level. An executable action never relies on color alone to show
whether the plan matches the current checkout.

### Session Node

Displays durable provider label, linked plan, requested branch, lifecycle state, and live-binding availability. A
top-right disclosure action expands a live binding into an interactive terminal, or an ended/interrupted record into
safe lifecycle detail. Selecting the session does not change its disclosure state or open the right Workbench.

Removing the node removes only its placement. Stopping a process requires a separate explicit action and confirmation.
Terminal input, text selection, and scrolling do not initiate Canvas drag or pan gestures; the session summary remains
the node drag target.

## Derived Connections

PM-037 may render subtle orientation lines for:

- Workspace contains plan, derived from repository/index state.
- Plan launched session, derived from the durable session record.

Connections have no handles, labels, delete action, or persistence in Canvas writes. The UI must not imply that moving,
hiding, or removing a node changes the underlying relationship.

## Initial And Restored Layout

- First load creates deterministic suggested positions for the workspace, current branch plans, and durable sessions.
- Existing placements always win over new suggestions.
- Newly indexed plans and sessions receive deterministic positions automatically during load or refresh.
- Automatic placement is silent and never moves existing nodes.
- Removed placements remain hidden and never re-enter automatic placement for that layout.
- **Reset layout** shows a preview and requires confirmation before patching positions.
- Page reload restores node positions. Current entity labels, status, Git state, capabilities, live-binding state, and
  verification freshness are resolved again.
- Viewport restoration must never hide the selected workspace with no obvious **Fit content** recovery control.

## Placement Save Behavior

1. Apply drag positions locally and record affected node IDs.
2. Save on drag end after a short debounce; coalesce multi-select movement into one bounded patch.
3. Include only position, collapsed state when changed, and expected placement revision.
4. Update successful revisions without replacing unaffected placements.
5. Preserve dirty positions through transient failures and retry with bounded backoff.
6. On conflict, pause retries for the affected nodes and offer **Reload position** or **Apply my position to latest** after
   an explicit comparison.
7. Warn before navigation only when dirty placement changes cannot be retained or retried.

Viewport saves use the layout metadata version and cannot conflict with placement revisions.

## Focused Workbench And Inline Sessions

| Selected Node                | PM-037 Content                                                                                                  |
|------------------------------|-----------------------------------------------------------------------------------------------------------------|
| Workspace                    | Right Workbench with branch and Git status summary, refresh, and links to existing workspace controls.          |
| Plan                         | Right Workbench with plan summary, branch context, launch, verification status/action, and **Open full view**.  |
| Live session                 | Canvas node whose top-right action opens the existing xterm surface, connection state, close, and cancellation. |
| Ended or interrupted session | Canvas node whose top-right action opens safe lifecycle summary and exit or interruption detail.                |
| Stale node                   | Safe explanation, placement removal, and explicit replacement candidate when available.                         |

PM-037 does not embed Markdown editing, file browsing, diff, Jira, complete Git controls, or full verification artifacts.
Those remain in their existing authoritative views.

Only one terminal surface is mounted for each expanded session node. Selection never changes disclosure. The top-right
action and terminal close action update the persisted `collapsed` placement field without stopping the process or
creating another subscriber or xterm owner; normal grant and reconnect rules apply after reload.

## Branch Mismatch UX

When `terminal.launch` is `conflicted` because the plan branch differs from the checkout:

- Show expected and current branch names next to the disabled launch action.
- Explain whether the working tree is dirty or conflicted when that information is available.
- Offer **Open branch controls** or **Refresh context** only when supported.
- Do not offer a one-click automatic switch from Canvas.
- Do not create a session placement for a rejected launch.
- If the branch changes between rendering and submission, display the structured server rejection and refresh the node.

The same launch idempotency key is reused while a single user submission is pending, preventing double-click process
creation.

## Session Lifecycle UX

| State                          | Presentation                                                                      |
|--------------------------------|-----------------------------------------------------------------------------------|
| `starting`                     | Durable record exists; launch is pending and cannot be submitted again.           |
| `running` with live binding    | Terminal can attach under existing grant rules.                                   |
| `running` without live binding | Refresh briefly; reconcile to interrupted rather than offering a false reconnect. |
| `exited`                       | Show exit outcome and optional relaunch.                                          |
| `cancelled`                    | Show explicit user cancellation.                                                  |
| `failed`                       | Show safe launch/process failure without command arguments or prompt.             |
| `interrupted`                  | Explain that metadata survived but the live process did not.                      |

A newly discovered running session is placed silently in collapsed presentation. A deliberately removed session remains
durable and may keep consuming a process slot, but Canvas does not recreate its hidden placement; users manage that
process through the existing session lifecycle surface.

## Git And Verification UX

### Git

The Workspace node and panel show current branch, clean/dirty/conflicted status, and changed-file count. Detailed file
operations remain in the existing Git UI.

### Verification

The Plan node shows result status separately from freshness:

- **Passed · current**
- **Passed · stale**
- **Failed · stale**
- **Inconclusive · repository changed during run**
- **Running for {abbreviated fingerprint}**

A stale green result must not use the same success emphasis as a current green result. The panel shows verified and
current branch/commit summaries and offers rerun when `verification.run` is available. Repository changes update
freshness without moving the node.

## Capability Presentation

| State         | UI Behavior                                                                               |
|---------------|-------------------------------------------------------------------------------------------|
| `available`   | Enable the action.                                                                        |
| `unavailable` | Disable it and show temporary recovery guidance.                                          |
| `unsupported` | Hide secondary actions or show a concise unsupported explanation for primary actions.     |
| `forbidden`   | Disable or hide according to disclosure policy and explain the missing permission safely. |
| `conflicted`  | Disable execution and present the context that must be resolved.                          |

Capability reason codes drive presentation; deployment, access-mode, provider, and datastore labels do not drive
behavior. Labels may still appear as contextual information when useful.

## Interaction Design

| Interaction              | Result                                                                                     |
|--------------------------|--------------------------------------------------------------------------------------------|
| Single select            | Focus a workspace or plan and update the Workbench; focus a session without disclosing it. |
| Session top-right action | Expand or collapse the terminal independently from node selection.                         |
| Double click or Enter    | Open the full entity view, or focus an already expanded terminal for a session node.       |
| Drag node                | Move only selected placement or selected placement set.                                    |
| Node search              | Filter by current resolved title, identifier, branch, or session state and focus a result. |
| Fit content              | Fit visible nodes without changing saved positions.                                        |
| Reset layout             | Preview deterministic positions and confirm before applying them.                          |
| Remove node              | Remove placement only after stating that the source entity and process are unchanged.      |

There are no connection handles, group drop targets, note editors, edge menus, minimap requirement, or semantic zoom
modes in PM-037.

## Accessibility

- Provide a normal toolbar, node search/list, plan/workspace Workbench, and status regions outside the viewport.
- Give every node an accessible name with kind, title, branch, lifecycle/status, and blocked state.
- Support keyboard node selection and bounded keyboard movement with an announced save result.
- Return focus predictably between a node, its inline terminal, the Workbench, and dialogs.
- Announce load, selection, save, conflict, branch mismatch, session lifecycle, and verification freshness changes.
- Provide visible zoom and fit controls; do not require precise drag gestures for essential navigation.
- Use text and icon cues in addition to color.
- Respect reduced motion and disable smooth viewport transitions when requested.
- Preserve visible focus rings, minimum target sizes, and both-theme contrast.

## Responsive Behavior

PM-037 targets desktop engineering workflows. On narrow windows, the plan/workspace Workbench becomes an overlay and
the expanded terminal node uses a smaller bounded width. The Canvas remains recoverable through search and fit controls
but does not claim touch-first editing support.

## Performance

- Lazy-load Canvas and xterm code.
- Resolve node projections in bounded backend batches.
- Memoize node components and avoid one request per node.
- Mount the terminal only for an explicitly expanded live session.
- Measure first render, drag, save, and selection at 25, 100, and the 300-placement MVP limit.
- Avoid animated connections and background auto-layout.

## Testing

| Level                | Coverage                                                                                              |
|----------------------|-------------------------------------------------------------------------------------------------------|
| Model unit           | API normalization, branch-aware references, derived connections, and verification labels.             |
| Hook unit            | Resolve/load, placement patching, retry, per-node conflict, viewport independence, and unplaced work. |
| Component            | Three node kinds, movement semantics, branch mismatch, capability states, and session reconciliation. |
| Terminal integration | One live binding and one terminal owner; selection never relaunches or cancels.                       |
| Router               | Active workspace and branch changes resolve the correct layout.                                       |
| Accessibility        | Search, keyboard movement, focus return, live regions, reduced motion, and non-color states.          |
| Browser playbook     | Arrange and restore, launch safely, inspect Git, verify, mutate repository, and observe stale result. |

## Design Decisions

| Decision                                          | Rationale                                                                                        |
|---------------------------------------------------|--------------------------------------------------------------------------------------------------|
| Three semantic node types only                    | Keeps the first workflow coherent and daily-use focused.                                         |
| Workspace moves independently                     | Semantic containment never becomes surprising spatial parenting.                                 |
| Thin Workbench                                    | Existing detailed views stay authoritative while Canvas validates orchestration value.           |
| Placement patches                                 | Drag saves stay small and unrelated nodes do not conflict.                                       |
| Branch context always visible                     | Terminal execution remains understandable and trustworthy.                                       |
| Durable record shown separately from live binding | Restored metadata never promises a process that no longer exists.                                |
| Verification status and freshness are separate    | A historical pass cannot look current after repository changes.                                  |
| No graph authoring in PM-037                      | Relationship meaning is deferred until repository, application, and visual origins are explicit. |
