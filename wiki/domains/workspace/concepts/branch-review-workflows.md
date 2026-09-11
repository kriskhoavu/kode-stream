---
slug: workspace-branch-review-workflows
title: Review, Import, Or Activate Another Branch
pageType: HOW_TO
topics: branch review, checkout, import, branch switch, read-only
summary: How to inspect another branch safely, import one plan, or make the branch operational through guarded checkout.
sourceRef: |
  plans/platform/PM-038/scenario/scenario-00-overview.md
  plans/platform/PM-038/design/design-02-frontend.md
lastVerified: never
---

## Review Another Branch
<!-- chunkId: workspace-branch-review-workflows-review -->
<!-- keywords: review branch, committed plans, pinned commit, checkout -->

[open] Does the current implementation still satisfy the review another branch behavior described below? This section has not yet been reverified.

From Workstream, choose **Review branch**, then select a branch other than the checkout. Wait for **Branch Review** and
**Read-only committed snapshot**. The header shows both the checkout and the reviewed branch with its short commit.
Choose a plan and committed file to inspect it. Editing, status movement, terminal, AI, and verification controls are
intentionally absent. Returning to Canvas or Knowledge still shows checkout content; see
[[workspace-branch-context]].

## Import One Structured Plan
<!-- chunkId: workspace-branch-review-workflows-import -->
<!-- keywords: import plan, confirmation, destination, conflict -->

[open] Does the current implementation still satisfy the import one structured plan behavior described below? This section has not yet been reverified.

Select a structured plan and choose **Import selected plan**. Confirm the source branch, pinned commit, destination
checkout, and plan identity. A successful import copies the complete plan directory into the checkout working tree,
refreshes its index, and opens the operational item. If the branch moved or the target exists, review remains open and
shows the conflict; Kode Stream never merges or overwrites automatically.

## Make The Reviewed Branch Operational
<!-- chunkId: workspace-branch-review-workflows-activate -->
<!-- keywords: switch workspace, dirty tree, confirmation, refresh -->

[open] Does the current implementation still satisfy the make the reviewed branch operational behavior described below? This section has not yet been reverified.

Choose the **Checkout** branch picker only when the reviewed branch should become the operational checkout. For local
tracked or untracked content, choose **Move changes and switch** when it is available, or enter a stash message and
choose **Stash and switch**. After Git switches successfully,
Workstream opens on the new checkout and Canvas, Knowledge, and item routes refresh from that same context. Kode Stream
does not reset, clean, or silently discard files.

## Handle Empty Or Stale Review State
<!-- chunkId: workspace-branch-review-workflows-recovery -->
<!-- keywords: no plans, stale commit, current checkout, recovery -->

[open] Does the current implementation still satisfy the handle empty or stale review state behavior described below? This section has not yet been reverified.

**No plans on this branch** means the pinned commit has no indexed items under configured sources. A branch that is now
the checkout should be used through Workstream instead of review. Refresh the snapshot before import when needed; if the
source ref moved after loading, reload review and inspect the new commit before trying again.

## Not supported
<!-- chunkId: workspace-branch-review-workflows-not-supported -->
<!-- keywords: capability boundary, exclusions, unsupported action -->

[open] Which additional unsupported operations should this page name after implementation verification?

## Known conflicts
<!-- chunkId: workspace-branch-review-workflows-known-conflicts -->
<!-- keywords: documentation drift, verification gap, competing description -->

[open] Do the retained plan descriptions disagree with current behavior? Verify the sections above before relying on them.

## Related pages

See [[workspace-branch-context-reference]], [[workspace-index]], and [[e2e-testing-index]].
