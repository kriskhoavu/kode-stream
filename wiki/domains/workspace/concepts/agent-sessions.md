---
slug: workspace-agent-sessions
title: Interactive And Harness Sessions
pageType: CONCEPT
topics: workspace, sessions, harness, agent space, terminal
summary: Every agent conversation is a session of one of two kinds — interactive, which a person drives in a terminal, or harness, which the harness drives headless — and Agent Space lists both in one list.
sourceRef: |
  PM-044 | Agent Space sessions
lastVerified: 2026-11-18
---

## One Word, Two Kinds
<!-- chunkId: workspace-agent-sessions-kinds -->
<!-- keywords: interactive session, harness session, terminology, harness run -->

[live] A **session** is one agent conversation: Claude Code, Codex or OpenCode working on a workspace. The two kinds differ in who drives the agent. Evidence: `web/src/features/sessions/agentSessions.ts:5`.

|                        | Interactive session                                   | Harness session                                                         |
|------------------------|-------------------------------------------------------|-------------------------------------------------------------------------|
| Who drives it          | A person, who types into it                           | The harness, for one task                                               |
| Who starts it          | A person, from a card, a plan or the Canvas           | The harness, for a Planner, Checker or Builder round                    |
| Where it works         | The workspace's checkout                              | The run's own worktree, on its `harness/` branch                        |
| How it runs            | In a PTY that Kode Stream keeps alive                 | Headless: the engine streams its events                                 |
| What you watch         | Its live terminal                                     | Its conversation: what it said, thought and called                      |
| How you steer it       | Type into the terminal, or stop it                    | Answer its questions and approve its work on the run page, or stop the run |
| What it changed        | The checkout's changes since it launched              | Its run's changes: the worktree while it lives, the branch after        |
| What Kode Stream keeps | A safe record: workspace, plan, provider, branch, commit, state | The run's ledger: step, round, engine, model, turns, time, cost, verdict, findings |

A harness agent is never a session record, and an interactive session is never part of a run. They meet only in the views that list them. A **harness run** is the pipeline for one task. It holds one harness session per agent round.

## Interactive Sessions
<!-- chunkId: workspace-agent-sessions-interactive -->
<!-- keywords: pty, embedded terminal, checkout, plan reference -->

[live] A person starts an interactive session from a Workstream card, a plan or the Canvas. It runs in an embedded terminal on the checkout. Kode Stream keeps only a safe record, never the prompt, output or buffer. Several interactive sessions share the one checkout. See [[workspace-ai-session-launch]] and [[workspace-card-agent-sessions]].

## Harness Sessions
<!-- chunkId: workspace-agent-sessions-harness -->
<!-- keywords: planner, checker, builder, worktree, headless -->

[live] The harness starts one harness session per round: a Planner writes the plan, a Checker reviews it, a Builder writes the code, and a Checker reviews the change. Each runs headless in the run's worktree, so it never touches the person's checkout. A person steers the run by answering questions and deciding at its gates. Evidence: `internal/harness/adapters/engine/claudecode/claudecode.go:54`, `internal/harness/adapters/engine/codex/codex.go:59`, `internal/harness/adapters/engine/opencode/opencode.go:57`. See [[platform-harness]].

## One List In Agent Space
<!-- chunkId: workspace-agent-sessions-agent-space -->
<!-- keywords: sessions view, kind filter, live only, ticket group -->

[live] Agent Space's Sessions view lists both kinds, grouped by ticket or plan. Each card has an **Interactive** or **Harness** chip and a status badge. **All · Interactive · Harness** filters by kind, and **Live only** keeps running sessions. Evidence: `web/src/features/sessions/SessionBrowser.tsx:124`, `web/src/features/sessions/agentSessions.ts:37`.

- A ticket's harness sessions share one card, however many rounds they took. Evidence: `web/src/features/sessions/SessionCard.tsx:116`.
- An interactive card opens its terminal and Changes.
- A harness card opens a compact run stepper. A step opens its last round, and round chips pick an earlier one. The chosen agent's conversation and the run's Changes show below, with **Open run page** and **Stop**, which stops the whole run. Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:27`.
- Interactive sessions are listed for the checked-out branch. Harness sessions are listed for the workspace's newest runs, whatever is checked out. Evidence: `web/src/features/canvas/CanvasSessionsView.tsx:58`.

## Not supported
<!-- chunkId: workspace-agent-sessions-not-supported -->
<!-- keywords: harness terminal, per-agent changes, canvas node, live count -->

[live] A harness session has no terminal and cannot be removed from the list. Reset or Remove on the task removes its sessions. Its Changes are the whole run's. Harness sessions are not drawn on the Canvas, and the live count counts interactive sessions only. While the harness is off, the view lists interactive sessions without the kind filter.

## Known conflicts
<!-- chunkId: workspace-agent-sessions-known-conflicts -->
<!-- keywords: terminology, live agent session, task harness -->

- "A live agent session can be a harness agent." In this corpus a live agent session is an interactive session that still runs. "Task harness" and "harness agent session" mean a harness session.

## Related pages

See [[workspace-agent-sessions-reference]], [[platform-harness]] and [[workspace-session-focus]].
