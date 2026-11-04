# Solution 02: Frontend

The dashboard is built in `web/` by a separate session on its own branch. This page records the agreement it builds
against; the API contract it calls is in [solution-01-backend.md](solution-01-backend.md#api-contract).

## Navigation

- A fourth navigation button, **Harness**, after Agent Space, with a count of approvals waiting.
- Routes: `/harness` (Inbox), `/harness/queue` (Queue as waves), `/harness/runs/:id?tab=` (Pipeline),
  `/harness/approvals`, `/harness/settings`. The page works across workspaces.

## Screens

| Screen   | Route                  | Calls                                                                                |
|----------|------------------------|--------------------------------------------------------------------------------------|
| Inbox    | `/harness`             | `GET /status`, `GET /approvals`, `GET /runs`, `GET /cost?groupBy=day`, event stream  |
| Pipeline | `/harness/runs/:id`    | `GET /runs/:id`, `GET /agent-runs/:id`, `GET /runs/:id/changes`, approval decide     |
| Queue    | `/harness/queue`       | `GET /queue`, `POST /tasks/:id/{pin,priority,dismiss,start}`, `POST /scout/poll`     |
| Settings | `/harness/settings`    | `GET/PUT /settings`, `GET /status` (engines), `POST /roles/:role/{pause,resume}`     |

## Vocabulary

One module, `features/harness/vocabulary.ts`, maps API states to plain words. Statuses: Not started, Up next,
Working, Waiting on you, Done, Needs a fix, Didn't pass, Stopped, Skipped, Done by hand. Stages: Plan, Plan check,
Your approval, Build, Code check, Test, Fix, Draft pull request. Roles: Finder (scout), Sorter (sorter), Planner
(planner), Checker (reviewer), Builder (coder), Tester (qa_fixer).

## State

A pure reducer takes a snapshot and then events from `GET /api/harness/events`. The browser `EventSource` sends
`Last-Event-ID`; on `reset` the page refetches its snapshot. When the stream fails for 10 seconds the page polls every
10 seconds and retries the stream every 30 seconds. An optimistic approval is reconciled by the later
`approval.decided` event; a 409 `already_decided` counts as success.

## Gates

`npm run typecheck`, `npm test`, `npm run build`. The phases F1–F6 are listed in
[implementation-plan.md](../implementation-plan.md).
