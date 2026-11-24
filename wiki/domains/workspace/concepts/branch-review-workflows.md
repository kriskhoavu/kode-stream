---
slug: workspace-branch-review-workflows
title: Review, Import, Or Activate Another Branch
pageType: HOW_TO
topics: branch review, checkout, import, branch switch, read-only
summary: How to inspect another branch safely, import one plan, or make the branch operational through guarded checkout.
sourceRef: |
  PM-038 (plan retired) | Review and checkout scenarios
  PM-038 (plan retired) | Review user experience
lastVerified: never
---

## Review Another Branch
<!-- chunkId: workspace-branch-review-workflows-review -->
<!-- keywords: committed plans, pinned commit, read-only snapshot -->

[open] Does the code still match this page? It is plan-derived and not yet checked.

1. In Workstream, choose **Review branch**.
2. Select a branch other than the checkout.
3. Wait for **Branch Review** and **Read-only committed snapshot**.
4. Choose a plan and a committed file.

The header shows the checkout and the reviewed branch with its short commit. Editing, status, terminal, AI and verification controls are absent.

## Import One Structured Plan
<!-- chunkId: workspace-branch-review-workflows-import -->
<!-- keywords: confirmation, destination, conflict, plan copy -->

1. Select a structured plan and choose **Import selected plan**.
2. Confirm the source branch, pinned commit, destination checkout and plan.

The import copies the whole plan directory into the checkout, refreshes the index and opens the item. If the branch moved or the target exists, review stays open and shows the conflict. Kode Stream never merges or overwrites.

## Make The Reviewed Branch Operational
<!-- chunkId: workspace-branch-review-workflows-activate -->
<!-- keywords: switch workspace, dirty tree, stash, move changes -->

Use the **Checkout** branch picker. If the working tree has changes, choose **Move changes and switch**, or enter a stash message and choose **Stash and switch**. Workstream, Canvas, Knowledge and item routes then refresh from the new checkout. Kode Stream never resets, cleans or discards files.

## Handle Empty Or Stale Review State
<!-- chunkId: workspace-branch-review-workflows-recovery -->
<!-- keywords: no plans, stale commit, recovery, reload -->

- **No plans on this branch**: the commit has no indexed items under configured sources.
- If the branch is now the checkout, use Workstream instead.
- If the source branch moved, reload review and inspect the new commit before you import.

## Not supported
<!-- chunkId: workspace-branch-review-workflows-not-supported -->
<!-- keywords: capability boundary, merge, overwrite -->

- Merging or overwriting an existing plan on import.
- Editing or running anything in review.

## Known conflicts
<!-- chunkId: workspace-branch-review-workflows-known-conflicts -->
<!-- keywords: documentation drift, verification gap, plan-derived -->

None known. The page is not yet checked against the code.

## Related pages

See [[workspace-branch-context]] and [[workspace-branch-context-reference]].
