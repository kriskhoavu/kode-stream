---
slug: workspace-terminal-canvas-workflows
title: Use Terminal Canvas
pageType: HOW_TO
topics: canvas, drag, terminal, git, verification
summary: How to arrange work, launch a branch-safe terminal, and interpret Git and verification state in Canvas.
sourceRef: |
  PM-037 (plan retired) | User workflows
  PM-038 (plan retired) | Checkout workflows
lastVerified: never
---

## Arrange And Restore Work
<!-- chunkId: workspace-terminal-canvas-workflows-arrange -->
<!-- keywords: canvas, nodes, drag, keyboard, restore -->

[open] Does the current implementation still satisfy the arrange and restore work behavior described below? This section has not yet been reverified.

Open **Canvas** for the active workspace checkout. Canvas resolves Git before loading or seeding the branch-scoped
layout; it has no independent branch selector. Drag a workspace, plan, or session node independently, or
focus it and use an arrow key to move 12 pixels; Shift plus an arrow moves one pixel. Wait for **Saved** before reloading.
Canvas silently gives newly discovered plans and sessions deterministic positions without moving saved nodes or showing
an action, count, or notification. **Reset layout** previews and confirms a presentation-only reset. **Remove from
Canvas** keeps the referenced entity but hides its placement for this layout, so automatic placement does not bring it
back.

Use **Search Canvas nodes** to find a title, identifier, branch, or session state. **Fit view** recovers off-screen nodes.
Closing the Workbench returns focus to the selected node.

To inspect a different branch without checkout, exit Canvas and use [[workspace-branch-review-workflows]]. Returning to a
previous checkout restores that branch's saved layout without making its layout a global branch selection.

## Launch A Session
<!-- chunkId: workspace-terminal-canvas-workflows-launch -->
<!-- keywords: plan, launch terminal, branch, session, idempotency -->

[open] Does the current implementation still satisfy the launch a session behavior described below? This section has not yet been reverified.

Select a plan and choose **Launch terminal**. The configured default provider is used. The server checks the workspace,
plan, expected branch, observed commit, provider, authorization, and limits again before launch. If the checkout changed,
Canvas shows **Checkout changed** with expected/current context and offers **Refresh Canvas and Git status**; it never
switches branches automatically.

A successful launch creates one safe durable record and one live process, places only that new session when needed,
and explicitly opens its Canvas node into the interactive terminal. Later selection does not expand or collapse a
session. Use the top-right disclosure action to control the terminal; that choice is saved with the placement. Closing
the terminal only collapses the session node. **Cancel process** remains separate from closing the terminal and from
**Remove from Canvas**. After an
application restart, an orphaned running record becomes interrupted and expands into lifecycle detail instead of
offering a false reconnect. See [[workspace-terminal-canvas-reference]].

## Inspect Git And Verification
<!-- chunkId: workspace-terminal-canvas-workflows-verify -->
<!-- keywords: git, verification, freshness, stale, historical -->

[open] Does the current implementation still satisfy the inspect git and verification behavior described below? This section has not yet been reverified.

Select the workspace for branch, HEAD, working-tree status, changed-file count, and current verification detail. A plan
also exposes **Run smoke verification** when its resolved capability allows it. Result and freshness are separate: a pass
may be current, stale, or inconclusive. After a relevant repository or verification-configuration change, the previous
pass appears as **Passed (historical)** until rerun.

## MVP Limits
<!-- chunkId: workspace-terminal-canvas-workflows-limits -->
<!-- keywords: local, groups, snapshots, cloud agent, deferred -->

[open] Does the current implementation still satisfy the mvp limits behavior described below? This section has not yet been reverified.

The delivered journey is a Local desktop workbench with workspace, plan, and session nodes. It is not a general graph
editor and does not provide groups, notes, artifacts, custom links, multiple canvases, Cloud Agent execution, Remote
Snapshot Canvas UX, worktrees, or collaborative layouts. See [[workspace-terminal-canvas]].

## Not supported
<!-- chunkId: workspace-terminal-canvas-workflows-not-supported -->
<!-- keywords: capability boundary, exclusions, unsupported action -->

[open] Which additional unsupported operations should this page name after implementation verification?

## Known conflicts
<!-- chunkId: workspace-terminal-canvas-workflows-known-conflicts -->
<!-- keywords: documentation drift, verification gap, competing description -->

[open] Do the retained plan descriptions disagree with current behavior? Verify the sections above before relying on them.

## Related pages

See [[workspace-terminal-canvas-reference]], [[workspace-index]], and [[e2e-testing-index]].
