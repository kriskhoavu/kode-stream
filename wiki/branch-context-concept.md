---
slug: platform-branch-context-concept
title: Checkout And Branch Review Concepts
pageType: CONCEPT
roles: BA, DEVELOPER, TESTER, USER
topics: checkout, branch review, snapshot, operational context, import
summary: Kode Stream uses the Git checkout for every operational page and isolates other branches in commit-pinned read-only review.
sourceRef: plans/platform/PM-038/README.md
sourceCount: 1
lastTicket: PM-038
---

## One Operational Context
<!-- chunkId: platform-branch-context-concept-operational-context -->
<!-- keywords: checkout, operational context, working tree, pages -->

The current Git checkout is the only operational context for Workstream, Item Workspace, Canvas, Knowledge, editing,
terminal launch, and verification. These surfaces resolve the checkout from Git instead of trusting a remembered branch
selection or a route parameter. When Git changes outside Kode Stream, focus and visibility refreshes invalidate the
operational projections. Canvas-specific ownership remains documented in [[platform-terminal-canvas-concept]].

## Branch Review Boundary
<!-- chunkId: platform-branch-context-concept-review-boundary -->
<!-- keywords: branch review, pinned commit, read-only, snapshot -->

Branch Review is a separate route for a non-checkout local branch. The server resolves the branch to a commit, scans
committed Git-tree content, and returns read-only plans and files. The review branch and commit are visible beside the
unchanged checkout. Review cannot edit files or metadata, move statuses, launch terminals, run verification, or mutate
Git. See [[platform-branch-review-workflows]] for the user journey.

## Explicit Import Instead Of First-Edit Copy
<!-- chunkId: platform-branch-context-concept-explicit-import -->
<!-- keywords: import, structured plan, conflict, source commit -->

Cross-branch copying remains useful but is an explicit structured-plan import. Before copying, Kode Stream revalidates
the reviewed source commit and refuses an existing target path, unsafe path, documentation-only item, or moved branch.
The checkout stays unchanged and unrelated working-tree modifications are preserved. Exact contracts and failure codes
are in [[platform-branch-context-reference]].

## Why Snapshot State Is Not Global
<!-- chunkId: platform-branch-context-concept-no-global-snapshot -->
<!-- keywords: snapshot state, conflict, authoritative branch, design decision -->

A global selected snapshot made pages disagree about which branch was active while mutable content still came from the
working tree. Separating review from operation gives each route one clear promise: operational pages may act on the
checkout, and review may only read a pinned commit. Branch-scoped indexes and Canvas layouts are retained as useful
derived state, but they no longer drive application-wide navigation.
