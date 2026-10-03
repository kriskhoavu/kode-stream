# Implementation Plan: PM-043 - Card Agent Sessions

## Overview

Name embedded Claude sessions after their conversation, discover and resume Claude transcripts, project live card
sessions onto the Canvas, then build the card's agent cluster, Agents tab, agent pane and dock hand-off on top.

## Execution Flow

```text
B1 Claude session naming ─> B2 Transcript discovery + resume ─┐
B3 Ephemeral Canvas nodes ────────────────────────────────────┤
F1 SessionTerminal + provider display + dock attach ──────────┼─> F2 Card cluster, Agents tab, pane ─> I1 Docs
```

## Terminology Lock

- `session` for kode-stream processes, `transcript` for Claude Code conversation files (never "external session").
- `ResumeClaudeSessionID` / `resumeClaudeSessionId` for the resume input.
- `ephemeral` for projected, unpersisted Canvas nodes.
- `agent` only in UI names: Agents tab, agent cluster, agent pane.

## Phases

| Phase | Name                                         | Track    | Status  |
|-------|----------------------------------------------|----------|---------|
| B1    | Claude session naming                        | Backend  | Done    |
| B2    | Transcript discovery, tail, and resume       | Backend  | Done    |
| B3    | Ephemeral Canvas nodes for live sessions     | Backend  | Done    |
| F1    | Shared terminal, provider display, dock      | Frontend | Done    |
| F2    | Card cluster, Agents tab, agent pane         | Frontend | Done    |
| I1    | Wiki and journey                             | Docs     | Done    |

### B1 Claude session naming

A `claude` provider launch generates a UUID, passes `--session-id <uuid>`, and uses `embedded-<uuid>` as the record
ID. The record therefore names its conversation without a schema change, and discovery can tell kode-stream's own
conversations from the user's.

### B2 Transcript discovery, tail, and resume

`ClaudeTranscripts(workspaceID)` reads `<claude config>/projects/<encoded workspace path>/*.jsonl` changed in the last
14 days, at most 50 files, keeping a file only when its `cwd` is the workspace path. Each transcript reports its
session ID, branch, first and last activity, message count, and the IDs of the workspace's items whose path or
identifier it mentions. Conversations already named by a session record are left out. `TranscriptTail` returns the
last user and assistant text turns of one transcript. `EmbeddedInput.ResumeClaudeSessionID` launches
`claude --resume <id>` through the existing guards and records `intent: resume`.

### B3 Ephemeral Canvas nodes

A live session with a plan reference, no placement, and a placed plan node is projected as an ephemeral session
node to the right of its plan. It disappears when the session stops; **Pin** places it.

### F1 Shared terminal, provider display, dock

`CanvasSessionTerminal` moves to `features/ai-session/SessionTerminal.tsx`. One provider display module replaces the
copied label maps and adds `AgentChip`. The dock accepts `attachEmbeddedSession` and brings the attached session to the front, as it does a started one.

### F2 Card cluster, Agents tab, agent pane

`useItemSessions` loads records and transcripts for the board. `PlanCard` shows the agent cluster; the drawer gains
the Agents tab and the agent pane; closing the drawer on a live session hands it to the dock;
`/workstream?itemId=&agent=` opens a session directly.

### I1 Wiki and journey

`workspace-card-agent-sessions` and its reference page record the delivered behaviour. The browser journey in
`automation/` has not been executed.
