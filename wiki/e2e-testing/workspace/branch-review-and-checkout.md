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
<!-- keywords: goal, disposable workspace, branches, marker, plan -->

[open] Does the current implementation still satisfy the goal and preconditions behavior described below? This section has not yet been reverified.

Validate [[workspace-branch-review-workflows]] in a non-production Local environment. Supply a base URL, authentication
reference, and an isolated disposable Git workspace. Branch A is the checkout and contains a known reversible
uncommitted marker; branch B contains a unique committed structured plan. Import and checkout are approved safe writes
for this fixture only. Run import and checkout as isolated fixture phases: checkpoint or remove the imported target
before switching to branch B, because Git must not overwrite that untracked path.

Use Playwright MCP in a fresh context. Keep credentials, terminal content, and unrelated repository data out of
evidence. Chrome DevTools may diagnose a failed Playwright step but is not the execution provider.

## Review Without Changing Checkout
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-review -->
<!-- keywords: workstream, branch review, read-only, commit, checkout -->

[open] Does the current implementation still satisfy the review without changing checkout behavior described below? This section has not yet been reverified.

1. Open Workstream and wait for the board; assert branch A is labeled **Checkout** and its plans are visible.
2. Choose **Review branch**, select branch B, and wait for **Branch Review** and **Read-only committed snapshot**.
3. Assert branch B and a short commit appear while checkout remains branch A.
4. Open the unique plan and one file; assert committed content is visible and editing, status, terminal, AI, and
   verification controls are absent.
5. When a stale `/items/{snapshot-id}` bookmark is supplied, open it and wait for item loading to finish. Assert only
   the review-only error and Back action remain; plan, file, Git, AI, verification, and mutation controls are absent.
6. Use **Back**, visit Canvas and Knowledge, then return to the review URL; assert both operational pages still use
   branch A.

These steps are read-only. Capture the checkout label, review context, and committed file preview.

## Import Into Checkout
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-import -->
<!-- keywords: import, confirmation, branch A, marker, conflict -->

[open] Does the current implementation still satisfy the import into checkout behavior described below? This section has not yet been reverified.

1. Select the unique structured plan and choose **Import selected plan**.
2. Confirm the preview names branch B, its pinned commit, branch A, and the selected plan.
3. Wait for the operational item route and assert the plan exists in branch A working-tree content.
4. Assert Git still reports branch A and the known uncommitted marker remains unchanged.

Import is an isolated safe write. If the destination exists or the source moved, assert a visible conflict and stop this
section; do not delete or overwrite content to force the journey through.

## Activate And Synchronize
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-activate -->
<!-- keywords: guarded checkout, dirty confirmation, canvas, knowledge, synchronization -->

[open] Does the current implementation still satisfy the activate and synchronize behavior described below? This section has not yet been reverified.

1. Reset to the isolated switch phase, return to Branch Review, and choose **Switch workspace to this branch**.
2. When the guarded dirty-tree prompt appears, verify its branch and local-change context, then confirm for the fixture.
3. Wait for Workstream to reload and assert branch B is now labeled **Checkout** with branch B plans.
4. Open Canvas and wait for its checkout label and plan nodes; open Knowledge and wait for its index.
5. Assert neither page silently shows branch A or a remembered snapshot selection.

The checkout is an approved safe write. A direct switch after import may report that the imported path would be
overwritten; that is the expected fail-closed Git outcome, not a reason to force checkout. Stop on the first failed
branch assertion because later actions would target an ambiguous repository context.

## Cleanup, Evidence, And Limits
<!-- chunkId: e2e-testing-workspace-branch-review-checkout-cleanup -->
<!-- keywords: cleanup, evidence, result, playwright, limitation -->

[open] Does the current implementation still satisfy the cleanup, evidence, and limits behavior described below? This section has not yet been reverified.

Record Workstream before and after switching, the review banner, import confirmation/result, and synchronized Canvas
state. Restore or remove the disposable workspace through its normal test cleanup; stop only processes started for the
run. Record passed, failed, blocked, and not-run sections in the ticket-local result.

PM-038 introduces this Local branch journey. It does not cover remote provider snapshots, merge or overwrite behavior,
arbitrary repository import, or Cloud Agent checkout. See [[workspace-branch-context-reference]].
