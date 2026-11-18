---
slug: workspace-agent-sessions
title: Interactive And Harness Sessions
pageType: CONCEPT
topics: workspace, sessions, harness, agent space, terminal, conversation
summary: Every agent conversation is a session of one of two kinds — interactive, which a person drives in a terminal, or harness, which the harness drives headless — and Agent Space lists both in one list.
sourceRef: |
  PM-044 | Agent Space sessions
lastVerified: 2026-11-18
---

## One Word, Two Kinds
<!-- chunkId: workspace-agent-sessions-kinds -->
<!-- keywords: session, interactive session, harness session, agent, conversation, terminology -->

[live] A **session** is one agent conversation: a coding agent (Claude Code, Codex or OpenCode) working on a workspace from start to end. Kode Stream has two kinds, and the difference is who drives the agent. Evidence: `web/src/features/sessions/agentSessions.ts:15`.

|                    | Interactive session                                                | Harness session                                                                     |
|--------------------|--------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| Who drives it      | A person, who types into it                                        | The harness, for one task                                                           |
| Who starts it      | A person, from a card, a plan or the Canvas                        | The harness, when a task reaches a step (a Planner, Checker or Builder round)       |
| Where it works     | The workspace's checkout, on the branch that is checked out        | The run's own worktree, on its `harness/` branch                                    |
| How it runs        | In a terminal (a PTY) that Kode Stream keeps alive                 | Headless: the engine runs without a terminal and streams its events                 |
| What you watch     | Its live terminal                                                  | Its conversation: what it said, thought and called                                  |
| How you steer it   | Type into the terminal; stop it                                    | Answer its questions and approve its work on the run page; stop the whole run       |
| What it changed    | The checkout's changes since it launched                           | Its run's changes: the worktree while it lives, the branch after                    |
| What Kode Stream keeps | A safe record: workspace, plan, provider, branch, commit, state | The run's ledger: step, round, engine, model, turns, time, cost, verdict, findings  |

The two kinds are separate in storage: a harness agent is never a session record, and an interactive session is never part of a run. They meet only in the views that list them.

Use these words. "Live agent session" means an interactive session that is still running; "task harness" or "harness agent session" means a harness session; a **harness run** is the pipeline for one task and holds several harness sessions, one per agent round. Agent Space shows a ticket's harness sessions as one card.

## Interactive Sessions
<!-- chunkId: workspace-agent-sessions-interactive -->
<!-- keywords: terminal, pty, embedded session, checkout, plan reference, cancel -->

[live] A person starts an interactive session from a Workstream card, a plan or the Canvas. It runs in an embedded terminal on the workspace's checkout and stays alive while someone watches it, so the person can keep typing to it. Kode Stream stores only a safe record of it, never its prompt, output or terminal buffer. Several interactive sessions share the one checkout. See [[workspace-ai-session-launch]], [[workspace-card-agent-sessions]] and [[workspace-session-focus]].

## Harness Sessions
<!-- chunkId: workspace-agent-sessions-harness -->
<!-- keywords: planner, checker, builder, round, worktree, headless, transcript, approval -->

[live] The harness starts a harness session for each round of a step: a Planner writes the plan, a Checker reviews it, a Builder writes the code, a Checker reviews the change. Each runs headless, through the engine's non-interactive mode, in the run's own worktree, so it never touches the checkout a person works in and several can run at once. Nobody types into it. A person steers the run instead: answering the questions an agent stops to ask, and approving or rejecting at the run's gates. Its conversation is read from the engine's event stream. Evidence: `internal/harness/adapters/engine/claudecode/claudecode.go:60`, `internal/harness/adapters/engine/codex/codex.go:65`, `internal/harness/adapters/engine/opencode/opencode.go:63`. See [[platform-harness]].

## One List In Agent Space
<!-- chunkId: workspace-agent-sessions-agent-space -->
<!-- keywords: agent space, sessions view, kind filter, group, live only, conversation tab -->

[live] Agent Space's Sessions view lists both kinds in one list, grouped by the ticket or plan they belong to. An interactive session on a plan and the harness card for the ticket with the same key share one group, which names the run's state beside its title. Each card carries an **Interactive** or **Harness** tag; **All · Interactive · Harness** narrows the list to one kind, and **Live only** keeps the running ones of either kind. Evidence: `web/src/features/sessions/SessionBrowser.tsx:60`, `web/src/features/sessions/agentSessions.ts:55`.

A ticket's harness sessions share **one card**, however many rounds they took, so a ticket the Planner and Checker went back and forth on does not fill the list. The card names the agent in play and the run's rounds and cost. Evidence: `web/src/features/sessions/SessionCard.tsx:126`.

Picking an interactive session shows its terminal and its Changes. Picking a harness card shows the run's stepper, the one the run page shows, made compact. It opens on the agent at work, else the last one that ran. Clicking a step opens that step's last round; where a step ran more than once, round chips under the stepper pick an earlier one. The chosen agent's conversation and the run's Changes show below, with **Open run page** and, while the run works, **Stop**, which stops the whole run. A step no agent works in, such as an approval or the tests, says so and points to the run page. A ticket with more than one run, after a Reset, offers a run picker. Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:32`.

Interactive sessions follow the checkout: the view lists those launched on the branch that is checked out. Harness sessions do not, because each works on a branch of its own: the view lists the tickets of the workspace's newest runs whatever is checked out, and names each run's branch. Evidence: `web/src/features/canvas/CanvasSessionsView.tsx:67`.

## Not supported
<!-- chunkId: workspace-agent-sessions-not-supported -->
<!-- keywords: typing into harness, harness terminal, per-agent changes, canvas node, global list -->

[live] A harness session has no terminal and cannot be typed into, and it cannot be removed from the list: Reset on the task removes a run's sessions. Its Changes are the whole run's, not one agent's. Harness sessions are not drawn on the Canvas, and the Sessions view's live count counts interactive sessions only. While the harness is off, the view lists interactive sessions alone, without the kind filter.

## Related pages

See [[workspace-agent-sessions-reference]], [[platform-harness]], [[workspace-session-focus]], [[workspace-card-agent-sessions]] and [[workspace-terminal-canvas]].
