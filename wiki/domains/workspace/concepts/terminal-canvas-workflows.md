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
<!-- keywords: nodes, keyboard, reset layout, search -->

[open] Does the code still match this page? It is plan-derived and not yet checked.

Open **Agent Space** and choose the **Canvas** view. Canvas uses the current checkout and has no branch selector.

- Drag a node, or focus it and press an arrow key to move it 12 pixels. Shift plus an arrow moves it one pixel.
- Wait for **Saved** before you reload.
- New plans and sessions get positions silently. Saved nodes never move.
- **Reset layout** previews, then resets presentation only.
- **Remove from Canvas** hides the placement. Automatic placement does not bring it back.
- **Search Canvas nodes** finds a title, identifier, branch or session state. **Fit view** recovers off-screen nodes.

To inspect another branch, use [[workspace-branch-review-workflows]].

## Launch A Session
<!-- chunkId: workspace-terminal-canvas-workflows-launch -->
<!-- keywords: launch terminal, checkout changed, idempotency, cancel -->

Select a plan and choose **Launch terminal**. Canvas uses the default provider. The server rechecks workspace, plan, branch, commit, provider, authorization and limits. If the checkout changed, Canvas shows **Checkout changed** and offers **Refresh Canvas and Git status**. It never switches branches.

A launch creates one session record and one process, and opens the new node's terminal. The top-right disclosure action opens or closes the terminal and is saved with the placement. Closing the terminal does not stop the process. **Cancel process** does.

## Inspect Git And Verification
<!-- chunkId: workspace-terminal-canvas-workflows-verify -->
<!-- keywords: freshness, stale, historical, smoke -->

Select the workspace node for branch, HEAD, working-tree status, changed-file count and verification detail. A plan offers **Run smoke verification** when its capability allows. A pass may be fresh, stale or inconclusive. After a relevant change it shows as **Passed (historical)** until you rerun it.

## Not supported
<!-- chunkId: workspace-terminal-canvas-workflows-not-supported -->
<!-- keywords: graph editor, worktrees, branch selector -->

Canvas is not a general graph editor. See [[workspace-terminal-canvas]] for the full list.

## Known conflicts
<!-- chunkId: workspace-terminal-canvas-workflows-known-conflicts -->
<!-- keywords: close terminal, stop process, remove node -->

- "Closing the terminal or removing the node stops the agent." It does not. Only **Cancel process** stops it.

## Related pages

See [[workspace-terminal-canvas-reference]] and [[e2e-testing-workspace-terminal-canvas]].
