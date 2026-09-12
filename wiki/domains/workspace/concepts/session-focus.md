---
slug: workspace-session-focus
title: Session Focus
pageType: CONCEPT
topics: workspace, canvas, sessions, focus, changes
summary: Session Focus connects active Canvas work, checkout changes, and the workspace session list.
sourceRef: |
  PM-042 (plan retired) | Session Focus
  PM-042 (plan retired) | User experience
  PM-042 (plan retired) | Canvas and Sessions UI
  PM-042 (plan retired) | Delivery
lastVerified: 2026-09-12
---

## Focused Canvas
<!-- chunkId: workspace-session-focus-focused-canvas -->
<!-- keywords: focus mode, active session, virtual layout, saved position -->

[live] Focus shows only plans with live starting or running sessions, those session nodes, and their connecting edges; inactive nodes are hidden and the visible set receives a temporary layout. Evidence: `web/src/features/canvas/CanvasBoard.tsx:92`, `web/src/features/canvas/CanvasBoard.tsx:128`.

Focus does not overwrite saved positions or the saved viewport. Dragging is disabled while focused, and leaving Focus restores the full saved Canvas. See [[workspace-terminal-canvas]].

[live] Focus is one Eye/EyeOff toggle. It does not add a status label, a separate Turn off link, or an automatic-layout hint beside the control. Evidence: `web/src/features/canvas/CanvasBoard.tsx:265`.

## Workbench Session Views
<!-- chunkId: workspace-session-focus-shared-session-browser -->
<!-- keywords: list mode, plan grouping, active filter, terminal changes -->

[live] The Workbench has Canvas and Sessions views for the current workspace checkout. Sessions is a plan-grouped list with Terminal and Changes detail, live-only filtering, explicit cancellation, and an action that opens the session node back on Canvas. Evidence: `web/src/pages/CanvasPage.tsx:72`, `web/src/features/canvas/CanvasSessionsView.tsx:43`, `web/src/features/sessions/SessionBrowser.tsx:92`.

When a selected live session stops, the live-only filter relaxes so the user's current context remains visible. The node-detail panel is reserved for the selected workspace, plan, or terminal session rather than hosting the workspace session list.

## Checkout-derived Changes
<!-- chunkId: workspace-session-focus-checkout-derived-changes -->
<!-- keywords: launch commit, git diff, shared checkout, attribution -->

[live] Changes are calculated from each session's launch commit through the current checkout, including tracked and untracked state, and are never stored in the session record. Evidence: `internal/ai/session_changes.go:154`, `internal/git/git_since.go:28`.

This is checkout history, not attribution. Work by another session or person in the same checkout can appear if it occurred after the launch commit.

## Not supported
<!-- chunkId: workspace-session-focus-not-supported -->
<!-- keywords: worktree isolation, transcript storage, cloud diff, attribution -->

[live] Session-owned worktrees, transcript or prompt storage, Cloud-provider changes, and per-session authorship attribution are not supported. Evidence: `internal/ai/session_changes.go:126`, `internal/ai/session_changes.go:154`.

## Known conflicts
<!-- chunkId: workspace-session-focus-known-conflicts -->
<!-- keywords: documentation drift, dim rule, hidden nodes -->

[legacy] The original PM-042 narrative proposed dimming inactive Canvas nodes. The delivered component filters them out and lays out only the active subset, and the domain guide now describes that delivered behavior. Evidence: `web/src/features/canvas/CanvasBoard.tsx:128`, `web/src/features/canvas/CanvasBoard.tsx:135`, `docs/domain/terminal/session-focus.md:61`.

[legacy] PM-042 shipped a top-level Sessions page and a workspace-node Sessions tab. Sessions is now a main Workbench view beside Canvas, while the node-detail panel remains specific to the selected node. Evidence: `web/src/app/router.ts:49`, `web/src/pages/CanvasPage.tsx:150`, `web/src/features/canvas/CanvasWorkbench.tsx:164`.

## Related pages

See [[workspace-session-focus-reference]], [[workspace-terminal-canvas]], and [[e2e-testing-workspace-terminal-canvas]].
