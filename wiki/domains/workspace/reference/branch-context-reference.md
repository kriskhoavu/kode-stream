---
slug: workspace-branch-context-reference
title: Checkout And Branch Review Reference
pageType: REFERENCE
topics: checkout loader, branch review API, import API, e2e runbooks, mutation audit, canvas
summary: API, synchronization, read-boundary, audit, and safety contracts for checkout-scoped operation and commit-pinned review.
sourceRef: |
  PM-013 (plan retired) | Historical materialization contract
  PM-038 (plan retired) | Backend contracts
  PM-038 (plan retired) | Frontend contracts
lastVerified: never
---

## Context Matrix
<!-- chunkId: workspace-branch-context-reference-context-matrix -->
<!-- keywords: route, source, editable, execution -->

[open] Does the code still match this page? It is plan-derived and not yet checked.

| Surface                        | Branch source                 | Content source  | Mutable or executable   |
|--------------------------------|-------------------------------|-----------------|-------------------------|
| Workstream, item, Canvas, wiki | Current Git checkout          | Working tree    | According to capability |
| Branch Review                  | Requested non-checkout branch | Pinned Git tree | Never                   |
| Agentless Remote Snapshot      | Provider snapshot contract    | Remote provider | Provider-dependent      |

## API Contracts
<!-- chunkId: workspace-branch-context-reference-apis -->
<!-- keywords: endpoint, expectedCommit, snapshot item, request body -->

| Method | Endpoint                                   | Required request                                                     | Result                     |
|--------|--------------------------------------------|----------------------------------------------------------------------|----------------------------|
| POST   | `/api/workspaces/{id}/workstream/checkout` | optional `force`                                                     | Working-tree branch result |
| POST   | `/api/workspaces/{id}/reviews/branch`      | `branch`, optional `force`                                           | Read-only pinned snapshot  |
| POST   | `/api/workspaces/{id}/reviews/import`      | `sourceBranch`, `expectedCommit`, `expectedCheckoutBranch`, `itemId` | Imported checkout item     |
| GET    | `/api/items/{id}`                          | Working-tree item ID                                                 | Operational item detail    |
| GET    | `/api/items/{id}/files`                    | `expectedCommit` for snapshots                                       | Commit-pinned file tree    |
| GET    | `/api/items/{id}/files/{fileId}`           | `expectedCommit` for snapshots                                       | Commit-pinned file content |
| GET    | `/api/items/{id}/diff`                     | Working-tree item ID                                                 | Checkout diff              |
| GET    | `/api/items/{id}/content-search`           | Working-tree item ID and query                                       | Checkout content matches   |
| GET    | `/api/items/{id}/e2e-runbooks`             | Working-tree item ID                                                 | Reusable E2E coverage      |

- Snapshot items keep their branch ref and commit, so the normal item file endpoints serve them.
- Every snapshot tree read uses the commit SHA. The ref is metadata only.
- Snapshot file endpoints require the displayed commit as `expectedCommit`.
- Snapshot verification selection and automation specs come from `plan.yaml` at that commit, never from a working tree.
- Item detail, diff, content search and E2E runbook discovery have no snapshot mode. They reject a snapshot ID before any checkout reader runs.

## Synchronization Rules
<!-- chunkId: workspace-branch-context-reference-synchronization -->
<!-- keywords: mutation lock, index reuse, focus invalidation, request identity -->

- The checkout loader reuses an index only when branch commit, source configuration hash and working-tree hash match.
- Canvas calls the loader before it resolves the default layout.
- Knowledge records the branch and commit behind its index and rebuilds when either changes.
- The frontend reloads branch inventory on focus or visibility return.
- One workspace mutation lock serializes checkout switch, plan import, operational checkout loads and Branch Review scans. A scan cannot overwrite the index that an import just published.
- The review UI disables switching while a review load runs. Only the newest review request may publish its result.
- The review file tree and preview are keyed by item ID and commit. The commit is in each request URL, so request deduplication never crosses commits.
- Snapshot rows stay in the derived index, but ordinary queries exclude them. Only the review query includes them.
- Item Workspace renders operational controls only after working-tree detail succeeds. A stale snapshot bookmark shows a message and **Back**.

## Mutation Audit Semantics
<!-- chunkId: workspace-branch-context-reference-mutation-audit -->
<!-- keywords: snapshot mutation, blocked status, workspace history -->

A rejected snapshot file, metadata or status change is audited with workspace, item and path context from the raw index. It records `blocked`, not `failed`, and keeps the original error.

## Conflict Codes
<!-- chunkId: workspace-branch-context-reference-conflicts -->
<!-- keywords: snapshot read-only, branch mismatch, moved commit, http 409 -->

| Condition                                      | Status | Code or behavior               |
|------------------------------------------------|--------|--------------------------------|
| Operational load requests another branch       | 409    | `branch_review_required`       |
| Snapshot mutation                              | 409    | `snapshot_read_only`           |
| Snapshot detail, diff, search or E2E runbooks  | 409    | `snapshot_review_only`         |
| Canvas branch differs from checkout            | 409    | `canvas_branch_mismatch`       |
| Review requests current checkout               | 409    | Review must exit to Workstream |
| Import item belongs to another workspace       | 404    | Item not found                 |
| Source branch moved before import              | 409    | `review_commit_moved`          |
| Snapshot file expected commit missing or stale | 409    | `review_commit_moved`          |
| Destination checkout changed before import     | 409    | `review_checkout_moved`        |
| Target item root already exists                | 409    | `import_target_exists`         |

`lastSelectedBranch` is still serialized but is never read or written for navigation.

## Safety Invariants
<!-- chunkId: workspace-branch-context-reference-safety -->
<!-- keywords: safe path, symlink, atomic rename, staging, rollback -->

- Review reads Git objects by commit SHA only.
- Import accepts structured plans inside configured sources that belong to the route's workspace.
- Import rejects traversal, symlink escape and any existing entry in the target item root.
- Import stages files in a sibling temporary directory and publishes with one atomic rename.
- If the index refresh fails after publication, import removes the new target.
- Review and import never reset, clean, stash, checkout or switch.
- A failed file-backed index write restores the prior in-memory state, so no ghost item remains.

## Not supported
<!-- chunkId: workspace-branch-context-reference-not-supported -->
<!-- keywords: capability boundary, exclusions, materialization flags -->

- Mutating a snapshot item, even with a materialization flag.
- Reviewing the current checkout.

## Known conflicts
<!-- chunkId: workspace-branch-context-reference-known-conflicts -->
<!-- keywords: documentation drift, first edit copy, compatibility flags -->

- "Branch materialization flags still work for compatibility." They do not. Callers use Branch Review and import.

## Related pages

See [[workspace-branch-context]], [[workspace-terminal-canvas-reference]] and [[workspace-index]].
