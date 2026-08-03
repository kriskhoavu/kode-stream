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
| POST   | `/api/workspaces/{id}/reviews/branch`      | `branch`, optional `force`                                           | read-only branch review result |
| POST   | `/api/workspaces/{id}/reviews/import`      | `sourceBranch`, `expectedCommit`, `expectedCheckoutBranch`, `itemId` | imported checkout item         |

The legacy Workstream branch endpoint accepts the current checkout during one compatibility cycle. A different branch
returns `409 branch_review_required`. Snapshot writes return `409 snapshot_read_only` even when a legacy
`materializeConfirmed` field is present.

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

## Operational Refresh

- A shared checkout loader resolves the current branch, working-tree hash, and source configuration before reusing an
  index entry.
- Hold the workspace mutation lock across checkout fingerprinting, scanning, and branch index replacement so a scan
  started before import cannot replace the post-import index afterward.
- Workstream and Canvas call the loader rather than reading an unverified branch cache.
- Canvas derives layout branch identity from the loader result.
- Knowledge stores checkout branch and commit metadata and rebuilds when either differs.
- A completed Git switch reports refresh warnings without pretending the switch failed or rolling Git back.

## Compatibility

- Preserve branch scan rows, Canvas layouts, and session records.
- Stop reading or writing `lastSelectedBranch` for navigation; retain the serialized field for compatibility.
- A legacy Canvas `branchKey` is accepted only when it matches the checkout; otherwise return
  `canvas_branch_mismatch`.
