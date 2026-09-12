---
slug: workspace-ai-session-launch
title: Launch AI Sessions
pageType: CONCEPT
topics: AI session, prompt composer, embedded terminal, external terminal, capabilities
summary: AI sessions combine an explicit workspace or item context with a provider, prompt, optional capabilities, and either an embedded or external terminal surface.
sourceRef: |
  PM-018 (plan retired) | External session launch
  PM-020 (plan retired) | Embedded terminal
  PM-027 (plan retired) | Launch composer and layouts
lastVerified: 2026-09-12
---

# Launch AI Sessions

## Compose Before Launch
<!-- chunkId: workspace-ai-session-launch-compose -->
<!-- keywords: provider, prompt, preset, skill, agent -->

[live] PM-018 and PM-027 the launch dialog resolves a provider, context mode, preset or edited prompt, and optional provider-discovered skills and agents before starting a process. Evidence: `web/src/features/ai-session/AISessionLaunchDialog.tsx:36`, `web/src/features/ai-session/AISessionLaunchDialog.tsx:168`, `internal/ai/ai_launch_service.go:348`.

Capability selection is validated against the chosen provider's catalog. The prompt carries the chosen workspace or item
context rather than giving the process an unbounded application snapshot.

## Choose Embedded Or External
<!-- chunkId: workspace-ai-session-launch-surface -->
<!-- keywords: embedded terminal, external terminal, PTY, process -->

[live] PM-018, PM-020, and PM-027 a session can launch through a configured external terminal or through the application-managed PTY. Evidence: `internal/ai/ai_launch_service.go:309`, `internal/ai/ai_terminal_repository.go:155`, `web/src/features/ai-session/AISessionLaunchControl.tsx:29`.

External launch delegates the visible process to the selected terminal adapter. Embedded launch keeps terminal I/O in
Kode Stream and records enough safe metadata for Canvas, the workspace Workbench tab, and the global Sessions page.

## Embedded Presentation Does Not Change Session Identity
<!-- chunkId: workspace-ai-session-launch-layout -->
<!-- keywords: floating terminal, side panel, maximize, minimize -->

[live] PM-027 an embedded session can be floating, docked to the right, maximized, or minimized without restarting the underlying process. Evidence: `web/src/features/ai-session/EmbeddedTerminalDock.tsx:8`, `web/src/features/ai-session/EmbeddedTerminalDock.tsx:81`, `web/src/features/ai-session/EmbeddedTerminalDock.tsx:131`.

Canvas may present a live session as a spatial node. The workspace-node Workbench tab and global Sessions page present
the same terminal and changes in scoped lists. These are views over session state, not different launch contracts. See
[[workspace-terminal-canvas]] and [[workspace-session-focus]].

## Not supported
<!-- chunkId: workspace-ai-session-launch-not-supported -->
<!-- keywords: snapshot launch, arbitrary capability, hidden prompt -->

[live] PM-027 reviewed snapshots cannot launch operational sessions, and unknown capability identifiers are rejected rather than injected into a prompt. Evidence: `web/src/features/ai-session/AISessionLaunchDialog.test.tsx:83`, `internal/ai/ai_launch_service.go:369`.

## Known conflicts
<!-- chunkId: workspace-ai-session-launch-known-conflicts -->
<!-- keywords: terminal dock, canvas, sessions page, old layout -->

[legacy] PM-020 treats the application dock as the sole embedded presentation. PM-027 added more layouts, and PM-037 plus PM-042 later added Canvas Workbench and Sessions views.

## Related pages

See [[workspace-ai-launch-reference]], [[workspace-terminal-canvas]], and [[workspace-session-focus]].
