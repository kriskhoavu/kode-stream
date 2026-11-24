---
slug: workspace-session-focus
title: Session Focus
pageType: CONCEPT
topics: workspace, canvas, sessions, focus, changes
summary: Session Focus connects active Canvas work, checkout changes, and Agent Space's session list.
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

[live] **Focus** on the Canvas toolbar shows only plans with a starting or running session, those session nodes and the edges between them. The visible set gets a temporary layout. Evidence: `web/src/features/canvas/CanvasBoard.tsx:92`.

Focus never overwrites saved positions or the viewport. Dragging is off while Focus is on. The control shows the active plan and session counts and offers **Turn off** when nothing runs.

## Session Surfaces
<!-- chunkId: workspace-session-focus-shared-session-browser -->
<!-- keywords: sessions view, plan grouping, active filter, cancellation -->

[live] Agent Space's Sessions view lists the workspace's sessions for the checkout, grouped by plan, with a live-only filter, cancellation and **Open on board**. [[workspace-agent-sessions]] describes the two kinds. Evidence: [[workspace-session-focus-reference]].

When a selected live session stops, the live-only filter relaxes so the selection stays visible.

## Checkout-derived Changes
<!-- chunkId: workspace-session-focus-checkout-derived-changes -->
<!-- keywords: launch commit, git diff, shared checkout, attribution -->

[live] Changes run from a session's launch commit to the current checkout, including uncommitted and untracked files. They are computed on request and never stored. Evidence: `internal/ai/session_changes.go:123`.

This is checkout history, not attribution. Work by another session or person in the same checkout appears too.

## Not supported
<!-- chunkId: workspace-session-focus-not-supported -->
<!-- keywords: worktree isolation, transcript storage, cloud diff, authorship -->

[live] Session-owned worktrees, transcript or prompt storage, Cloud-provider changes and per-session authorship. Evidence: `internal/ai/session_changes.go:98`.

## Known conflicts
<!-- chunkId: workspace-session-focus-known-conflicts -->
<!-- keywords: dim rule, hidden nodes, inactive plans -->

- "Focus dims inactive nodes." It hides them and lays out only the active set.

## Related pages

See [[workspace-session-focus-reference]], [[workspace-terminal-canvas]] and [[e2e-testing-workspace-terminal-canvas]].
