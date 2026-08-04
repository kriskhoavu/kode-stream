---
slug: platform-terminal-canvas-workflows
title: Use Terminal Canvas
pageType: HOW_TO
roles: USER, BA, TESTER
topics: canvas, drag, terminal, git, verification
summary: How to arrange work, launch a branch-safe terminal, and interpret Git and verification state in Canvas.
sourceRef: plans/platform/PM-037/scenario/scenario-00-overview.md
sourceRef: plans/platform/PM-038/scenario/scenario-00-overview.md
sourceCount: 2
lastTicket: PM-038
---

## Arrange And Restore Work
<!-- chunkId: platform-terminal-canvas-workflows-arrange -->
<!-- keywords: canvas, nodes, drag, keyboard, restore -->

Open **Canvas** for the active workspace checkout. Canvas resolves Git before loading or seeding the branch-scoped
layout; it has no independent branch selector. Drag a workspace, plan, or session node independently, or
focus it and use an arrow key to move 12 pixels; Shift plus an arrow moves one pixel. Wait for **Saved** before reloading.
**Add new nodes** accepts deterministic positions for newly discovered plans and sessions without moving saved nodes.
**Reset layout** previews and confirms a presentation-only reset. **Remove from Canvas** keeps the referenced entity but
hides its placement for this layout, so **Add new nodes** does not bring it back.

Use **Search Canvas nodes** to find a title, identifier, branch, or session state. **Fit view** recovers off-screen nodes.
Closing the Workbench returns focus to the selected node.

To inspect a different branch without checkout, exit Canvas and use [[platform-branch-review-workflows]]. Returning to a
previous checkout restores that branch's saved layout without making its layout a global branch selection.

## Launch A Session
<!-- chunkId: platform-terminal-canvas-workflows-launch -->
<!-- keywords: plan, launch terminal, branch, session, idempotency -->

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
offering a false reconnect. See [[platform-terminal-canvas-reference]].

## Inspect Git And Verification
<!-- chunkId: platform-terminal-canvas-workflows-verify -->
<!-- keywords: git, verification, freshness, stale, historical -->

Select the workspace for branch, HEAD, working-tree status, changed-file count, and current verification detail. A plan
also exposes **Run smoke verification** when its resolved capability allows it. Result and freshness are separate: a pass
may be current, stale, or inconclusive. After a relevant repository or verification-configuration change, the previous
pass appears as **Passed (historical)** until rerun.

## MVP Limits
<!-- chunkId: platform-terminal-canvas-workflows-limits -->
<!-- keywords: local, groups, snapshots, cloud agent, deferred -->

The delivered journey is a Local desktop workbench with workspace, plan, and session nodes. It is not a general graph
editor and does not provide groups, notes, artifacts, custom links, multiple canvases, Cloud Agent execution, Remote
Snapshot Canvas UX, worktrees, or collaborative layouts. See [[platform-terminal-canvas-concept]].
