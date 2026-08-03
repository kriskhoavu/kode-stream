---
slug: platform-terminal-canvas-concept
title: Terminal Canvas Concepts
pageType: CONCEPT
roles: BA, DEVELOPER, USER
topics: canvas, workspace, plan, session, capabilities
summary: Terminal Canvas is a branch-scoped projection for arranging and operating workspace, plan, and session state.
sourceRef: plans/platform/PM-037/README.md
sourceRef: plans/platform/PM-038/README.md
sourceCount: 2
lastTicket: PM-038
---

## Purpose And Boundary
<!-- chunkId: platform-terminal-canvas-concept-purpose -->
<!-- keywords: canvas, projection, workspace, plan, session -->

Terminal Canvas supports the daily workspace → plan → session loop without becoming another source of truth. It stores
layout identity, viewport, positions, collapsed presentation, and checkout-aware references. Repository plans, Git state,
verification, safe session records, and live terminal processes remain authoritative in their own domains. See
[[platform-terminal-canvas-reference]] for exact ownership and [[platform-branch-context-concept]] for the operational
checkout boundary.

## Independent Runtime Concerns
<!-- chunkId: platform-terminal-canvas-concept-runtime-concerns -->
<!-- keywords: topology, content provider, execution provider, datastore, authorization -->

Deployment topology, workspace-content provider, execution provider, app-state datastore, and authorization are separate
axes. Actions use resolved capability states—available, unavailable, unsupported, forbidden, or conflicted—and are
revalidated by the owning service. UI behavior does not branch on Local, Cloud, Agentless, data-dir, or database names.

PM-037 delivers Local checkout content, Local process execution, and either data-dir or SQLite app state. Cloud Agent
execution and Agentless Remote Snapshot Canvas UX remain separate future provider capabilities.

## Movement And Relationships
<!-- chunkId: platform-terminal-canvas-concept-movement -->
<!-- keywords: movement, placement, relationships, groups, edges -->

Workspace, plan, and durable session nodes move independently. A workspace is a semantic anchor, not a spatial parent;
moving it does not move related nodes. Repository and application relationships are derived, read-only connections.
Canvas-only links, groups, notes, artifacts, multiple canvases, and collaboration are deferred. Future group movement
must use explicit membership rather than overlap.

## Safety Outcomes
<!-- chunkId: platform-terminal-canvas-concept-safety -->
<!-- keywords: branch, terminal, session, verification, stale -->

Terminal launch rechecks the plan and current checkout immediately before process start. Durable session metadata stays
separate from live PTY state, and verification outcome stays separate from fingerprint freshness. These boundaries are
exercised in [[platform-terminal-canvas-workflows]], [[platform-terminal-canvas-journey]], and
[[platform-branch-review-checkout-journey]]. Canvas never treats a reviewed snapshot branch as executable context.
