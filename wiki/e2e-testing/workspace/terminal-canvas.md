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
<!-- keywords: goal, local workspace, branch, provider, verification -->

[open] Does the current implementation still satisfy the goal and preconditions behavior described below? This section has not yet been reverified.

Validate [[workspace-terminal-canvas-workflows]] in a non-production Local environment. Supply a base URL, authentication
reference, isolated writable Git workspace with an indexed plan, matching and mismatch branch fixtures, authenticated
test-safe terminal provider, deterministic smoke verification, and reversible fingerprint-relevant mutation.

Use Playwright MCP in a fresh context. Do not substitute another browser provider. Confirm safe writes before execution;
never record credentials or terminal content.

## Arrange And Restore
<!-- chunkId: e2e-testing-workspace-terminal-canvas-arrange -->
<!-- keywords: arrange, restore, workspace, plan, session -->

[open] Does the current implementation still satisfy the arrange and restore behavior described below? This section has not yet been reverified.

1. Open **Canvas** for the supplied workspace and branch; wait for **Fit view**.
2. Move the workspace; wait for **Saved** and assert the plan stayed fixed.
3. Move the plan, reload, and assert both positions restore with current labels and Git state.
4. Trigger the supplied safe plan scan, refresh Canvas, and assert the new plan appears without moving saved nodes or
   showing a placement action, count, or notification.
5. After launch, move the session, reload without restarting the backend, and assert its placement and lifecycle restore.

These placement operations are isolated safe writes and never mutate repository entities. Capture initial, moved, and
restored layouts without terminal content.

## Launch And Branch Safety
<!-- chunkId: e2e-testing-workspace-terminal-canvas-launch -->
<!-- keywords: launch terminal, idempotency, mismatch, interrupted, process -->

[open] Does the current implementation still satisfy the launch and branch safety behavior described below? This section has not yet been reverified.

1. Select the plan, confirm **Launch terminal**, **Run smoke verification**, and **Open full view** are present as
   capabilities allow, then choose **Launch terminal**.
2. Wait for the new session node to be placed and expanded; assert its interactive terminal is inside the Canvas and the
   right Workbench is closed.
3. While **Launching…** is visible, activate again only if enabled; assert at most one record and process.
4. Select another node and return; assert selection does not change terminal disclosure and no relaunch occurs. Use the
   session's top-right action to collapse and expand it. Interacting with terminal input and scrolling must not drag the
   session node.
5. Close the terminal and assert the node collapses without cancellation; reopen it with the top-right action, then use the explicit
   **Cancel process** control and assert the placement remains.
6. Use the supplied mismatch fixture, attempt the intentionally stale action, and assert **Checkout changed** appears
   with expected/current context and no new session.
7. Verify an application restart changes an orphaned running record to interrupted and expands into lifecycle detail
   without a false reconnect.

Branch manipulation and process launch are safe writes only in the supplied isolated fixture. Stop this section at the
first failed assertion and follow the documented cleanup.

## Git, Verification, And Accessibility
<!-- chunkId: e2e-testing-workspace-terminal-canvas-verification -->
<!-- keywords: git, verification, stale, search, keyboard -->

[open] Does the current implementation still satisfy the git, verification, and accessibility behavior described below? This section has not yet been reverified.

1. Assert workspace branch, HEAD, clean/dirty/conflicted text, and changed-file count.
2. Run smoke verification from the plan and assert result, freshness, and abbreviated revisions are separate.
3. Apply the supplied reversible mutation; wait for refresh and assert the previous pass becomes stale and historical.
4. Use **Search Canvas nodes**, focus the plan, press an arrow key, and assert only it moves and **Saved** is announced.
5. Remove the session placement without cancellation, refresh and assert it stays hidden, cancel a reset preview, and
   revisit with reduced motion enabled.

Capture current/stale verification and visible keyboard focus. Diagnostics may use Chrome DevTools only after Playwright
execution; Chrome is never the execution provider.

## Focus And Session Surfaces
<!-- chunkId: e2e-testing-workspace-terminal-canvas-session-focus -->
<!-- keywords: focus toggle, hidden nodes, list mode, checkout changes -->

[open] Can the current build complete the PM-042 journey across the global Sessions page and checkout-scoped Workbench tab with two isolated workspaces, deterministic edits, and native terminal keyboard interaction? Historical evidence covered only one throwaway workspace.

1. Start live sessions for plans in two isolated Local workspaces, open **Canvas** for one workspace, and activate **Focus**.
2. Assert only active plans, their live sessions, and their connecting edges remain; confirm dragging is unavailable and saved positions are unchanged after Focus is disabled.
3. Select the workspace node, open its **Sessions** tab, verify records are grouped by plan for the current checkout, switch between Terminal and Changes, and inspect one bounded file diff.
4. Stop the selected session through the confirmation flow and assert the live-only filter relaxes while the selected record remains visible.
5. Choose **Open on board** and assert the existing session node is selected. Activate Focus and assert the stopped session leaves the focused set without altering the restored layout.
6. Open the top-level **Sessions** page and assert both authorized workspaces are grouped without leaking inaccessible workspace records.

Treat session launch, cancellation, and repository edits as isolated safe writes. Use Playwright MCP in a fresh context, and retain only screenshots and safe identifiers as evidence.

## Cleanup And Result
<!-- chunkId: e2e-testing-workspace-terminal-canvas-cleanup -->
<!-- keywords: cleanup, cancel, revert, evidence, result -->

[open] Does the current implementation still satisfy the cleanup and result behavior described below? This section has not yet been reverified.

Cancel created processes through the normal control, revert the isolated mutation, and restore the original test branch
through guarded controls. Record passed, failed, blocked, and not-run sections plus safe evidence references in the
ticket-local latest result. The current PM-037 result is not run because runtime inputs were not supplied.

## Coverage Deltas And Limits
<!-- chunkId: e2e-testing-workspace-terminal-canvas-limits -->
<!-- keywords: coverage, local, cloud agent, snapshot, limits -->

[open] Does the current implementation still satisfy the coverage deltas and limits behavior described below? This section has not yet been reverified.

PM-037 introduces this journey. It covers only Local content and execution. Cloud Agent execution, Agentless Remote
Snapshot Canvas UX, groups, custom links, multiple canvases, and collaboration need separate capability delivery and
future journey deltas. PM-042 adds Focus, global and checkout-scoped session browsing, and checkout-derived changes. See
[[workspace-terminal-canvas]], [[workspace-session-focus]], and [[workspace-session-focus-reference]].
