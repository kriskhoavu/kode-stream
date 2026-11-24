---
slug: e2e-testing-workspace-branch-review-checkout
title: Review, Import, And Activate A Branch
pageType: HOW_TO
topics: branch review, checkout, import, canvas, knowledge
summary: Provider-neutral browser journey for pinned review, explicit plan import, guarded checkout, and cross-page synchronization.
sourceRef: |
  PM-038 (plan retired) | Browser journey
lastVerified: never
---

## Goal And Preconditions
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-goal -->
<!-- keywords: disposable workspace, branch fixtures, marker, safe writes -->

[open] Can the current build complete this journey? It has not run against a supplied runtime.

This journey checks [[workspace-branch-review-workflows]] in a non-production Local environment. Supply a base URL, an authentication reference and a disposable Git workspace:

- Branch A is the checkout and holds a known, reversible uncommitted marker.
- Branch B holds a unique committed structured plan.

Import and checkout are approved safe writes for this fixture only. Run them as separate phases. Remove or checkpoint the imported target before switching to branch B, because Git must not overwrite that untracked path. Follow the [[e2e-testing-index]] execution policy.

## Review Without Changing Checkout
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-review -->
<!-- keywords: read-only review, pinned commit, stale bookmark -->

Read-only steps:

1. Open Workstream. Assert branch A is labeled **Checkout** and its plans show.
2. Choose **Review branch** and select branch B. Wait for **Branch Review** and **Read-only committed snapshot**.
3. Assert branch B and a short commit show while the checkout stays branch A.
4. Open the unique plan and one file. Assert the content shows and edit, status, terminal, AI and verification controls are absent.
5. If a stale `/items/{snapshot-id}` bookmark is supplied, open it. Assert only the review-only error and **Back** remain.
6. Choose **Back**, visit Canvas and Knowledge, then return to review. Assert both pages still use branch A.

Capture the checkout label, review context and file preview.

## Import Into Checkout
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-import -->
<!-- keywords: import confirmation, marker unchanged, conflict -->

1. Select the plan and choose **Import selected plan**.
2. Confirm the preview names branch B, its commit, branch A and the plan.
3. Wait for the item route. Assert the plan exists in branch A's working tree.
4. Assert Git still reports branch A and the marker is unchanged.

If the target exists or the source moved, assert a visible conflict and stop. Never delete or overwrite to force it.

## Activate And Synchronize
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-activate -->
<!-- keywords: guarded checkout, dirty confirmation, synchronization -->

1. Reset to the switch phase, return to Branch Review and choose **Switch workspace to this branch**.
2. When the dirty-tree prompt appears, check its branch and change context, then confirm.
3. Wait for Workstream. Assert branch B is labeled **Checkout** with its plans.
4. Open Canvas and Knowledge. Assert neither shows branch A or a remembered snapshot.

A switch right after import may report that the imported path would be overwritten. That is the expected Git refusal. Stop at the first failed branch assertion.

## Cleanup, Evidence, And Limits
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-cleanup -->
<!-- keywords: cleanup, evidence, result, coverage limit -->

Capture Workstream before and after the switch, the review banner, the import result and the synchronized Canvas. Restore the disposable workspace through its normal cleanup and stop only processes this run started. Record passed, failed, blocked and not-run sections in the ticket-local result.

The journey does not cover provider snapshots, merge or overwrite, arbitrary repository import or Cloud Agent checkout. See [[workspace-branch-context-reference]].
