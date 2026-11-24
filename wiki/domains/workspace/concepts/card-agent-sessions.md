---
slug: workspace-card-agent-sessions
title: Card Agent Sessions
pageType: CONCEPT
topics: workspace, workstream, sessions, claude, terminal
summary: A Workstream card shows its live agent sessions, opens their terminal in its drawer, and resumes the user's Claude Code conversations.
sourceRef: |
  PM-043 | Overview
  PM-043 | Components
  PM-043 | Boundaries
lastVerified: 2026-10-03
---

## Agents On The Card
<!-- chunkId: workspace-card-agent-sessions-card -->
<!-- keywords: agent chip, plus count, hollow chip, launcher -->

[live] A card shows one green robot chip per live session launched from it, at most three, then `+N`. Each chip's tooltip names the provider. A Claude Code conversation from the user's own terminal, active in the last ten minutes, gets a dashed terminal chip. A card with neither offers a launcher on hover or focus. Evidence: `web/src/features/workstream/AgentsPanel.tsx:42`, `web/src/features/workstream/useItemSessions.ts:8`.

Each chip is its own button. Clicking it opens that agent and never previews or drags the card. Documentation cards have no chips. Evidence: `web/src/features/workstream/PlanCard.tsx:85`.

## Agents Tab And Agent Pane
<!-- chunkId: workspace-card-agent-sessions-drawer -->
<!-- keywords: drawer, main pane, back to document, pop out -->

[live] The drawer's Agents tab holds a launcher, the card's live sessions, the Claude conversations that mention it, and ended sessions under **Earlier**. Picking one replaces the document in the main pane with that agent. Evidence: `web/src/features/workstream/AgentsPanel.tsx:64`.

The pane shows the live terminal beside a Changes view, **Pop out to dock** and **Back to document**. Escape typed into the terminal goes to the agent and does not close the drawer. Evidence: `web/src/features/workstream/plan-drawer/PlanPreviewDrawer.tsx:99`.

## Keeping A Session Alive
<!-- chunkId: workspace-card-agent-sessions-dock-hand-off -->
<!-- keywords: dock hand-off, last viewer, fresh grant, reconnect -->

[live] Kode Stream cancels an embedded process shortly after its last viewer leaves. So the drawer keeps every opened live session's terminal mounted, hidden. Closing the drawer, opening another card or leaving the page hands each one to the terminal dock with a fresh grant. A closed or cancelled session leaves the card at once. Evidence: `web/src/features/workstream/AgentsPanel.tsx:142`, `web/src/features/workstream/AgentsPanel.tsx:113`.

[live] A dropped terminal reconnects for fifteen seconds, with a fresh grant each try. A terminal size outside the PTY limits gets an error, not a closed channel. Evidence: `web/src/features/ai-session/EmbeddedTerminal.tsx:65`, `internal/server/api/terminal_channel.go:139`.

## Claude Code Conversations
<!-- chunkId: workspace-card-agent-sessions-claude -->
<!-- keywords: transcript, continue here, resume, fork -->

[live] Kode Stream reads the user's Claude Code conversations in the workspace root from Claude Code's project directory. It ties each one to the items that the prompts, replies, or file reads and edits name. A plan named only in tool output does not count. A conversation shows its last turns read-only, with **Continue here**, which resumes it in an embedded terminal. Evidence: `internal/ai/claude_transcripts.go:122`, `internal/ai/ai_embedded_service.go:153`.

An embedded Claude session is named after its conversation, so Kode Stream's own conversations never show as the user's. A conversation written in the last two minutes may still be open elsewhere. Continuing it forks it, and the pane says so. Evidence: `web/src/features/workstream/AgentsPanel.tsx:234`.

## Across The Workbench
<!-- chunkId: workspace-card-agent-sessions-workbench -->
<!-- keywords: ephemeral node, pin to canvas, open on card -->

[live] A session from a card is an ordinary embedded session with a plan reference. The Sessions list shows it while live and after. The Canvas draws it beside its plan only while live, unless it is pinned, moved or resized. Evidence: `internal/canvas/projection_nodes.go:52`.

Agent Space opens an interactive session on its card with **Open on card**. Once the harness works on a card, its Agents tab also shows the run, and a harness agent opens there as its conversation. See [[workspace-agent-sessions]]. Evidence: `web/src/features/sessions/SessionBrowser.tsx:199`, `web/src/features/workstream/AgentsPanel.tsx:24`.

## Not supported
<!-- chunkId: workspace-card-agent-sessions-not-supported -->
<!-- keywords: cloud, other agents, external terminal, worktree -->

[live] Conversation discovery runs in Local mode only. Agents other than Claude Code, and sessions in Terminal.app, iTerm2 or WezTerm, do not show on the card. Every session shares the checkout. Evidence: `internal/ai/claude_transcripts.go:102`.

## Known conflicts
<!-- chunkId: workspace-card-agent-sessions-known-conflicts -->
<!-- keywords: hidden terminal, drawer close, process cancel -->

- "Closing the drawer stops the agent." It does not. The terminal moves to the dock.

## Related pages

See [[workspace-card-agent-sessions-reference]], [[workspace-agent-sessions]] and [[workspace-ai-session-launch]].
