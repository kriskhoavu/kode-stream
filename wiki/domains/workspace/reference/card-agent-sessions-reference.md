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

[live] `GET /api/workspaces/{id}/claude-transcripts?branch=` reads `$CLAUDE_CONFIG_DIR` or `~/.claude`, then `projects/<workspace path with every non-alphanumeric character as ->`. It keeps UUID-named transcripts changed in the last 14 days, at most 50, whose recorded working directory is the workspace path. Evidence: `internal/ai/claude_transcripts.go:29`, `internal/ai/claude_transcripts.go:137`.

Each result carries the conversation ID, branch, first and last activity, the count of user and assistant text turns, and the IDs of the branch's items whose path or identifier it mentions as a whole token. Conversations named by a session record are left out. Scans are cached per file until its size or modification time changes. Evidence: `internal/ai/claude_transcripts.go:265`, `internal/ai/claude_transcripts.go:419`.

## Transcript Tail
<!-- chunkId: workspace-card-agent-sessions-reference-tail -->
<!-- keywords: last turns, uuid, path traversal, not stored -->

[live] `GET /api/workspaces/{id}/claude-transcripts/{sessionId}/tail?limit=` returns at most 50 user and assistant text turns, each cut at 4,000 characters. A non-UUID ID is refused as `invalid_claude_session`; a conversation outside the workspace's directory is 404. Transcripts are read on request and never stored. Evidence: `internal/ai/claude_transcripts.go:197`, `internal/ai/claude_transcripts.go:219`.

Both routes answer 404 in Cloud mode. Evidence: `internal/server/api/ai_controller.go:143`.

## Resume
<!-- chunkId: workspace-card-agent-sessions-reference-resume -->
<!-- keywords: resumeClaudeSessionId, claude --resume, intent resume, record id -->

[live] `POST /api/items/{id}/ai-sessions/embedded` with `resumeClaudeSessionId` launches `claude --resume <id>` through the same branch, revision, and idempotency guards as a new launch. The record has intent `resume` and the ID `embedded-<conversation>-<suffix>`. A conversation the workspace does not hold is `claude_session_not_found` (404). Evidence: `internal/ai/ai_embedded_service.go:224`.

[live] A new embedded Claude launch passes `--session-id <uuid>` and is recorded as `embedded-<uuid>`. Evidence: `internal/ai/ai_embedded_service.go:19`.

## Board Polling
<!-- chunkId: workspace-card-agent-sessions-reference-polling -->
<!-- keywords: ten seconds, visibility, session started event, local only -->

[live] The board loads the branch's session records and the workspace's conversations every 10 seconds while the page is visible, and again when any session starts or the page becomes visible. After a refusal it stops asking for conversations until the workspace or branch changes. Evidence: `web/src/features/workstream/useItemSessions.ts:7`, `web/src/features/workstream/useItemSessions.ts:30`.

## Ephemeral Canvas Nodes
<!-- chunkId: workspace-card-agent-sessions-reference-ephemeral -->
<!-- keywords: ephemeral, unplaced, live, beside plan, pin -->

[live] A live session with a plan reference and no placement, whose plan node is placed, is projected with `ephemeral: true` to the right of its plan and expanded. It is not offered as unplaced. Moving, resizing, collapsing, or **Pin to Canvas** places it. Evidence: `internal/canvas/service.go:590`, `internal/canvas/service.go:650`.

## Related pages

See [[workspace-card-agent-sessions]] and [[workspace-session-focus-reference]].
