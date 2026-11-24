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
<!-- keywords: preset, promptDraft, customPrompt, capability discovery -->

[live] The service accepts a preset, a `promptDraft` or the older `customPrompt`, and rejects an ambiguous mix. An edited draft can keep its preset identity. Evidence: `internal/ai/ai_launch_service.go:194`.

Selected skills and agents are normalized against provider discovery. Discovery reads the provider's capability roots in the workspace and app configuration. It never treats arbitrary files as tools.

## Launch Guards
<!-- chunkId: workspace-ai-launch-reference-guards -->
<!-- keywords: expected branch, observed commit, idempotency, record write -->

[live] An embedded launch carries the expected workspace and branch, the observed commit and an idempotency key. Evidence: `internal/ai/ai_embedded_service.go:37`.

The service revalidates context before the process starts. A mismatch blocks the launch with recovery context and never switches branches. Session records are written around process state changes, so a failed write cannot leave an untracked process running.

## Embedded PTY Contract
<!-- chunkId: workspace-ai-launch-reference-pty -->
<!-- keywords: resize, channel, cancel, exit state -->

[live] The embedded adapter starts a bounded PTY, supports resize, streams through a session channel and tracks exit state. Evidence: `internal/ai/ai_terminal_repository.go:105`, `internal/ai/ai_terminal_repository.go:283`.

Closing a normal embedded window cancels its process unless the owning surface keeps it. Hiding a Canvas view does not change process lifetime.

## Presentation Modes
<!-- chunkId: workspace-ai-launch-reference-presentation -->
<!-- keywords: floating geometry, side panel, maximized, responsive -->

[live] The dock stores floating geometry, resizes the right panel and maximizes a side panel that is too narrow. Evidence: `web/src/features/ai-session/EmbeddedTerminalDock.tsx:30`.

## Not supported
<!-- chunkId: workspace-ai-launch-reference-not-supported -->
<!-- keywords: shell injection, request executable, branch mutation -->

[live] A request cannot name an executable. A branch mismatch never triggers a checkout. Evidence: `internal/ai/ai_external_launch.go:101`.

## Known conflicts
<!-- chunkId: workspace-ai-launch-reference-known-conflicts -->
<!-- keywords: custom prompt, compatibility field, current clients -->

- "Clients send `customPrompt`." Current clients send `promptDraft`. `customPrompt` is a compatibility field.

## Related pages

See [[workspace-ai-session-launch]] and [[workspace-terminal-canvas-reference]].
