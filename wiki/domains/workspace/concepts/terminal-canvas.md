---
slug: workspace-terminal-canvas
title: Terminal Canvas Concepts
pageType: CONCEPT
topics: canvas, workspace, plan, session, capabilities
summary: Terminal Canvas is a branch-scoped projection for arranging and operating workspace, plan, and session state.
sourceRef: |
  PM-037 (plan retired) | Terminal Canvas
  PM-038 (plan retired) | Checkout context
lastVerified: never
---

## Purpose And Boundary
<!-- chunkId: workspace-terminal-canvas-purpose -->
<!-- keywords: projection, layout, source of truth, ownership -->

[open] Does the code still match this page? It is plan-derived and not yet checked.

Canvas supports the daily workspace → plan → session loop. It stores only layout: viewport, positions, collapsed state and references, per workspace and checkout branch. Plans, Git state, verification, session records and live terminals stay with their own owners. See [[workspace-terminal-canvas-reference]] and [[workspace-branch-context]].

## Independent Runtime Concerns
<!-- chunkId: workspace-terminal-canvas-runtime-concerns -->
<!-- keywords: content provider, execution provider, datastore, authorization -->

Deployment topology, content provider, execution provider, app-state store and authorization are separate axes. Each action carries a resolved capability state: available, unavailable, unsupported, forbidden or conflicted. The owning service revalidates it. The UI never branches on Local, Cloud, Agentless, `datadir` or database names.

Canvas works with a Local checkout and Local process execution.

## Movement And Relationships
<!-- chunkId: workspace-terminal-canvas-movement -->
<!-- keywords: placement, drag, derived edges, anchor -->

Workspace, plan and session nodes move independently. A workspace is a semantic anchor, not a spatial parent. Edges between repository and application entities are derived and read-only.

A named section groups two or more nodes. The browser keeps sections per layout, not the server. **Reset layout** arranges nodes into default service sections.

## Safety Outcomes
<!-- chunkId: workspace-terminal-canvas-safety -->
<!-- keywords: branch recheck, stale result, live process, record -->

- Terminal launch rechecks the plan and the checkout just before the process starts.
- A durable session record is separate from the live process. A reload can reattach while the process lives. After an app restart, a record without a process shows as interrupted.
- Removing a session node deletes only its placement. **Cancel process** is a separate confirmed action.
- Verification outcome is separate from freshness. A pass turns **Passed (historical)** when the repository changes.
- Canvas never treats a reviewed branch as executable context.

## Not supported
<!-- chunkId: workspace-terminal-canvas-not-supported -->
<!-- keywords: groups, notes, multiple canvases, cloud execution -->

Notes, artifacts, custom links, multiple canvases, collaborative layouts, sections shared across browsers, Cloud Agent execution and a Remote Snapshot Canvas.

## Known conflicts
<!-- chunkId: workspace-terminal-canvas-known-conflicts -->
<!-- keywords: workspace parent, node movement, layout authority -->

- "Moving a workspace node moves its plans." It does not. Nodes move independently.

## Related pages

See [[workspace-terminal-canvas-workflows]], [[workspace-terminal-canvas-reference]] and [[e2e-testing-workspace-terminal-canvas]].
