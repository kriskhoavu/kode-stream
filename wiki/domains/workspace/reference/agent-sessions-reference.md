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
<!-- keywords: session records, harness runs, event feed, run limit -->

[live] The Sessions view reads each kind from its owner and stores neither. Evidence: `web/src/features/canvas/CanvasSessionsView.tsx:58`.

| Kind        | Read from                                                                                         | Kept fresh by                                           |
|-------------|---------------------------------------------------------------------------------------------------|---------------------------------------------------------|
| Interactive | `GET /api/ai/session-records?workspaceId=&branch=` for the checked-out branch                     | A reload after Stop or Remove, and the Canvas refresh   |
| Harness     | `GET /api/harness/runs`, then `GET /api/harness/runs/{id}` for the 12 newest runs, working first | The harness event feed: a run whose revision moves is reread |

A harness row is one ticket: its runs among those 12, newest first, with their agent runs. A run without a task is a row of its own. A queued agent run is not shown. While the harness is off or its storage is unsupported, the list shows interactive sessions only. Evidence: `web/src/features/canvas/CanvasSessionsView.tsx:80`, `web/src/features/sessions/agentSessions.ts:7`.

## Grouping And Filters
<!-- chunkId: workspace-agent-sessions-reference-grouping -->
<!-- keywords: ticket key, kind filter, live only, text search -->

[live] Rows group by key. A harness row's key is its ticket key (`APP-664`). An interactive row's key is its plan identifier, else its plan title, intent or provider. A group is titled `key · title`. A harness card's badge reads Working, Waiting on you, Done, Failed or Stopped. Evidence: `web/src/features/sessions/agentSessions.ts:37`.

- Groups with a live row come first, then by title. Inside a group, live rows come first, then newest.
- **Live only** keeps an interactive session that is starting or running, and a harness session whose agent works.
- The text filter matches titles, ticket keys, branches, providers, agent roles, engines and models.
- The selection is remembered per workspace. Evidence: `web/src/features/sessions/useSessionBrowserSelection.ts:7`.

## Cards
<!-- chunkId: workspace-agent-sessions-reference-cards -->
<!-- keywords: kind tag, harness run title, remove action -->

[live] An interactive card shows its state (Starting, Running, Exited, Failed, Cancelled, Interrupted), provider and age, and offers Remove once ended. A harness card stands for a ticket. It is titled **Harness run**, or **Harness run · N runs** after a Reset. It shows the newest run's state, the agent in play (`Builder · round 2`), the rounds and the age. It offers no Remove. Evidence: `web/src/features/sessions/SessionCard.tsx:116`.

## Detail
<!-- chunkId: workspace-agent-sessions-reference-detail -->
<!-- keywords: terminal tab, conversation tab, changes source, run page -->

[live] Both kinds share the Changes panel. Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:27`, `web/src/features/sessions/SessionChangesPanel.tsx:8`.

|            | Interactive                                                                    | Harness                                                                                                    |
|------------|--------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------|
| Navigation | None                                                                           | Compact run stepper, round chips for a repeated step, a run picker after a Reset                           |
| First tab  | **Terminal**: the live embedded terminal                                       | **Conversation**: `GET /api/harness/agent-runs/{id}/transcript?limit=50`, reread every 5 s while it works |
| Changes    | `GET /api/sessions/{id}/changes` and `/diff?path=`: the checkout since launch  | `GET /api/harness/runs/{id}/changes` and `/diff?path=`: the run's worktree, then its branch                |
| Actions    | **Open on board**, **Open on card**, **Stop** (`DELETE /api/ai/sessions/{id}`) | **Open run page** (`/harness/runs/{id}`), **Stop** while the run works                                     |
| Header     | Plan, branch, launch commit, started, duration                                 | Ticket, run, branch, started, rounds, time, cost, then the chosen agent's role, round, state, engine, model |

The harness detail opens on the working agent, else the last to start, or on **Your review** when the run waits there. It follows the run until a person picks a step or round. Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:47`.

- Planner belongs to Plan, plan Checker to Plan check, Builder to Build, code Checker to Code check, fixer to Fix.
- A step without an agent (Start, plan approval, Test, hand-off) points to the run page.
- **Your review** shows the Review decision (Hand off, Back to Build, Back to Plan) and the person's own session in the run's worktree. That session runs on the run's branch, so the interactive list does not hold it.
- The conversation folds the harness's brief to one line. A tool call names a worktree file from the worktree root (`./plans/…`).

## Not supported
<!-- chunkId: workspace-agent-sessions-reference-not-supported -->
<!-- keywords: older runs, per-agent diff, harness on canvas, live count -->

[live] Runs beyond the 12 newest are not listed. Their agents stay on each run's page. A harness card's Changes are its run's, because the harness keeps one worktree per run. The header's live count and the Canvas count interactive sessions only.

## Known conflicts
<!-- chunkId: workspace-agent-sessions-reference-known-conflicts -->
<!-- keywords: review session, branch filter, missing session -->

- "The person's review session appears in the interactive list." It does not, because it runs on the run's branch.

## Related pages

See [[workspace-agent-sessions]], [[platform-harness-reference]] and [[workspace-session-focus-reference]].
