---
slug: platform-branch-review-checkout-journey
title: Review, Import, And Activate A Branch
pageType: HOW_TO
roles: TESTER, DEVELOPER
topics: branch review, checkout, import, canvas, knowledge
summary: Provider-neutral browser journey for pinned review, explicit plan import, guarded checkout, and cross-page synchronization.
sourceRef: plans/platform/PM-038/automation/scenario-01-review-and-activate-branch.md
sourceCount: 1
lastTicket: PM-038
---

## Goal And Preconditions
<!-- chunkId: platform-branch-review-checkout-journey-goal -->
<!-- keywords: goal, disposable workspace, branches, marker, plan -->

Validate [[platform-branch-review-workflows]] in a non-production Local environment. Supply a base URL, authentication
reference, and an isolated disposable Git workspace. Branch A is the checkout and contains a known reversible
uncommitted marker; branch B contains a unique committed structured plan. Import and checkout are approved safe writes
for this fixture only. Run import and checkout as isolated fixture phases: checkpoint or remove the imported target
before switching to branch B, because Git must not overwrite that untracked path.

Use Playwright MCP in a fresh context. Keep credentials, terminal content, and unrelated repository data out of
evidence. Chrome DevTools may diagnose a failed Playwright step but is not the execution provider.

## Review Without Changing Checkout
<!-- chunkId: platform-branch-review-checkout-journey-review -->
<!-- keywords: workstream, branch review, read-only, commit, checkout -->

1. Open Workstream and wait for the board; assert branch A is labeled **Checkout** and its plans are visible.
2. Choose **Review branch**, select branch B, and wait for **Branch Review** and **Read-only committed snapshot**.
3. Assert branch B and a short commit appear while checkout remains branch A.
4. Open the unique plan and one file; assert committed content is visible and editing, status, terminal, AI, and
   verification controls are absent.
5. Visit Canvas and Knowledge, then return to the review URL; assert both operational pages still use branch A.

These steps are read-only. Capture the checkout label, review context, and committed file preview.

## Import Into Checkout
<!-- chunkId: platform-branch-review-checkout-journey-import -->
<!-- keywords: import, confirmation, branch A, marker, conflict -->

1. Select the unique structured plan and choose **Import selected plan**.
2. Confirm the preview names branch B, its pinned commit, branch A, and the selected plan.
3. Wait for the operational item route and assert the plan exists in branch A working-tree content.
4. Assert Git still reports branch A and the known uncommitted marker remains unchanged.

Import is an isolated safe write. If the destination exists or the source moved, assert a visible conflict and stop this
section; do not delete or overwrite content to force the journey through.

## Activate And Synchronize
<!-- chunkId: platform-branch-review-checkout-journey-activate -->
<!-- keywords: guarded checkout, dirty confirmation, canvas, knowledge, synchronization -->

1. Reset to the isolated switch phase, return to Branch Review, and choose **Switch workspace to this branch**.
2. When the guarded dirty-tree prompt appears, verify its branch and local-change context, then confirm for the fixture.
3. Wait for Workstream to reload and assert branch B is now labeled **Checkout** with branch B plans.
4. Open Canvas and wait for its checkout label and plan nodes; open Knowledge and wait for its index.
5. Assert neither page silently shows branch A or a remembered snapshot selection.

The checkout is an approved safe write. A direct switch after import may report that the imported path would be
overwritten; that is the expected fail-closed Git outcome, not a reason to force checkout. Stop on the first failed
branch assertion because later actions would target an ambiguous repository context.

## Cleanup, Evidence, And Limits
<!-- chunkId: platform-branch-review-checkout-journey-cleanup -->
<!-- keywords: cleanup, evidence, result, playwright, limitation -->

Record Workstream before and after switching, the review banner, import confirmation/result, and synchronized Canvas
state. Restore or remove the disposable workspace through its normal test cleanup; stop only processes started for the
run. Record passed, failed, blocked, and not-run sections in the ticket-local result.

PM-038 introduces this Local branch journey. It does not cover remote provider snapshots, merge or overwrite behavior,
arbitrary repository import, or Cloud Agent checkout. See [[platform-branch-context-reference]].
