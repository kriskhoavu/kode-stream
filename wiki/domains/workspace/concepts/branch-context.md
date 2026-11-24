---
slug: workspace-branch-context
title: Checkout And Branch Review Concepts
pageType: CONCEPT
topics: checkout, branch review, snapshot, operational context, import
summary: Kode Stream uses the Git checkout for every operational page and isolates other branches in commit-pinned read-only review.
sourceRef: |
  PM-013 (plan retired) | Superseded snapshot materialization
  PM-038 (plan retired) | Checkout-first branch context
lastVerified: never
---

## One Operational Context
<!-- chunkId: workspace-branch-context-operational-context -->
<!-- keywords: checkout, working tree, pages, refresh -->

[open] Does the code still match this page? It is plan-derived and not yet checked.

The current Git checkout is the only operational context. Workstream, Item Workspace, Canvas, Knowledge, editing, terminal launch and verification all read the checkout from Git. They never trust a remembered branch or a route parameter. A focus or visibility refresh picks up Git changes made outside Kode Stream.

## Branch Review Boundary
<!-- chunkId: workspace-branch-context-review-boundary -->
<!-- keywords: pinned commit, read-only, snapshot, committed content -->

Branch Review is a separate route for a local branch that is not the checkout. The server resolves the branch to a commit and returns read-only plans and files from that commit. Review cannot edit, move statuses, launch terminals, run verification or change Git. See [[workspace-branch-review-workflows]].

## Explicit Import
<!-- chunkId: workspace-branch-context-explicit-import -->
<!-- keywords: structured plan, conflict, source commit, copy -->

To bring a plan from a reviewed branch into the checkout, a person imports it. Kode Stream revalidates the source commit and refuses an existing target, an unsafe path, a documentation-only item or a moved branch. The checkout and its other working-tree changes stay as they are. See [[workspace-branch-context-reference]].

## Why Snapshot State Is Not Global
<!-- chunkId: workspace-branch-context-no-global-snapshot -->
<!-- keywords: design decision, authoritative branch, page agreement -->

One global snapshot selection let pages disagree about the active branch. Each route now makes one promise: operational pages act on the checkout, and review only reads a pinned commit. Branch-scoped indexes and Canvas layouts stay as derived state but do not drive navigation.

## Not supported
<!-- chunkId: workspace-branch-context-not-supported -->
<!-- keywords: capability boundary, exclusions, unsupported action -->

- Editing a reviewed branch in place.
- Switching branches automatically from review or Canvas.

## Known conflicts
<!-- chunkId: workspace-branch-context-known-conflicts -->
<!-- keywords: documentation drift, first edit copy, materialization -->

- "The first edit on a reviewed branch copies the plan into the checkout." It does not. Import is an explicit action.

## Related pages

See [[workspace-branch-context-reference]] and [[workspace-index]].
