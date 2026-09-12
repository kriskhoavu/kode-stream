---
slug: workspace-session-focus
title: Session Focus
pageType: CONCEPT
topics: workspace, canvas, sessions, focus, changes
summary: Session Focus connects active Canvas work, checkout changes, and global or checkout-scoped session browsing.
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

[live] While Focus is active, its control reports the active plan and session counts, shows an automatic-layout hint, and offers **Turn off** when nothing is running. Evidence: `web/src/features/canvas/CanvasBoard.tsx:107`, `web/src/features/canvas/CanvasBoard.tsx:254`.

## Session Surfaces
<!-- chunkId: workspace-session-focus-shared-session-browser -->
<!-- keywords: global inventory, plan grouping, active filter, terminal changes -->

[live] The top-level Sessions page groups authorized records by workspace and plan. A workspace node's Workbench also has a checkout-scoped Sessions tab grouped by plan. Both provide Terminal and Changes detail, live-only filtering, explicit cancellation, and an action that opens the existing session node on Canvas. Evidence: [[workspace-session-focus-reference]].

When a selected live session stops, the live-only filter relaxes so the user's current context remains visible.

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

[live] The top-level Sessions page and workspace-node Sessions tab both ship. The checkout-scoped tab supplements the global page rather than replacing it. Evidence: [[workspace-session-focus-reference]].

## Related pages

See [[workspace-session-focus-reference]], [[workspace-terminal-canvas]], and [[e2e-testing-workspace-terminal-canvas]].
