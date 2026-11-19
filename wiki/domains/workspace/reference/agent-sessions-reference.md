---
slug: workspace-agent-sessions-reference
title: Interactive And Harness Sessions Reference
pageType: REFERENCE
topics: sessions, harness, agent space, api, session browser
summary: Reference for how Agent Space's Sessions view reads, groups, filters and shows interactive and harness sessions, and which routes each kind uses.
sourceRef: |
  PM-044 | Agent Space sessions
lastVerified: 2026-11-18
---

## Sources
<!-- chunkId: workspace-agent-sessions-reference-sources -->
<!-- keywords: session records, harness runs, run detail, event feed, run limit -->

[live] The Sessions view reads its two kinds from two owners and never stores either. Evidence: `web/src/features/canvas/CanvasSessionsView.tsx:67`.

| Kind        | Read from                                                                                              | Kept fresh by                                                                    |
|-------------|--------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------|
| Interactive | `GET /api/ai/session-records?workspaceId=&branch=`, the checked-out branch                             | A reload after Stop or Remove, and the Canvas's own refresh                      |
| Harness     | `GET /api/harness/runs`, then `GET /api/harness/runs/{id}` for each of the workspace's 12 newest runs, working runs first | The harness event feed: a run whose revision moves is read again                 |

A harness row is one ticket: the runs of its task among those 12, newest first, each with its agent runs. A run without a task forms a row of its own, keyed by its id. An agent run still queued is not counted or shown. While the harness is off or its storage is unsupported, the view passes no harness rows, and the list shows interactive sessions only, without the kind filter. Evidence: `web/src/features/canvas/CanvasSessionsView.tsx:89`, `web/src/features/sessions/agentSessions.ts:21`.

## Grouping And Filters
<!-- chunkId: workspace-agent-sessions-reference-grouping -->
<!-- keywords: plan key, ticket key, group title, run state, kind filter, live only, search -->

[live] Rows are grouped by key. A harness row's key is its ticket's key (`APP-664`); an interactive row's key is its plan's identifier when the Canvas knows the plan, else its plan title, intent or provider. Rows that share a key share a group, titled `key · title`. Each card states where it stands with the harness's status badge, the one a task's details use: Working, Waiting on you, Done, Failed or Stopped for a harness card. Evidence: `web/src/features/sessions/agentSessions.ts:55`.

Groups with a live row come first, then by title; inside a group, live rows first, then newest. **All · Interactive · Harness** filters by kind, **Live only** keeps an interactive session that is starting or running and a harness session whose agent is working, and the text filter matches titles, ticket keys, branches, providers, agent roles, engines and models. The selection is remembered per workspace. Evidence: `web/src/features/sessions/SessionBrowser.tsx:60`, `web/src/features/sessions/useSessionBrowserSelection.ts:16`.

## Cards
<!-- chunkId: workspace-agent-sessions-reference-cards -->
<!-- keywords: kind tag, glyph, harness glyph, changes asked, approved -->

[live] Every card wears its kind's tag while both kinds share the list. An interactive card shows its state (Starting, Running, Exited, Failed, Cancelled, Interrupted), provider and age, and offers Remove once it has ended. A harness card stands for a whole ticket. It is titled **Harness run**, or **Harness run · N runs** after a Reset, and shows the newest run's state, the agent in play (`Builder · round 2`), the run's rounds, and its age, on two lines that never wrap. It offers no Remove. Evidence: `web/src/features/sessions/SessionCard.tsx:126`.

## Detail
<!-- chunkId: workspace-agent-sessions-reference-detail -->
<!-- keywords: terminal, conversation, changes source, open run page, stop run -->

[live] The two kinds share the Changes panel and differ in everything else. Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:32`, `web/src/features/sessions/SessionChangesPanel.tsx:13`.

|            | Interactive                                                                 | Harness                                                                                                      |
|------------|-----------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------|
| Navigation | None: one session, one terminal                                              | The run's stepper (compact), then round chips for a step that ran more than once; a run picker after a Reset |
| First tab  | **Terminal**: the live embedded terminal, through its grant and channel      | **Conversation** of the chosen agent: `GET /api/harness/agent-runs/{id}/transcript?limit=50`, read again every 5 s while it works |
| Changes    | `GET /api/sessions/{id}/changes` and `/diff?path=`: the checkout since launch | `GET /api/harness/runs/{id}/changes` and `/diff?path=`: the run's worktree, then its branch                  |
| Actions    | **Open on board**, **Open on card**, **Stop** (`DELETE /api/ai/sessions/{id}`) | **Open run page** (`/harness/runs/{id}`), **Stop** while the run works, which cancels the whole run          |
| Header     | Plan, branch, launch commit, started, duration                               | Ticket, run, branch, started, rounds · time · cost; under the stepper, the chosen agent's role, round, state, engine and model, turns, time and cost |

The harness detail opens on the agent at work, else the last to start, or on Your review when the run waits there, and follows the run until a person picks a step or a round. Each agent belongs to one step: Planner to Plan, plan Checker to Plan check, Builder to Build, code Checker to Code check, fixer to Fix. A step without an agent (Start, plan approval, Test, hand-off) shows a line pointing to the run page. Your review instead shows the Review decision (Hand off, Back to Build, Back to Plan) and the person's own session in the run's worktree, with a button that starts it and tabs for the session's terminal and the run's changes. That session runs on the run's branch, so the interactive list, which shows the checkout's branch, does not hold it. A line under the stepper says that other questions and approvals happen on the run page. In the conversation, the brief the harness gave the agent is folded to one line that opens it, and a tool call names a file inside the run's worktree from the worktree's root (`./plans/…`). Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:20`.

## Not supported
<!-- chunkId: workspace-agent-sessions-reference-not-supported -->
<!-- keywords: older runs, per-agent diff, harness on canvas, live count -->

[live] Runs beyond the workspace's 12 newest are not listed; their agents remain on each run's page. A harness card's Changes are its chosen run's, because the harness keeps one worktree per run, not per round. The Agent Space header's live count and the Canvas count interactive sessions only.

## Related pages

See [[workspace-agent-sessions]], [[platform-harness-reference]], [[workspace-session-focus-reference]] and [[workspace-card-agent-sessions-reference]].
