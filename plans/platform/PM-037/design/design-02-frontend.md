# Frontend Design: Terminal Canvas And Workbench

## Overview

Add a lazy-loaded `/canvas` route and Workspace navigation entry. The page uses the existing `@xyflow/react`
dependency and Knowledge Graph conventions for pan, zoom, selection, handles, and viewport fitting. Custom semantic
nodes show current workspace, plan, session, and artifact state. Selecting a node opens a resizable right or bottom
Workbench that composes existing terminal, item, file, diff, Git, and verification features.

The Canvas is an additional orchestrator, not a replacement for Workstream or Item Workspace. Existing detailed pages
remain directly reachable from node actions.

## Route And Navigation

| Route / control         | Behavior                                                                                 |
|-------------------------|------------------------------------------------------------------------------------------|
| `/canvas`               | Resolve the default Canvas for the active workspace.                                     |
| `/canvas?canvasId={id}` | Open a specific owned Canvas document.                                                   |
| Workspace nav: Canvas   | Navigate to the active workspace's Canvas without changing workspace selection.          |
| Node: Open full view    | Navigate to existing Item Workspace, Workstream, Workspace, or verification destination. |
| Command palette         | Search and focus Canvas nodes while the Canvas route is active.                          |

The Canvas page is not rendered in the Chrome extension surface during the first PM-037 delivery because PM-034 does
not support embedded terminal streaming there. A future extension-safe read-only surface can reuse the same capability
gates without changing the Canvas data model.

## Frontend Data Model

### API Types

| Type                   | Responsibility                                                                      |
|------------------------|-------------------------------------------------------------------------------------|
| `CanvasDocument`       | Persisted document identity, version, viewport, preferences, nodes, and edges.      |
| `CanvasNode`           | Node layout, kind, entity reference, grouping, and note presentation data.          |
| `CanvasEdge`           | Typed relationship between two node IDs.                                            |
| `ResolvedCanvasNode`   | Current title, status, resolution, capabilities, and recovery guidance.             |
| `CanvasCapabilities`   | Effective layout, repository, terminal, AI, Git, runtime, and verification actions. |
| `CanvasUpdateInput`    | Expected version plus complete validated spatial document body.                     |
| `ActiveSessionSummary` | Safe session lifecycle metadata without channel grant or terminal bytes.            |

### Local Interaction State

| State                     | Owner                | Persistence                                                         |
|---------------------------|----------------------|---------------------------------------------------------------------|
| Selected node IDs         | `CanvasPage`         | Session only.                                                       |
| Active Workbench tab      | Workbench controller | Local storage per user and Canvas; not server domain state.         |
| Unsaved nodes and edges   | Canvas editor hook   | In memory until debounced save succeeds.                            |
| Save status and conflict  | Canvas editor hook   | In memory; visible through status region and conflict dialog.       |
| Search and focus mode     | Canvas toolbar       | Session only unless promoted to a display preference.               |
| Saved viewport/layout     | Canvas document      | App-state repository through versioned API.                         |
| Terminal emulator/process | Existing AI session  | Existing in-memory manager and WebSocket; never Canvas persistence. |

## Component Structure

> `CanvasPage` -> `CanvasToolbar`, `CanvasViewport`, `CanvasWorkbench`, `CanvasStatusRegion`, and conflict/empty dialogs.

> `CanvasViewport` -> React Flow -> `WorkspaceCanvasNode`, `PlanCanvasNode`, `SessionCanvasNode`, `ArtifactCanvasNode`,
> `NoteCanvasNode`, and `GroupCanvasNode`.

> `CanvasWorkbench` -> `TerminalWorkbench`, `PlanWorkbench`, `ArtifactWorkbench`, or read-only snapshot guidance.

Suggested files:

