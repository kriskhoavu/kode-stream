---
slug: workspace-branch-context
title: Checkout And Branch Review Concepts
pageType: CONCEPT
topics: checkout, branch review, snapshot, operational context, import
summary: Kode Stream uses the Git checkout for every operational page and isolates other branches in commit-pinned read-only review.
sourceRef: |
  PM-013 (plan retired) | Superseded snapshot materialization
  plans/platform/PM-038/README.md
lastVerified: never
---

## One Operational Context
<!-- chunkId: workspace-branch-context-operational-context -->
<!-- keywords: checkout, operational context, working tree, pages -->

[open] Does the current implementation still satisfy the one operational context behavior described below? This section has not yet been reverified.

The current Git checkout is the only operational context for Workstream, Item Workspace, Canvas, Knowledge, editing,
terminal launch, and verification. These surfaces resolve the checkout from Git instead of trusting a remembered branch
selection or a route parameter. When Git changes outside Kode Stream, focus and visibility refreshes invalidate the
operational projections. Canvas-specific ownership remains documented in [[workspace-terminal-canvas]].

## Branch Review Boundary
<!-- chunkId: workspace-branch-context-review-boundary -->
<!-- keywords: branch review, pinned commit, read-only, snapshot -->

[open] Does the current implementation still satisfy the branch review boundary behavior described below? This section has not yet been reverified.

Branch Review is a separate route for a non-checkout local branch. The server resolves the branch to a commit, scans
committed Git-tree content, and returns read-only plans and files. The review branch and commit are visible beside the
unchanged checkout. Review cannot edit files or metadata, move statuses, launch terminals, run verification, or mutate
Git. See [[workspace-branch-review-workflows]] for the user journey.

## Explicit Import Instead Of First-Edit Copy
<!-- chunkId: workspace-branch-context-explicit-import -->
<!-- keywords: import, structured plan, conflict, source commit -->

[open] Does the current implementation still satisfy the explicit import instead of first-edit copy behavior described below? This section has not yet been reverified.

Cross-branch copying remains useful but is an explicit structured-plan import. Before copying, Kode Stream revalidates
the reviewed source commit and refuses an existing target path, unsafe path, documentation-only item, or moved branch.
The checkout stays unchanged and unrelated working-tree modifications are preserved. Exact contracts and failure codes
are in [[workspace-branch-context-reference]].

## Why Snapshot State Is Not Global
<!-- chunkId: workspace-branch-context-no-global-snapshot -->
<!-- keywords: snapshot state, conflict, authoritative branch, design decision -->

[open] Does the current implementation still satisfy the why snapshot state is not global behavior described below? This section has not yet been reverified.

A global selected snapshot made pages disagree about which branch was active while mutable content still came from the
working tree. Separating review from operation gives each route one clear promise: operational pages may act on the
checkout, and review may only read a pinned commit. Branch-scoped indexes and Canvas layouts are retained as useful
derived state, but they no longer drive application-wide navigation.

[legacy] PM-013's first-edit materialization workflow was removed. PM-038 replaced it with explicit reviewed-plan import and kept non-checkout content read-only.

## Not supported
<!-- chunkId: workspace-branch-context-not-supported -->
<!-- keywords: capability boundary, exclusions, unsupported action -->

[open] Which additional unsupported operations should this page name after implementation verification?

## Known conflicts
<!-- chunkId: workspace-branch-context-known-conflicts -->
<!-- keywords: documentation drift, verification gap, competing description -->

[open] Do the retained plan descriptions disagree with current behavior? Verify the sections above before relying on them.

## Related pages

See [[workspace-branch-context-reference]], [[workspace-index]], and [[e2e-testing-index]].
