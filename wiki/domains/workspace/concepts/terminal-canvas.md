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
<!-- keywords: canvas, projection, workspace, plan, session -->

[open] Does the current implementation still satisfy the purpose and boundary behavior described below? This section has not yet been reverified.

Terminal Canvas supports the daily workspace → plan → session loop without becoming another source of truth. It stores
layout identity, viewport, positions, collapsed presentation, and checkout-aware references. Repository plans, Git state,
verification, safe session records, and live terminal processes remain authoritative in their own domains. See
[[workspace-terminal-canvas-reference]] for exact ownership and [[workspace-branch-context]] for the operational
checkout boundary.

## Independent Runtime Concerns
<!-- chunkId: workspace-terminal-canvas-runtime-concerns -->
<!-- keywords: topology, content provider, execution provider, datastore, authorization -->

[open] Does the current implementation still satisfy the independent runtime concerns behavior described below? This section has not yet been reverified.

Deployment topology, workspace-content provider, execution provider, app-state datastore, and authorization are separate
axes. Actions use resolved capability states—available, unavailable, unsupported, forbidden, or conflicted—and are
revalidated by the owning service. UI behavior does not branch on Local, Cloud, Agentless, data-dir, or database names.

PM-037 delivers Local checkout content, Local process execution, and either data-dir or SQLite app state. Cloud Agent
execution and Agentless Remote Snapshot Canvas UX remain separate future provider capabilities.

## Movement And Relationships
<!-- chunkId: workspace-terminal-canvas-movement -->
<!-- keywords: movement, placement, relationships, groups, edges -->

[open] Does the current implementation still satisfy the movement and relationships behavior described below? This section has not yet been reverified.

Workspace, plan, and durable session nodes move independently. A workspace is a semantic anchor, not a spatial parent;
moving it does not move related nodes. Repository and application relationships are derived, read-only connections.
Canvas-only links, groups, notes, artifacts, multiple canvases, and collaboration are deferred. Future group movement
must use explicit membership rather than overlap.

## Safety Outcomes
<!-- chunkId: workspace-terminal-canvas-safety -->
<!-- keywords: branch, terminal, session, verification, stale -->

[open] Does the current implementation still satisfy the safety outcomes behavior described below? This section has not yet been reverified.

Terminal launch rechecks the plan and current checkout immediately before process start. Durable session metadata stays
separate from live PTY state, and verification outcome stays separate from fingerprint freshness. These boundaries are
exercised in [[workspace-terminal-canvas-workflows]], [[e2e-testing-workspace-terminal-canvas]], and
[[e2e-testing-workspace-branch-review-checkout]]. Canvas never treats a reviewed snapshot branch as executable context.

## Not supported
<!-- chunkId: workspace-terminal-canvas-not-supported -->
<!-- keywords: capability boundary, exclusions, unsupported action -->

[open] Which additional unsupported operations should this page name after implementation verification?

## Known conflicts
<!-- chunkId: workspace-terminal-canvas-known-conflicts -->
<!-- keywords: documentation drift, verification gap, competing description -->

[open] Do the retained plan descriptions disagree with current behavior? Verify the sections above before relying on them.

## Related pages

See [[workspace-terminal-canvas-reference]], [[workspace-index]], and [[e2e-testing-index]].