| File / area                                         | Purpose                                                                               |
|-----------------------------------------------------|---------------------------------------------------------------------------------------|
| `web/src/pages/CanvasPage.tsx`                      | Route-level loading, ownership, empty, error, and orchestration states.               |
| `web/src/features/canvas/CanvasViewport.tsx`        | React Flow setup, viewport, selection, keyboard focus, and node/edge change handling. |
| `web/src/features/canvas/canvasModel.ts`            | Convert API document plus projections into React Flow nodes and edges.                |
| `web/src/features/canvas/useCanvasDocument.ts`      | Load/resolve, optimistic edit state, debounced save, conflict, and retry behavior.    |
| `web/src/features/canvas/useCanvasLayout.ts`        | Deterministic initial and reset layout.                                               |
| `web/src/features/canvas/nodes/*`                   | Semantic node components with bounded detail and accessible actions.                  |
| `web/src/features/canvas/CanvasWorkbench.tsx`       | Selected-node workbench shell and resize behavior.                                    |
| `web/src/features/ai-session/TerminalWorkbench.tsx` | Reusable terminal renderer extracted from the current dock.                           |
| `web/src/features/canvas/canvas.css`                | Canvas, node, toolbar, Workbench, responsive, and motion styling.                     |

## Semantic Zoom

| Detail level | Approximate zoom | Visible information                                                            |
|--------------|------------------|--------------------------------------------------------------------------------|
| Overview     | below 0.55       | Node kind, title, aggregate health/status, active-session count.               |
| Standard     | 0.55 to 1.25     | Branch/status, owner/provider, dependency badges, verification summary.        |
| Detail       | above 1.25       | Secondary metadata and compact actions; full content still opens in Workbench. |

Zoom thresholds are frontend constants and do not change persisted entity data. Node dimensions remain bounded so
semantic detail does not cause uncontrolled re-layout.

## Default Layout

The initial layout is deterministic and recoverable:

1. Place one workspace node at the left origin.
2. Group plan nodes in columns by existing status order.
3. Order plans by current Workstream ordering with stable item ID as a tie-breaker.
4. Place active session nodes beside their associated plan; place workspace-only sessions in a separate workbench lane.
5. Place verification artifacts after the plan or session that produced them.
6. Fit the viewport only after node sizes settle.
7. Save positions after creation or explicit reset preview acceptance.

Manual node moves do not trigger auto-layout. New indexed plans enter an **Unplaced work** tray until the user chooses
**Place new items** or runs auto-layout, preventing background scans from rearranging spatial memory.

## Workbench Behavior

| Selected kind | Default Workbench content                                                                 |
|---------------|-------------------------------------------------------------------------------------------|
| Workspace     | Workspace health, branch state, scan, and workspace-level AI launch.                      |
| Plan          | Markdown preview, files, metadata, diff, Jira, Git, verification, and AI launch controls. |
| Session       | Existing xterm terminal plus lifecycle controls and plan/workspace context.               |
| Artifact      | Verification result, Git-change summary, commit, or linked file detail.                   |
| Note / group  | App-owned text or group properties; never repository content.                             |
| Stale node    | Resolution explanation, remove reference, locate replacement, or relaunch action.         |

Extract reusable terminal presentation from `EmbeddedTerminalDock` while preserving the dock as an alternate host.
One session has one xterm/channel attachment at a time. Moving it between dock and Canvas Workbench transfers the
presentation owner rather than creating two simultaneous terminal consumers.

## Capability Presentation

Node and Workbench actions consume the backend capability snapshot and current global runtime context.

| State                  | Presentation                                                                            |
|------------------------|-----------------------------------------------------------------------------------------|
| Local writable         | Full repository and terminal actions.                                                   |
| Agent-Backed online    | Full role-authorized actions with **Cloud Agent** execution label.                      |
| Agent-Backed offline   | Read model remains; execution actions disabled with **Reconnect owner Agent** guidance. |
| Remote Snapshot        | Commit/ref label, read-only content, and **Connect Agent** or **Open locally** handoff. |
| Role lacks Canvas edit | Canvas controls are read-only; entity actions follow their independent permissions.     |
| Stale entity           | Mutating/execution controls hidden; recovery actions remain.                            |

Do not infer support from `runtimeContext.mode` alone. Workspace access mode, role, effective capabilities, and Agent
availability may each remove actions.

## Save And Conflict UX

- Apply node moves, viewport changes, grouping, notes, and edges locally first.
- Debounce document saves and coalesce rapid drag/zoom changes.
- Show compact **Saving**, **Saved**, **Offline**, or **Conflict** state without blocking Canvas interaction.
- Keep unsaved state in memory after transient failures and retry with bounded backoff.
- On `409`, stop automatic retries and show **Reload latest** and **Keep my layout as a copy**.
- Warn before navigation only when unsaved changes cannot be queued or recovered.
- Never include resolved titles/status, terminal grants, terminal output, file content, or diffs in update payloads.

