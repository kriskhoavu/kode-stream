---
slug: platform-terminal-canvas-journey
title: Operate The Focused Terminal Canvas
pageType: HOW_TO
roles: TESTER, DEVELOPER
topics: canvas, terminal, branch, verification, accessibility
summary: Browser journey for arranging Canvas nodes, launching safely, and observing verification staleness.
sourceRef: plans/platform/PM-037/automation/scenario-01-orchestrate-plan-terminal.md
sourceCount: 1
lastTicket: PM-037
---

## Goal And Preconditions
<!-- chunkId: platform-terminal-canvas-journey-goal -->
<!-- keywords: goal, local workspace, branch, provider, verification -->

Validate [[platform-terminal-canvas-workflows]] in a non-production Local environment. Supply a base URL, authentication
reference, isolated writable Git workspace with an indexed plan, matching and mismatch branch fixtures, authenticated
test-safe terminal provider, deterministic smoke verification, and reversible fingerprint-relevant mutation.

Use Playwright MCP in a fresh context. Do not substitute another browser provider. Confirm safe writes before execution;
never record credentials or terminal content.

## Arrange And Restore
<!-- chunkId: platform-terminal-canvas-journey-arrange -->
<!-- keywords: arrange, restore, workspace, plan, session -->

1. Open **Canvas** for the supplied workspace and branch; wait for **Fit view**.
2. Move the workspace; wait for **Saved** and assert the plan stayed fixed.
3. Move the plan, reload, and assert both positions restore with current labels and Git state.
4. After launch, move the session, reload without restarting the backend, and assert its placement and lifecycle restore.

These placement operations are isolated safe writes and never mutate repository entities. Capture initial, moved, and
restored layouts without terminal content.

## Launch And Branch Safety
<!-- chunkId: platform-terminal-canvas-journey-launch -->
<!-- keywords: launch terminal, idempotency, mismatch, interrupted, process -->

1. Select the plan, confirm **Launch terminal**, **Run smoke verification**, and **Open full view** are present as
   capabilities allow, then choose **Launch terminal**.
2. Wait for the new session node to be placed and expanded; assert its interactive terminal is inside the Canvas and the
   right Workbench is closed.
3. While **Launching…** is visible, activate again only if enabled; assert at most one record and process.
4. Select away and return; assert the same terminal remains and no relaunch occurs. Interacting with terminal input and
   scrolling must not drag the session node.
5. Close the terminal and assert the node collapses without cancellation; reopen it, then use the explicit
   **Cancel process** control and assert the placement remains.
6. Use the supplied mismatch fixture, attempt the intentionally stale action, and assert **Checkout changed** appears
   with expected/current context and no new session.
7. Verify an application restart changes an orphaned running record to interrupted and expands into lifecycle detail
   without a false reconnect.

Branch manipulation and process launch are safe writes only in the supplied isolated fixture. Stop this section at the
first failed assertion and follow the documented cleanup.

## Git, Verification, And Accessibility
<!-- chunkId: platform-terminal-canvas-journey-verification -->
<!-- keywords: git, verification, stale, search, keyboard -->

1. Assert workspace branch, HEAD, clean/dirty/conflicted text, and changed-file count.
2. Run smoke verification from the plan and assert result, freshness, and abbreviated revisions are separate.
3. Apply the supplied reversible mutation; wait for refresh and assert the previous pass becomes stale and historical.
4. Use **Search Canvas nodes**, focus the plan, press an arrow key, and assert only it moves and **Saved** is announced.
5. Remove the session placement without cancellation, cancel a reset preview, and revisit with reduced motion enabled.

Capture current/stale verification and visible keyboard focus. Diagnostics may use Chrome DevTools only after Playwright
execution; Chrome is never the execution provider.

## Cleanup And Result
<!-- chunkId: platform-terminal-canvas-journey-cleanup -->
<!-- keywords: cleanup, cancel, revert, evidence, result -->

Cancel created processes through the normal control, revert the isolated mutation, and restore the original test branch
through guarded controls. Record passed, failed, blocked, and not-run sections plus safe evidence references in the
ticket-local latest result. The current PM-037 result is not run because runtime inputs were not supplied.

## Coverage Deltas And Limits
<!-- chunkId: platform-terminal-canvas-journey-limits -->
<!-- keywords: coverage, local, cloud agent, snapshot, limits -->

PM-037 introduces this journey. It covers only Local content and execution. Cloud Agent execution, Agentless Remote
Snapshot Canvas UX, groups, custom links, multiple canvases, and collaboration need separate capability delivery and
future journey deltas. See [[platform-terminal-canvas-concept]] and [[platform-terminal-canvas-reference]].
