# PM-043: Card Agent Sessions

## Overview

PM-043 puts a plan's agent sessions on its Workstream card. A card shows a chip for every live session working on it;
clicking a chip opens the card's drawer with that session's live terminal in the main pane, where the conversation
can be continued by typing. A card with no live session offers a launcher that starts one for that card. Claude Code
conversations held in the user's own terminal for the same workspace are found from Claude Code's transcript store,
shown read-only, and can be resumed into an embedded terminal with **Continue here**.

No new kind of session is introduced. Every session started or resumed from a card is an ordinary embedded session
with a plan reference, so it is also listed on the Sessions page and the Workbench Sessions tab and, while live, drawn
on the Canvas next to its plan.

## Glossary

| Term               | Meaning                                                                                   | Maps To (code)                         |
|--------------------|-------------------------------------------------------------------------------------------|----------------------------------------|
| Card session       | An embedded session whose plan reference is the card's item.                              | `ai.SessionRecord.PlanRef.ItemID`      |
| Live session       | A session whose record is `starting` or `running` and whose process binding exists.       | `ai.SessionRecordView.Live`            |
| Agent cluster      | The card's chips: one per live session, then `+N`, or a launcher when none is live.       | `AgentCluster`                         |
| Agents tab         | The card drawer's side tab listing the card's sessions and transcripts.                   | `AgentsTab`                            |
| Agent pane         | The drawer's main pane showing a live terminal or a transcript instead of the document.   | `AgentPane`                            |
| Claude transcript  | A Claude Code conversation file for the workspace under Claude Code's projects directory. | `ai.ClaudeTranscript`                  |
| Continue here      | Resuming a Claude transcript as an embedded session with `claude --resume`.               | `EmbeddedInput.ResumeClaudeSessionID`  |
| Dock hand-off      | Moving a live session from the closing drawer to the terminal dock, which keeps it alive. | `attachEmbeddedSession`                |
| Ephemeral node     | A Canvas session node projected for a live, unplaced card session; never persisted.       | `canvas.Node.Ephemeral`                |

## Components

| Layer      | Component                                        | Purpose                                                                      |
|------------|--------------------------------------------------|------------------------------------------------------------------------------|
| Service    | `ai.Service.StartEmbedded`                       | Names Claude sessions after their conversation; resumes a transcript.        |
| Service    | `ai.Service.ClaudeTranscripts` / `TranscriptTail`| Lists a workspace's Claude conversations by item; reads a bounded tail.      |
| Service    | `canvas.Service` projection                      | Adds ephemeral nodes for live, unplaced card sessions.                       |
| Controller | `internal/server/api/ai_controller.go`           | `GET /api/workspaces/{id}/claude-transcripts[/{sessionId}/tail]`.            |
| Frontend   | `features/workstream` agent cluster and tab      | Card chips, launcher, Agents tab, agent pane, dock hand-off.                 |
| Frontend   | `features/ai-session/SessionTerminal`            | One reattaching terminal shared by the card, Canvas and Sessions page.       |
| Frontend   | `features/ai-session/EmbeddedTerminalDock`       | Accepts an existing session as well as a newly started one.                  |

## Boundaries

- Transcript discovery runs in Local mode only; Cloud mode answers 404, because a hosted server has no user home.
- Transcripts are read on request and never stored. Session records still carry no prompt or output.
- A transcript is matched to an item only by the item's repository path or identifier appearing in it.
- Worktree isolation remains a non-goal: every session shares the workspace checkout.