## Interaction Design

| Interaction          | Result                                                                                |
|----------------------|---------------------------------------------------------------------------------------|
| Single select        | Focus node and open/update Workbench.                                                 |
| Double click / Enter | Open the entity's full existing page or activate terminal focus.                      |
| Drag node            | Move within Canvas and mark layout dirty.                                             |
| Connect handles      | Offer only relationship kinds valid for the selected source/target kinds.             |
| Search               | Filter by visible title/context and focus the chosen node.                            |
| Focus mode           | Dim or hide nodes not connected to the selected plan.                                 |
| Fit content          | Fit currently visible nodes without changing saved positions.                         |
| Auto-layout          | Preview deterministic positions; confirmation replaces current positions.             |
| Delete node          | Remove reference only; explicit message states the source entity is unchanged.        |
| Delete Canvas        | Confirm metadata-only deletion; workspace, plans, sessions, and Git remain unchanged. |

## Accessibility

- Treat the toolbar and Workbench as normal focusable regions; do not rely on Canvas drag gestures for essential actions.
- Provide node search/list navigation as the keyboard-equivalent spatial navigator.
- Give every node an accessible name containing kind, title, status, and blocked state.
- Return focus predictably between selected node and Workbench controls.
- Announce load, selection, save, conflict, Agent availability, and session lifecycle in polite live regions.
- Expose visible zoom, fit, focus, auto-layout, and reset controls.
- Use text and iconography in addition to color for edge kinds and node states.
- Respect `prefers-reduced-motion`; disable animated edges and smooth viewport transitions.
- Preserve minimum target sizes, visible focus rings, and usable contrast in both themes.

## Responsive Behavior

| Width           | Canvas / Workbench behavior                                                                     |
|-----------------|-------------------------------------------------------------------------------------------------|
| Large desktop   | Canvas fills content area; Workbench docks right with bounded resize.                           |
| Medium desktop  | Workbench may switch between right panel and bottom panel.                                      |
| Narrow viewport | Canvas remains navigable; Workbench becomes a full overlay with explicit return-to-node action. |

The first release optimizes for desktop engineering workflows. It must remain recoverable on narrow windows but does
not claim touch-first mobile editing.

## Performance

- Lazy-load `CanvasPage`, React Flow node code, and xterm Workbench code.
- Resolve entity state in backend batches and avoid per-node network requests.
- Memoize node components and pass small projection objects.
- Render full terminal, Markdown, diff, and verification content only for the selected Workbench node.
- Keep animations disabled for large graphs and reduced-motion users.
- Measure initial layout, save payload, and interaction performance at 100, 500, and the 1,000-node server limit.

## Testing

| Level            | Coverage                                                                                        |
|------------------|-------------------------------------------------------------------------------------------------|
| Model unit       | Entity-to-node conversion, semantic detail, edge kinds, deterministic layout, stale references. |
| Hook unit        | Resolve/load, debounced save, retry, conflict pause, copy recovery, active-session refresh.     |
| Component        | Empty/loading/error, node selection, Workbench switching, capability denial, Agent state.       |
| Router/App shell | `/canvas`, active workspace changes, lazy loading, extension-surface exclusion.                 |
| Accessibility    | Labels, focus return, live regions, visible controls, reduced motion, keyboard search.          |
| Browser playbook | Create/restore Canvas, launch terminal, inspect read-only snapshot, recover offline/conflict.   |

## Design Decisions

| Decision                                      | Rationale                                                                                       |
|-----------------------------------------------|-------------------------------------------------------------------------------------------------|
| Reuse React Flow and Knowledge Graph patterns | Reduces custom viewport code and keeps interactions consistent with an existing dependency.     |
| Lazy-load Canvas and full Workbench content   | Protects current Workstream startup and avoids rendering many expensive detail components.      |
| Keep one selected full terminal               | Prevents dozens of xterm renderers and competing channel consumers while nodes remain live.     |
| Use an unplaced-work tray                     | Background indexing must not destroy the user's saved spatial memory.                           |
| Preserve existing full pages                  | Canvas provides orchestration while mature page-specific actions remain available.              |
| Explicit conflict recovery                    | Users choose between the latest shared document and their own copy; no silent coordinate loss.  |
| Capability-driven controls                    | One UI supports Local, connected/offline Agent-Backed, Agentless, and role restrictions safely. |
