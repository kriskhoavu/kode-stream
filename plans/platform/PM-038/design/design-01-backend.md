# Backend Design: Checkout Context And Branch Review

## Overview

Split working-tree loading from snapshot review while reusing the scanner and branch-aware item index. Operational
services resolve the checkout themselves. Review responses are commit-pinned and mutation services reject snapshot
items. Canvas ensures checkout plans are indexed before first placement, and Knowledge records the checkout branch and
commit that produced its index.

## API Contract

| Method | Endpoint                                   | Request                                    | Response                       |
|--------|--------------------------------------------|--------------------------------------------|--------------------------------|
| POST   | `/api/workspaces/{id}/workstream/checkout` | optional `force`                           | checkout branch load result    |
| POST   | `/api/workspaces/{id}/reviews/branch`      | `branch`, optional `force`                 | read-only branch review result |
| POST   | `/api/workspaces/{id}/reviews/import`      | `sourceBranch`, `expectedCommit`, `itemId` | imported checkout item         |

The legacy Workstream branch endpoint accepts the current checkout during one compatibility cycle. A different branch
returns `409 branch_review_required`. Snapshot writes return `409 snapshot_read_only` even when a legacy
`materializeConfirmed` field is present.

## Review And Import Rules

- Resolve a review branch to its full ref and commit before scanning.
- Review files use Git-tree reads and never fall back to the filesystem.
- Import only structured item roots under a configured source.
- Re-resolve the source branch and require `expectedCommit` immediately before copying.
- Reject any existing target path; do not merge, overwrite, or rename.
- Preserve unrelated dirty checkout content and never call reset, clean, stash, checkout, or switch.
- Refresh the checkout index after copying and return the imported checkout item.

## Operational Refresh

- A shared checkout loader resolves the current branch, working-tree hash, and source configuration before reusing an
  index entry.
- Workstream and Canvas call the loader rather than reading an unverified branch cache.
- Canvas derives layout branch identity from the loader result.
- Knowledge stores checkout branch and commit metadata and rebuilds when either differs.
- A completed Git switch reports refresh warnings without pretending the switch failed or rolling Git back.

## Compatibility

- Preserve branch scan rows, Canvas layouts, and session records.
- Stop reading or writing `lastSelectedBranch` for navigation; retain the serialized field for compatibility.
- A legacy Canvas `branchKey` is accepted only when it matches the checkout; otherwise return
  `canvas_branch_mismatch`.
