---
slug: platform-branch-context-reference
title: Checkout And Branch Review Reference
pageType: REFERENCE
roles: DEVELOPER, TESTER, BA
topics: checkout loader, branch review API, import API, canvas, knowledge
summary: API, synchronization, compatibility, and safety contracts for checkout-scoped operation and commit-pinned review.
sourceRef: plans/platform/PM-038/design/design-01-backend.md
sourceRef: plans/platform/PM-038/design/design-02-frontend.md
sourceCount: 1
lastTicket: PM-038
---

## Context Matrix
<!-- chunkId: platform-branch-context-reference-context-matrix -->
<!-- keywords: route, source, editable, execution, branch -->

| Surface                        | Branch source                 | Content source  | Mutable or executable   |
|--------------------------------|-------------------------------|-----------------|-------------------------|
| Workstream, item, Canvas, wiki | Current Git checkout          | Working tree    | According to capability |
| Branch Review                  | Requested non-checkout branch | Pinned Git tree | Never                   |
| Agentless Remote Snapshot      | Provider snapshot contract    | Remote provider | Provider-dependent      |

Local Branch Review does not replace Agentless Remote Snapshot provider behavior. Canvas continues to store layouts by
workspace and checkout branch; see [[platform-terminal-canvas-reference]].

## API Contracts
<!-- chunkId: platform-branch-context-reference-apis -->
<!-- keywords: endpoint, checkout, review, import, commit -->

| Method | Endpoint                                   | Required request                                                     | Result                     |
|--------|--------------------------------------------|----------------------------------------------------------------------|----------------------------|
| POST   | `/api/workspaces/{id}/workstream/checkout` | optional `force`                                                     | Working-tree branch result |
| POST   | `/api/workspaces/{id}/reviews/branch`      | `branch`, optional `force`                                           | Read-only pinned snapshot  |
| POST   | `/api/workspaces/{id}/reviews/import`      | `sourceBranch`, `expectedCommit`, `expectedCheckoutBranch`, `itemId` | Imported checkout item     |

Snapshot file trees and content are served by the normal item read endpoints because indexed snapshot items retain
their branch ref and commit. Every snapshot tree operation uses the immutable commit SHA; the branch ref is metadata
only. Snapshot verification selection and discovered automation specs also come from `plan.yaml` at that commit;
snapshot reads never consult checkout or external automation working trees. Mutation endpoints reject those items even
if a legacy materialization flag is supplied.

## Synchronization Rules
<!-- chunkId: platform-branch-context-reference-synchronization -->
<!-- keywords: canvas, knowledge, index, focus, invalidation -->

The checkout loader compares branch commit, source configuration, and a working-tree hash before reusing an index.
Canvas invokes it before resolving the default layout and rejects a legacy `branchKey` that differs from checkout.
Knowledge records the checkout branch and commit that produced its index and rebuilds after either changes. The frontend
reloads branch inventory on focus or visibility return and increments its content refresh key when checkout changed.
In-app checkout switching and reviewed-plan import share one workspace-scoped mutation lock. Import holds it from source
and destination revalidation through atomic publication and checkout index refresh, so switching cannot interleave.

## Conflict And Compatibility Codes
<!-- chunkId: platform-branch-context-reference-conflicts -->
<!-- keywords: conflict code, snapshot read-only, branch mismatch, moved commit -->

| Condition                                     | Status | Code or behavior               |
|-----------------------------------------------|--------|--------------------------------|
| Legacy operational load requests other branch | 409    | `branch_review_required`       |
| Snapshot mutation                             | 409    | `snapshot_read_only`           |
| Legacy Canvas branch differs from checkout    | 409    | `canvas_branch_mismatch`       |
| Review requests current checkout              | 409    | Review must exit to Workstream |
| Source branch moved before import             | 409    | `review_commit_moved`          |
| Destination checkout changed before import    | 409    | `review_checkout_moved`        |
| Target item root already exists               | 409    | `import_target_exists`         |

`lastSelectedBranch` remains serialized for compatibility but is not read or written for navigation. Existing branch
scan rows and Canvas layouts remain reusable derived state.

## Safety Invariants
<!-- chunkId: platform-branch-context-reference-safety -->
<!-- keywords: safe path, symlink, reset, overwrite, dirty tree -->

Review reads Git objects by immutable commit SHA only. Import accepts structured plans within configured sources,
rejects traversal and symlink escape, revalidates both the expected source commit and destination checkout, and rejects
the complete target item root when any entry already occupies it. Import writes every file into a sibling temporary
directory, removes staging on failure, and publishes only the complete plan through an atomic rename. Review and import
never reset, clean, stash, checkout, or switch. Only the explicit guarded switch action may change the checkout, and it
is serialized with import for the same workspace.
