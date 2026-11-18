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
<!-- keywords: agent chip, live ring, launcher, plus count, hollow chip -->

[live] A card shows one green robot chip per live session launched from it, at most three, then `+N`; the provider is named in each chip's tooltip. A Claude Code conversation from the user's terminal that is active in the last ten minutes gets a dashed terminal chip. A card with neither offers a launcher, shown on hover or focus. Evidence: `web/src/features/workstream/AgentsPanel.tsx:41`, `web/src/features/workstream/useItemSessions.ts:9`.

Every chip is its own button, so clicking one opens that agent and never previews or drags the card. Documentation cards carry no cluster. Evidence: `web/src/pages/WorkstreamPage.tsx:1536`.

## Agents Tab And Agent Pane
<!-- chunkId: workspace-card-agent-sessions-drawer -->
<!-- keywords: agents tab, main pane, back to document, continue prompting, pop out -->

[live] The card drawer's Agents side tab holds a session launcher, the card's live sessions, the Claude conversations that mention it, and its ended sessions under **Earlier**. Picking one replaces the document in the drawer's main pane with that agent. Evidence: `web/src/features/workstream/AgentsPanel.tsx:64`, `web/src/features/workstream/AgentsPanel.tsx:118`.

A session pane shows the live terminal, where the conversation continues by typing, beside a Changes view, **Pop out to dock**, and **Back to document**. An Escape typed into the terminal goes to the agent and does not close the drawer. Evidence: `web/src/pages/WorkstreamPage.tsx:1621`.

## Keeping A Session Alive
<!-- chunkId: workspace-card-agent-sessions-dock-hand-off -->
<!-- keywords: dock hand-off, last viewer, fresh grant, cancel grace -->

[live] A session closed or cancelled anywhere in the app leaves the card at once, before the next poll confirms it. An embedded process is cancelled shortly after its last viewer leaves, so every live session opened in the drawer keeps its terminal mounted, hidden, while the drawer shows the document, a transcript, or another session. Closing the drawer, opening another card, or leaving the page hands each of them to the terminal dock with a fresh grant. Evidence: `web/src/features/workstream/AgentsPanel.tsx:118`, `web/src/pages/WorkstreamPage.tsx:1641`, `web/src/features/workstream/AgentsPanel.tsx:107`.

[live] A terminal whose connection drops reconnects for fifteen seconds from the drop, asking for a fresh grant each time, and the channel answers a terminal size outside the PTY's limits with an error instead of closing. Evidence: `web/src/features/ai-session/EmbeddedTerminal.tsx:67`, `internal/server/api/ai_controller.go:343`.

## Claude Code Conversations
<!-- chunkId: workspace-card-agent-sessions-claude -->
<!-- keywords: transcript, continue here, resume, session id, fork -->

[live] Conversations the user holds with Claude Code in the workspace root are read from Claude Code's project directory and tied to the items that the user's prompts, Claude's replies, or Claude's file reads and edits name; a plan that only appears in a tool's output does not count. They show read-only as their last turns, with **Continue here**, which resumes the conversation in an embedded terminal on the card. Evidence: `internal/ai/claude_transcripts.go:137`, `internal/ai/ai_embedded_service.go:224`.

An embedded Claude session is named after its conversation, so kode-stream's own conversations are never listed as the user's, and an ended one can be continued from **Earlier**. A conversation written in the last two minutes may still be open in the user's terminal; continuing it here forks it, and the pane says so. Evidence: `internal/ai/ai_embedded_service.go:19`, `web/src/features/workstream/AgentsPanel.tsx:223`.

## Across The Workbench
<!-- chunkId: workspace-card-agent-sessions-workbench -->
<!-- keywords: ephemeral node, pin to canvas, open on card, sessions list -->

[live] A session started or resumed from a card is an ordinary embedded session with a plan reference. The Sessions list shows it while live and after it ends. The Canvas draws it beside its placed plan only while it is live, unless it is pinned, moved, or resized. Evidence: `internal/canvas/service.go:650`, `web/src/features/canvas/CanvasBoard.tsx:262`.

Agent Space's Sessions view opens an interactive session on its card with **Open on card**, through `/workstream?itemId=&agent=`. The harness's agents are harness sessions, not interactive ones: once the harness works on a card, its Agents tab shows the run beside the card's interactive sessions, and a harness agent opens there as its conversation. See [[workspace-agent-sessions]]. Evidence: `web/src/features/sessions/SessionBrowser.tsx:212`, `web/src/app/router.ts:151`, `web/src/features/workstream/AgentsPanel.tsx:25`.

## Not supported
<!-- chunkId: workspace-card-agent-sessions-not-supported -->
<!-- keywords: cloud, other agents, external terminal, worktree -->

[live] Conversation discovery runs in Local mode only. Agents other than Claude Code, and sessions opened in Terminal.app, iTerm2 or WezTerm, are not shown on the card. Every session still shares the workspace checkout. Evidence: `internal/ai/claude_transcripts.go:113`.

## Related pages

See [[workspace-card-agent-sessions-reference]], [[workspace-agent-sessions]], [[workspace-session-focus]], [[workspace-terminal-canvas]], and [[workspace-ai-session-launch]].
