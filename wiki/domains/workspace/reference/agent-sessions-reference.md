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

A harness row is one agent run of a run's detail, under the step the run page names (`Plan`, `Build 2`); an agent run still queued is not listed. While the harness is off or its storage is unsupported, the view passes no harness rows, and the list shows interactive sessions only, without the kind filter. Evidence: `web/src/features/canvas/CanvasSessionsView.tsx:80`, `web/src/features/harness/RunDrawer.tsx:362`.

## Grouping And Filters
<!-- chunkId: workspace-agent-sessions-reference-grouping -->
<!-- keywords: plan key, ticket key, group title, run state, kind filter, live only, search -->

[live] Rows are grouped by key. A harness row's key is its task's source key (`APP-664`); an interactive row's key is its plan's identifier when the Canvas knows the plan, else its plan title, intent or provider. Rows that share a key share a group, titled `key · title`, with the newest run's state beside it: Working, Waiting on you, Done or Stopped. Evidence: `web/src/features/sessions/agentSessions.ts:46`.

Groups with a live row come first, then by title; inside a group, live rows first, then newest. **All · Interactive · Harness** filters by kind, **Live only** keeps an interactive session that is starting or running and a harness session whose agent is working, and the text filter matches titles, ticket keys, steps, providers, engines and models. The selection is remembered per workspace. Evidence: `web/src/features/sessions/SessionBrowser.tsx:60`, `web/src/features/sessions/useSessionBrowserSelection.ts:16`.

## Cards
<!-- chunkId: workspace-agent-sessions-reference-cards -->
<!-- keywords: kind tag, glyph, harness glyph, changes asked, approved -->

[live] Every card wears its kind's tag while both kinds share the list. An interactive card shows its state (Starting, Running, Exited, Failed, Cancelled, Interrupted), provider and age, and offers Remove once it has ended. A harness card is titled `Role · round N` and shows Working, Queued, Done, Approved, Changes asked, Failed, Interrupted or Stopped, its step, its engine and model, and its age; it offers no Remove. Evidence: `web/src/features/sessions/SessionCard.tsx:71`.

## Detail
<!-- chunkId: workspace-agent-sessions-reference-detail -->
<!-- keywords: terminal, conversation, changes source, open run page, stop run -->

[live] The two kinds share the Changes panel and differ in everything else. Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:22`, `web/src/features/sessions/SessionChangesPanel.tsx:13`.

|            | Interactive                                                                 | Harness                                                                                                      |
|------------|-----------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------|
| First tab  | **Terminal**: the live embedded terminal, through its grant and channel      | **Conversation**: `GET /api/harness/agent-runs/{id}/transcript?limit=50`, read again every 5 s while it works |
| Changes    | `GET /api/sessions/{id}/changes` and `/diff?path=`: the checkout since launch | `GET /api/harness/runs/{id}/changes` and `/diff?path=`: the run's worktree, then its branch                  |
| Actions    | **Open on board**, **Open on card**, **Stop** (`DELETE /api/ai/sessions/{id}`) | **Open run page** (`/harness/runs/{id}`), **Stop** while the run works, which cancels the whole run          |
| Header     | Plan, branch, launch commit, started, duration                               | Task, step, branch, engine and model, started, turns · time · cost                                            |

A harness session's detail also says that the harness drives it and that questions and approvals happen on the run page.

## Not supported
<!-- chunkId: workspace-agent-sessions-reference-not-supported -->
<!-- keywords: older runs, per-agent diff, harness on canvas, live count -->

[live] Runs beyond the workspace's 12 newest are not listed; their agents remain on each run's page. A harness session's Changes are its run's, because the harness keeps one worktree per run, not per agent. The Agent Space header's live count and the Canvas count interactive sessions only.

## Related pages

See [[workspace-agent-sessions]], [[platform-harness-reference]], [[workspace-session-focus-reference]] and [[workspace-card-agent-sessions-reference]].
