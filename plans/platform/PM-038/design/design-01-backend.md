# Backend Design: Checkout Context And Branch Review

## Overview

Split working-tree loading from snapshot review while reusing the scanner and branch-aware item index. Operational
services resolve the checkout themselves. Review responses are commit-pinned and mutation services reject snapshot
items. Canvas ensures checkout plans are indexed before first placement, and Knowledge records the checkout branch and
commit that produced its index.

## API Contract

| Method | Endpoint                                   | Request                                                              | Response                       |
|--------|--------------------------------------------|----------------------------------------------------------------------|--------------------------------|
| POST   | `/api/workspaces/{id}/workstream/checkout` | optional `force`                                                     | checkout branch load result    |
| POST   | `/api/workspaces/{id}/git/switch`          | `name`, optional `strategy`, optional `stashMessage`                 | Git result or switch decision  |
| POST   | `/api/workspaces/{id}/reviews/branch`      | `branch`, optional `force`                                           | read-only branch review result |
| POST   | `/api/workspaces/{id}/reviews/import`      | `sourceBranch`, `expectedCommit`, `expectedCheckoutBranch`, `itemId` | imported checkout item         |
| GET    | `/api/items/{id}`                          | working-tree item ID                                                 | operational item detail        |
| GET    | `/api/items/{id}/files`                    | `expectedCommit` required for snapshot items                         | commit-pinned file tree        |
| GET    | `/api/items/{id}/files/{fileId}`           | `expectedCommit` required for snapshot items                         | commit-pinned file content     |
| GET    | `/api/items/{id}/diff`                     | working-tree item ID                                                 | checkout diff                  |
| GET    | `/api/items/{id}/content-search`           | working-tree item ID and query                                       | checkout content matches       |
| GET    | `/api/items/{id}/e2e-runbooks`             | working-tree item ID                                                 | reusable E2E coverage          |

The legacy Workstream branch endpoint accepts the current checkout during one compatibility cycle. A different branch
returns `409 branch_review_required`. Snapshot writes return `409 snapshot_read_only` even when a legacy
`materializeConfirmed` field is present. Snapshot operational detail, diff, and content-search requests return
`409 snapshot_review_only`; detail cannot open a review identity inside Item Workspace, and the latter operations are
checkout-backed without a commit-pinned implementation.
Snapshot E2E runbook requests also return `409 snapshot_review_only`; the endpoint reads ticket-local and canonical
coverage without an expected-commit contract and is therefore operational-only.

## Review And Import Rules

- Resolve a review branch to its full ref and commit before scanning.
- Hold the workspace mutation lock across the current-checkout check, commit resolution, snapshot scan, and branch
  index replacement. A concurrent switch therefore completes first or waits; a review can never publish snapshot rows
  for a branch that has become the checkout.
- Use the resolved commit SHA for every review scan, metadata lookup, tree walk, file read, and import source read;
  retain the branch ref only as descriptive metadata.
- Read verification selection and discovered automation specs for snapshot items from `plan.yaml` at the resolved
  commit; never consult the checkout filesystem or an external automation working tree for snapshot metadata.
- Review files use Git-tree reads and never fall back to the filesystem.
- Shared index queries exclude `sourceMode: snapshot` by default. Only the branch-scoped review query opts in, so item
  listing, workspace consumers, and global search cannot route users from operational pages into reviewed items.
- Preserve snapshot rows when synchronizing or migrating storage, while keeping the operational query default in both
  file-backed and SQLite repositories.
- Reject snapshot diff and item content-search requests before invoking Git diff or filesystem content search.
- Reject snapshot item detail before reading a checkout README or returning metadata to operational consumers.
- Reject snapshot E2E runbook requests before reading checkout `plan.yaml`, `automation/`, result files, or canonical
  Knowledge coverage.
- Require `expectedCommit` on snapshot file-tree and file-content reads. Compare it with the indexed snapshot before
  reading and return `409 review_commit_moved` when missing or different; working-tree reads remain compatible without
  the parameter. Once validated, read through the copied item's immutable commit even if another refresh replaces the
  shared index concurrently.
- Require the snapshot item to belong to the workspace named by `/api/workspaces/{id}/reviews/import` before acquiring
  a mutation lock; return item-not-found semantics for cross-workspace item IDs.
- Import only structured item roots under a configured source.
- Serialize in-app checkout switching and import with one workspace-scoped mutation lock.
- Under that lock, re-resolve the source branch and require `expectedCommit`, then resolve the checkout and require
  `expectedCheckoutBranch`; return `409 review_checkout_moved` when it changed after confirmation.
- Reject the target item root when any filesystem entry already occupies it, including an empty directory or one with
  unrelated files; do not merge, overwrite, or rename.
- Build every source file in a sibling temporary directory, remove that directory on any failure, and atomically rename
  the complete directory to the target. Hold the mutation lock through publication and checkout index refresh.
- If checkout scanning or index persistence fails after publication, remove the newly published target before returning
  the error so a retry is not converted into `import_target_exists`. File-backed index mutations restore the complete
  prior in-memory state when persistence fails, preventing a rolled-back target from remaining as a ghost item.
- Preserve unrelated dirty checkout content and never call reset, clean, stash, checkout, or switch.
- Refresh the checkout index after copying and return the imported checkout item.
- Resolve audit identity for blocked mutations directly from the raw item index so snapshot file, metadata, and status
  failures remain visible in workspace-scoped history without weakening operational detail isolation. Classify these
  expected read-only rejections as blocked rather than failed audit outcomes.

## Operational Refresh

- A shared checkout loader resolves the current branch, working-tree hash, and source configuration before reusing an
  index entry.
- Hold the workspace mutation lock across checkout fingerprinting, scanning, and branch index replacement so a scan
  started before import cannot replace the post-import index afterward.
- Workstream and Canvas call the loader rather than reading an unverified branch cache.
- Canvas derives layout branch identity from the loader result.
- Knowledge stores checkout branch and commit metadata and rebuilds when either differs.
- A completed Git switch reports refresh warnings without pretending the switch failed or rolling Git back.

## Dirty Checkout Safety

- A dirty checkout returns `branch_switch_decision_required` with source, target, and `canCarryChanges` until the user chooses `carry` or `stash`.
- Carry is available only when the target cannot touch a tracked local path or collide with an untracked path.
- Stash uses `git stash push --include-untracked --message` under the workspace mutation lock.
- Conflicted worktrees reject both choices. A failed checkout after a stash preserves its stash reference for recovery.

## Compatibility

- Preserve branch scan rows, Canvas layouts, and session records.
- Stop reading or writing `lastSelectedBranch` for navigation; retain the serialized field for compatibility.
- A legacy Canvas `branchKey` is accepted only when it matches the checkout; otherwise return
  `canvas_branch_mismatch`.
