---
slug: workspace-card-agent-sessions-reference
title: Card Agent Sessions Reference
pageType: REFERENCE
topics: sessions, claude, transcripts, api, canvas
summary: Reference for Claude conversation discovery, transcript tails, resume, board polling, and ephemeral Canvas session nodes.
sourceRef: |
  PM-043 | Implementation Plan
lastVerified: 2026-10-03
---

## Conversation Discovery
<!-- chunkId: workspace-card-agent-sessions-reference-discovery -->
<!-- keywords: claude config dir, project directory, fourteen days, fifty files -->

[live] `GET /api/workspaces/{id}/claude-transcripts?branch=` reads `$CLAUDE_CONFIG_DIR` or `~/.claude`, then `projects/<workspace path with each non-alphanumeric character as ->`. It keeps UUID-named transcripts changed in the last 14 days, at most 50, whose recorded working directory is the workspace path. Evidence: `internal/ai/claude_transcripts.go:22`, `internal/ai/claude_transcripts.go:122`.

Each result carries the conversation ID, branch, first and last activity, the count of text turns, and the branch's items it mentions as a whole token: the item path, or its identifier when that contains a digit. Only prompts, replies and tool-call inputs count. Conversations named by a session record are left out. Scans are cached per file until its size or modification time changes. Evidence: `internal/ai/claude_transcript_scan.go:25`, `internal/ai/claude_transcript_scan.go:100`.

## Transcript Tail
<!-- chunkId: workspace-card-agent-sessions-reference-tail -->
<!-- keywords: last turns, uuid check, cloud 404, not stored -->

[live] `GET /api/workspaces/{id}/claude-transcripts/{sessionId}/tail?limit=` returns at most 50 text turns, each cut at 4,000 characters. A non-UUID ID returns `invalid_claude_session`. A conversation outside the workspace's directory returns 404. Transcripts are read on request and never stored. Both routes return 404 in Cloud mode. Evidence: `internal/ai/claude_transcripts.go:180`, `internal/server/api/ai_controller.go:142`.

## Resume
<!-- chunkId: workspace-card-agent-sessions-reference-resume -->
<!-- keywords: resumeClaudeSessionId, claude resume, intent, record id -->

[live] `POST /api/items/{id}/ai-sessions/embedded` with `resumeClaudeSessionId` runs `claude --resume <id>` through the same branch, revision and idempotency guards as a new launch. The record has intent `resume` and ID `embedded-<conversation>-<suffix>`. An unknown conversation returns `claude_session_not_found` (404). Evidence: `internal/ai/ai_embedded_service.go:153`.

[live] A new embedded Claude launch passes `--session-id <uuid>` and is recorded as `embedded-<uuid>`. Evidence: `internal/ai/ai_embedded_service.go:16`.

## Board Polling
<!-- chunkId: workspace-card-agent-sessions-reference-polling -->
<!-- keywords: ten seconds, visibility, session started event -->

[live] The board loads the branch's session records and the workspace's conversations every 10 seconds while visible, and again when a session starts or the page becomes visible. After a refusal it stops asking for conversations until the workspace or branch changes. Evidence: `web/src/features/workstream/useItemSessions.ts:7`.

## Ephemeral Canvas Nodes
<!-- chunkId: workspace-card-agent-sessions-reference-ephemeral -->
<!-- keywords: unplaced, beside plan, pin, projection flag -->

[live] A live session with a plan reference and no placement, whose plan is placed, is projected with `ephemeral: true` to the right of its plan, expanded. Moving, resizing, collapsing or **Pin to Canvas** places it. Evidence: `internal/canvas/projection_nodes.go:52`.

## Not supported
<!-- chunkId: workspace-card-agent-sessions-reference-not-supported -->
<!-- keywords: transcript storage, cloud discovery, older transcripts -->

Transcripts older than 14 days, more than 50 transcripts, transcript storage and discovery in Cloud mode.

## Known conflicts
<!-- chunkId: workspace-card-agent-sessions-reference-known-conflicts -->
<!-- keywords: tool output mention, item linking, injected context -->

- "A plan path in a tool's output links the conversation to the plan." It does not. Only prompts, replies and tool-call inputs count.

## Related pages

See [[workspace-card-agent-sessions]] and [[workspace-session-focus-reference]].
