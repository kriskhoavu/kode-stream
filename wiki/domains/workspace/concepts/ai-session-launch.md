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

[live] The launch dialog resolves a provider, a context mode, a preset or edited prompt, and optional skills and agents the provider exposes. Then it starts the process. Evidence: `web/src/features/ai-session/AISessionLaunchDialog.tsx:37`, `internal/ai/ai_launch_capabilities.go:43`.

Kode Stream checks each capability against the provider's catalog. The prompt carries only the chosen workspace or item context.

## Choose Embedded Or External
<!-- chunkId: workspace-ai-session-launch-surface -->
<!-- keywords: external terminal, PTY, terminal adapter, safe record -->

[live] A session runs in a configured external terminal or in a PTY that Kode Stream manages. Evidence: `internal/ai/ai_launch_service.go:238`, `internal/ai/ai_terminal_repository.go:105`.

An external launch hands the process to the terminal adapter. An embedded launch keeps terminal I/O in Kode Stream and records safe metadata for Canvas and the Sessions view.

## Embedded Presentation
<!-- chunkId: workspace-ai-session-launch-layout -->
<!-- keywords: floating terminal, side panel, maximize, minimize -->

[live] An embedded session can float, dock right, maximize or minimize without restarting its process. Evidence: `web/src/features/ai-session/EmbeddedTerminalDock.tsx:9`.

Canvas and the Sessions view are further views of the same session, not other launch contracts. See [[workspace-terminal-canvas]] and [[workspace-session-focus]].

## Not supported
<!-- chunkId: workspace-ai-session-launch-not-supported -->
<!-- keywords: snapshot launch, arbitrary capability, hidden prompt -->

[live] A reviewed snapshot cannot launch a session. An unknown capability is rejected, never injected into a prompt. Evidence: `internal/ai/ai_launch_capabilities.go:43`.

## Known conflicts
<!-- chunkId: workspace-ai-session-launch-known-conflicts -->
<!-- keywords: terminal dock, sole presentation, layouts -->

- "The terminal dock is the only place an embedded session shows." Canvas, the Workbench and the Sessions view show it too.

## Related pages

See [[workspace-ai-launch-reference]] and [[workspace-agent-sessions]].
