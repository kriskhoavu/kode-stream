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
| GET    | `/api/items/{id}/files`                    | `expectedCommit` for snapshots                                       | Commit-pinned file tree    |
| GET    | `/api/items/{id}/files/{fileId}`           | `expectedCommit` for snapshots                                       | Commit-pinned file content |
| GET    | `/api/items/{id}/diff`                     | Working-tree item ID                                                 | Checkout diff              |
| GET    | `/api/items/{id}/content-search`           | Working-tree item ID and query                                       | Checkout content matches   |

Snapshot file trees and content are served by the normal item read endpoints because indexed snapshot items retain
their branch ref and commit. Every snapshot tree operation uses the immutable commit SHA; the branch ref is metadata
only. Snapshot file endpoints require the displayed commit as `expectedCommit`, reject a missing or mismatching value
with `409 review_commit_moved`, and then read through the validated item's copied immutable SHA. Snapshot verification
selection and discovered automation specs also come from `plan.yaml` at that commit;
snapshot reads never consult checkout or external automation working trees. Mutation endpoints reject those items even
if a legacy materialization flag is supplied. Diff and item content search have no snapshot mode: they reject a
snapshot ID with `409 snapshot_review_only` before invoking checkout-backed readers.

## Synchronization Rules
<!-- chunkId: platform-branch-context-reference-synchronization -->
<!-- keywords: canvas, knowledge, index, focus, invalidation -->

The checkout loader compares branch commit, source configuration, and a working-tree hash before reusing an index.
Canvas invokes it before resolving the default layout and rejects a legacy `branchKey` that differs from checkout.
Knowledge records the checkout branch and commit that produced its index and rebuilds after either changes. The frontend
reloads branch inventory on focus or visibility return and increments its content refresh key when checkout changed.
In-app checkout switching and reviewed-plan import share one workspace-scoped mutation lock. Import holds it from source
and destination revalidation through atomic publication and checkout index refresh, so switching cannot interleave.
Operational checkout loads hold that lock through fingerprinting, scanning, and branch index replacement, preventing a
scan started before import from overwriting the post-import index. Branch Review also holds the lock across checkout
validation, commit-pinned scanning, and branch index replacement. A refresh therefore cannot publish snapshot rows for
a branch that becomes operational during the scan. The review UI disables switching while its initial load or refresh
is pending. Each review load also has a monotonically increasing identity bound to its workspace, requested branch,
and checkout context. Only the current request may publish review data, errors, plan selection, or loading completion;
superseded responses cannot make actions target a branch different from the route and selector. The file tree and
preview are keyed by both selected item ID and reviewed commit. If refresh advances a branch while the stable item ID
remains unchanged, the frontend clears and reloads those files so the header, plan metadata, and preview stay on one
commit. The commit is part of each request URL, so in-flight GET deduplication cannot cross commits. Automatic and
manual file reads share a request generation; refresh invalidates older generations and late responses cannot replace
newer preview content or loading state.

Snapshot rows may remain in the shared derived index so Branch Review can reuse branch caches, but ordinary index
queries exclude them by default. Only the branch-scoped review query opts in. Operational item listing, workspace
consumers, and global search therefore expose current-checkout rows only, while storage synchronization preserves the
review cache without making it operational.

## Conflict And Compatibility Codes
<!-- chunkId: platform-branch-context-reference-conflicts -->
<!-- keywords: conflict code, snapshot read-only, branch mismatch, moved commit -->

| Condition                                      | Status | Code or behavior               |
|------------------------------------------------|--------|--------------------------------|
| Legacy operational load requests other branch  | 409    | `branch_review_required`       |
| Snapshot mutation                              | 409    | `snapshot_read_only`           |
| Snapshot diff or item content search           | 409    | `snapshot_review_only`         |
| Legacy Canvas branch differs from checkout     | 409    | `canvas_branch_mismatch`       |
| Review requests current checkout               | 409    | Review must exit to Workstream |
| Import item belongs to another workspace       | 404    | Item not found                 |
| Source branch moved before import              | 409    | `review_commit_moved`          |
| Snapshot file expected commit missing or stale | 409    | `review_commit_moved`          |
| Destination checkout changed before import     | 409    | `review_checkout_moved`        |
| Target item root already exists                | 409    | `import_target_exists`         |

`lastSelectedBranch` remains serialized for compatibility but is not read or written for navigation. Existing branch
scan rows and Canvas layouts remain reusable derived state.

## Safety Invariants
<!-- chunkId: platform-branch-context-reference-safety -->
<!-- keywords: safe path, symlink, reset, overwrite, dirty tree -->

Review reads Git objects by immutable commit SHA only. Import accepts structured plans within configured sources,
requires the item to belong to the workspace named by the route, rejects traversal and symlink escape, revalidates both
the expected source commit and destination checkout, and rejects the complete target item root when any entry already
occupies it. Import writes every file into a sibling temporary directory, removes staging on failure, and publishes only
the complete plan through an atomic rename. If checkout index refresh fails after publication, import removes the new
target before returning the error. Review and import never reset, clean, stash, checkout, or switch. Only the explicit
guarded switch action may change the checkout, and it is serialized with import and review refresh for the same
workspace. File-backed index replacement and deletion restore their complete prior in-memory state if persistence
fails, preventing a rolled-back filesystem import from remaining visible as a ghost item.
