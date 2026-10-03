---
slug: workspace-ai-launch-reference
title: AI Session Launch Reference
pageType: REFERENCE
topics: launch request, prompt resolution, capability discovery, PTY, terminal layout, launch guard
summary: Reference for composing, validating, launching, presenting, and recording embedded and external AI sessions.
sourceRef: |
  PM-018 (plan retired) | External launch contract
  PM-020 (plan retired) | Embedded PTY and presentation contracts
  PM-027 (plan retired) | Prompt, capability, and layout contracts
lastVerified: 2026-09-12
---

# AI Session Launch Reference

## Prompt Resolution
<!-- chunkId: workspace-ai-launch-reference-prompt -->
<!-- keywords: preset, prompt draft, custom prompt, context mode -->

[live] PM-027 the service accepts a preset, `promptDraft`, or the legacy `customPrompt` shape but rejects ambiguous combinations; an edited draft can retain its preset identity. Evidence: `internal/ai/ai_launch_service.go:318`.

Selected skills and agents are normalized against provider capability discovery. Discovery reads provider-specific
capability roots in the workspace and application configuration without recursively treating arbitrary files as tools.

## Launch Guards
<!-- chunkId: workspace-ai-launch-reference-guards -->
<!-- keywords: workspace ID, expected branch, observed commit, idempotency -->

[live] PM-018 and PM-020 embedded launches carry expected workspace and branch context, observed commit, and an idempotency key. Evidence: `internal/ai/ai_embedded_service.go:40`, `internal/ai/ai_embedded_service.go:42`, `internal/ai/ai_embedded_service.go:43`.

The service revalidates operational context before process start. A mismatch is a blocked launch with recovery context,
not permission to switch branches automatically. Session records are published around process state transitions so a
failed record write cannot silently produce an untracked running process.

## Embedded PTY Contract
<!-- chunkId: workspace-ai-launch-reference-pty -->
<!-- keywords: PTY, resize, channel, cancel, process state -->

[live] PM-020 the embedded adapter starts a bounded PTY, supports terminal resize, streams through a session channel, and tracks exit state. Evidence: `internal/ai/ai_terminal_repository.go:155`, `internal/ai/ai_terminal_repository.go:297`, `web/src/features/ai-session/EmbeddedTerminal.tsx:13`.

Closing a normal embedded window cancels its process unless the owning surface explicitly retains it. Hiding a compact
Canvas view does not redefine process lifetime; the session record remains the shared source for other views.

## Presentation Modes
<!-- chunkId: workspace-ai-launch-reference-presentation -->
<!-- keywords: floating, side panel, maximized, minimized, responsive -->

[live] PM-027 the dock stores floating geometry, resizes the right panel, and promotes an undersized side panel to maximized presentation. Evidence: `web/src/features/ai-session/EmbeddedTerminalDock.tsx:30`, `web/src/features/ai-session/EmbeddedTerminalDock.tsx:154`, `web/src/features/ai-session/EmbeddedTerminalDock.tsx:173`.

## Not supported
<!-- chunkId: workspace-ai-launch-reference-not-supported -->
<!-- keywords: shell injection, unknown capability, branch mutation -->

[live] PM-018 launch does not accept an arbitrary executable from the request, and branch mismatch does not trigger an implicit checkout. Evidence: `internal/ai/ai_launch_service.go:84`, `internal/ai/ai_embedded_session_record_test.go:48`.

## Known conflicts
<!-- chunkId: workspace-ai-launch-reference-known-conflicts -->
<!-- keywords: custom prompt, old terminal layout, session record -->

[legacy] `customPrompt` survives as a compatibility field from the earlier launch contract. Current clients send `promptDraft`, and current session browsing depends on the durable session record introduced after PM-020.

## Related pages

See [[workspace-ai-session-launch]], [[workspace-session-focus-reference]], and [[workspace-terminal-canvas-reference]].
