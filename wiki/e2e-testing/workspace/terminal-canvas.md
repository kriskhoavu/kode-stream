---
slug: e2e-testing-workspace-terminal-canvas
title: Operate The Focused Terminal Canvas
pageType: HOW_TO
topics: canvas, terminal, branch, verification, accessibility
summary: Browser journey for arranging Canvas nodes, launching safely, and observing verification staleness.
sourceRef: |
  PM-037 (plan retired) | Browser journey
  PM-042 (plan retired) | Session Focus journey
  PM-042 (plan retired) | Visible actions and assertions
  PM-042 (plan retired) | Historical result
lastVerified: never
---

## Goal And Preconditions
<!-- chunkId: e2e-testing-workspace-terminal-canvas-goal -->
<!-- keywords: local workspace, branch fixtures, provider, smoke verification -->

[open] Can the current build complete this journey? It has not run against a supplied runtime.

This journey checks [[workspace-terminal-canvas-workflows]] in a non-production Local environment. Supply a base URL, an authentication reference, a writable Git workspace with an indexed plan, matching and mismatching branch fixtures, a test-safe terminal provider, a deterministic smoke verification and a reversible change that alters the fingerprint. Follow the [[e2e-testing-index]] execution policy. Never record terminal content.

## Arrange And Restore
<!-- chunkId: e2e-testing-workspace-terminal-canvas-arrange -->
<!-- keywords: drag, saved positions, silent placement, reload -->

1. Open **Agent Space**, choose the **Canvas** view and wait for **Fit view**.
2. Move the workspace node. Wait for **Saved**. Assert the plan did not move.
3. Move the plan and reload. Assert both positions restore with current labels and Git state.
4. Run the supplied plan scan and refresh. Assert the new plan appears without moving saved nodes or any notice.
5. After a launch, move the session node and reload. Assert placement and lifecycle restore.

Placement changes are safe writes and never touch repository content.

## Launch And Branch Safety
<!-- chunkId: e2e-testing-workspace-terminal-canvas-launch -->
<!-- keywords: launch terminal, idempotency, mismatch, interrupted -->

1. Select the plan. Assert **Launch terminal**, **Run smoke verification** and **Open full view** show as allowed. Choose **Launch terminal**.
2. Assert the new session node is placed and expanded, with its terminal inside Canvas and the Workbench closed.
3. While **Launching…** shows, activate again if enabled. Assert at most one record and one process.
4. Select another node and return. Assert the terminal state did not change and nothing relaunched. Collapse and expand with the top-right action. Assert terminal input and scroll never drag the node.
5. Close the terminal. Assert the node collapses without cancelling. Reopen it, then use **Cancel process**. Assert the placement remains.
6. Use the mismatch fixture. Assert **Checkout changed** with expected and current context, and no new session.
7. Restart the app. Assert the orphaned record shows as interrupted with no false reconnect.

Stop at the first failed assertion and clean up.

## Git, Verification, And Accessibility
<!-- chunkId: e2e-testing-workspace-terminal-canvas-verification -->
<!-- keywords: freshness, stale pass, keyboard move, reduced motion -->

1. Assert the workspace node shows branch, HEAD, clean, dirty or conflicted state, and changed-file count.
2. Run smoke verification. Assert result, freshness and short revisions show separately.
3. Apply the reversible change. Assert the pass turns stale and historical.
4. Use **Search Canvas nodes**, focus the plan and press an arrow key. Assert only it moves and **Saved** is announced.
5. Remove the session placement without cancelling and refresh. Assert it stays hidden. Cancel a reset preview. Revisit with reduced motion on.

## Focus And Session Surfaces
<!-- chunkId: e2e-testing-workspace-terminal-canvas-session-focus -->
<!-- keywords: focus toggle, sessions view, checkout changes, open on board -->

1. Start live sessions on two plans and turn on **Focus**.
2. Assert only active plans, their sessions and their edges remain, and dragging is off. Turn Focus off and assert the saved positions are unchanged.
3. Open the **Sessions** view. Assert records are grouped by plan for the checkout. Switch between Terminal and Changes, and open one file diff.
4. Stop the selected session through its confirmation. Assert the live-only filter relaxes and the record stays visible.
5. Choose **Open on board**. Assert the existing session node is selected.

## Cleanup And Result
<!-- chunkId: e2e-testing-workspace-terminal-canvas-cleanup -->
<!-- keywords: cancel, revert, evidence, result -->

Cancel created processes, revert the reversible change and restore the original branch through guarded controls. Record passed, failed, blocked and not-run sections with safe evidence references in the ticket-local result.

The journey covers Local content and execution only. See [[workspace-terminal-canvas]] and [[workspace-session-focus]].
