---
slug: platform-branch-review-workflows
title: Review, Import, Or Activate Another Branch
pageType: HOW_TO
roles: USER, BA, TESTER
topics: branch review, checkout, import, branch switch, read-only
summary: How to inspect another branch safely, import one plan, or make the branch operational through guarded checkout.
sourceRef: plans/platform/PM-038/scenario/scenario-00-overview.md
sourceRef: plans/platform/PM-038/design/design-02-frontend.md
sourceCount: 1
lastTicket: PM-038
---

## Review Another Branch
<!-- chunkId: platform-branch-review-workflows-review -->
<!-- keywords: review branch, committed plans, pinned commit, checkout -->

From Workstream, choose **Review branch**, then select a branch other than the checkout. Wait for **Branch Review** and
**Read-only committed snapshot**. The header shows both the checkout and the reviewed branch with its short commit.
Choose a plan and committed file to inspect it. Editing, status movement, terminal, AI, and verification controls are
intentionally absent. Returning to Canvas or Knowledge still shows checkout content; see
[[platform-branch-context-concept]].

## Import One Structured Plan
<!-- chunkId: platform-branch-review-workflows-import -->
<!-- keywords: import plan, confirmation, destination, conflict -->

Select a structured plan and choose **Import selected plan**. Confirm the source branch, pinned commit, destination
checkout, and plan identity. A successful import copies the complete plan directory into the checkout working tree,
refreshes its index, and opens the operational item. If the branch moved or the target exists, review remains open and
shows the conflict; Kode Stream never merges or overwrites automatically.

## Make The Reviewed Branch Operational
<!-- chunkId: platform-branch-review-workflows-activate -->
<!-- keywords: switch workspace, dirty tree, confirmation, refresh -->

Choose **Switch workspace to this branch** only when the reviewed branch should become the operational checkout. The
normal guarded switch asks for confirmation when local changes need acknowledgement. After Git switches successfully,
Workstream opens on the new checkout and Canvas, Knowledge, and item routes refresh from that same context. Kode Stream
does not reset, clean, stash, or silently discard files.

## Handle Empty Or Stale Review State
<!-- chunkId: platform-branch-review-workflows-recovery -->
<!-- keywords: no plans, stale commit, current checkout, recovery -->

**No plans on this branch** means the pinned commit has no indexed items under configured sources. A branch that is now
the checkout should be used through Workstream instead of review. Refresh the snapshot before import when needed; if the
source ref moved after loading, reload review and inspect the new commit before trying again.
