---
slug: platform-harness-reference
title: Engineering Harness Reference
pageType: REFERENCE
topics: harness, queue, leases, stages, engines, scope
summary: Reference for the harness's storage, queue, state machines, stages, engine adapters, scope rules, and the board's API.
sourceRef: |
  PM-044 | Solution 01 backend
  PM-044 | Implementation plan
lastVerified: 2026-11-07
---

## Package And Ownership
<!-- chunkId: platform-harness-reference-package -->
<!-- keywords: internal/harness, removable, adapters, ports -->

[live] The harness is the `internal/harness` tree. Only the composition root, the HTTP transport and the SQL storage import it, so removing the tree and those references removes the feature. Evidence: `internal/server/server.go:101`, `internal/storage/storage.go:253`.

Adapters depend on ports and the domain. Application packages hold the pure parts: ranking, briefs and reports, scouting, the event feed and webhook payloads. Stage logic lives in the `harness` package, because each step needs the repository, the feed and the settings in one transaction.

## Storage And The Queue
<!-- chunkId: platform-harness-reference-storage -->
<!-- keywords: migration 12, conditional update, lease, skip locked -->

[live] Migration 12 adds the harness tables to SQLite and Postgres. Every change is one conditional update on ID, state and revision. Zero rows means another writer won, and the API returns 409. Evidence: `internal/storage/sql_harness_repository.go:91`.

[live] A role loop claims the oldest queued agent run of its roles in a transaction (`FOR UPDATE SKIP LOCKED` on Postgres), under a lease it extends every 30 seconds. An expired lease interrupts the agent run and requeues it while attempts remain. Start-up interrupts every running agent run. Evidence: `internal/storage/sql_harness_agent_runs.go:98`, `internal/harness/lifecycle.go:13`.

The harness runs as one replica. SQLite has one connection, so code inside a transaction uses only that transaction's repository.

## Ranking And Sources
<!-- chunkId: platform-harness-reference-ranking -->
<!-- keywords: score, pin, re-ranker, local folder, stale task -->

[live] A task's score is its source priority, label boosts, half a point per waiting day up to five, a manual override, and 1000 when pinned. Each term is a readable reason. An optional model re-ranker may only permute the unpinned tasks among the top N. Evidence: `internal/harness/application/queue/ranker.go:42`, `internal/harness/application/queue/ranker.go:116`.

[live] A source is Jira, GitHub, Bitbucket or a local folder. Evidence: `internal/harness/adapters/scout/local/local.go:40`, `internal/harness/domain/entities.go:12`.

- A local folder holds one Markdown file per task: the file name is the key, the first `# ` heading the title, the rest the body. A relative folder resolves inside the checkout.
- A workspace with a local folder hands off by pushing its branch to origin. It opens no pull request.
- `scripts/harness-sandbox.sh init` builds a sandbox: a tiny Go module with a local bare origin and one task. `next` resets it and adds a new task.

[live] A task its source stops listing goes stale after three successful polls. A done or dismissed task never reopens. A cancelled task stays open, and `POST /api/harness/tasks/{id}/start` queues a new run for it. Evidence: `internal/harness/application/scout/poller.go:18`, `internal/harness/queue.go:192`.

[live] `POST /api/harness/queue/clear` deletes every task no run has touched (discovered, ranked, stale or dismissed) in one workspace or all, and records `task.removed`. The next poll adds back what a source still returns. Evidence: `internal/harness/task_lifecycle.go:161`.

## Settings API
<!-- chunkId: platform-harness-reference-settings -->
<!-- keywords: workspaces entry, workspaceModels, settings file parts -->

[live] `GET /api/harness/settings` returns the defaults plus `workspaces`, one entry per workspace: `enabled`, `autoStart`, `requireReview`, `askQuestions`, `verificationCommands`, `scope`, `models` and `procedure`. An absent switch follows the default. Evidence: `internal/harness/dashboard_settings.go:25`.

- With `workspaceModels` true, `models` holds every role, and `modelPolicy` is only what a new workspace starts from.
- A save that omits `workspaces` keeps the stored ones.
- The Test step and the builder's gate run the workspace's test command. With none, Test passes.

[live] `GET /api/workspaces/{id}/settings-file` reads `.kode-stream/settings.yaml`: whether it exists and is valid, its parts, and what an export would write. `POST …/settings-file/export` and `POST …/settings-file/import` take `{parts}` from `provider`, `pipeline`, `tests`, `scope`, `models`, `procedure` and `sources`. Export keeps unnamed parts and refuses to overwrite an invalid file. Import refuses a part the file lacks. Evidence: `internal/server/api/settings_file_controller.go:77`.

## Stages
<!-- chunkId: platform-harness-reference-stages -->
<!-- keywords: review rounds, question gate, clarify gate, draft pr -->

[live] Question gate: a producer report that is `blocked` or lists `open` questions stops its round at a `question` gate. The card and **Needs you** show the questions. The answer sends the same role out again with questions and answer in its brief. A rejection cancels the run. With the gate off, a blocked report blocks the run and open questions go to the plan gate. The gate is on unless settings, workspace or task turn it off, and the task wins. Evidence: `internal/harness/pipeline.go:190`, `internal/harness/domain/settings.go:268`.

[live] Clarify gate: a Jira source's `checkReady` (absent means off) and `readyStatuses` (empty means `READY`, any case) appear under Integrations as "Start ready tickets on their own" and "Ready statuses". Evidence: `internal/harness/clarify.go:12`, `internal/harness/domain/settings.go:51`, `web/src/features/harness/settingsSources.tsx:47`.

- A ticket in a ready status starts on its own, whatever Auto-start says.
- The planner's brief adds the reporter and latest comments. If the ticket is not plannable, the planner reports `needs_info` with the problem and questions, and writes no plan.
- The run waits at a `clarify` gate. The task sets `waitingFor` to the reporter and `needsYou` false. The gate holds a draft comment that mentions the reporter: `[~accountid:…]` on Jira Cloud, `[~name]` on Jira Server.
- After each poll, a new comment by anyone but the assignee decides the gate on channel `source` and reruns the planner. **Continue anyway** with a note does the same and ends the check. **Dismiss** with a reason cancels the run.
- After three replies the check closes, and further questions use the `question` gate.
- A clarify gate is not listed under **Needs you**.

[live] Plan and build each run up to three producer and reviewer rounds. Requested changes carry into the next brief. The last round blocks the run with a resume approval. A failed agent run retries within three attempts. A missing report gets one reminder. Evidence: `internal/harness/pipeline.go:378`, `internal/harness/pipeline.go:133`.

[live] After each builder and tester run, the harness reverts paths outside scope and runs the verification commands. A failure sends the builder back with the end of the log. Evidence: `internal/harness/build.go:50`.

[live] QA runs the verification and the runtime smoke check twice. Both must pass, with at most two tester rounds. The draft PR stage pushes `harness/<key>-<run>` as `feature/<key>-<run>`, which matches hooks such as `feature/[A-Z0-9]+-\d+-.+`, and opens a draft pull request on GitHub or Bitbucket, retrying a minute apart. Evidence: `internal/harness/stages.go:73`, `internal/harness/adapters/worktree/gitworktree/gitworktree.go:21`.

[live] Review follows QA while `requireReview` is on. Its `review` gate takes `approve` (hand off), `reject` (back to Build) or `replan` (back to Plan). The last two need a reason. Evidence: `internal/harness/approvals.go:304`, `internal/harness/pipeline.go:238`, `internal/harness/approvals.go:52`.

- Every decision first commits uncommitted worktree changes as `Apply the review of …`.
- A hand-off goes straight to the draft PR when the commit is the one QA last verified (`lastGate.head`). Otherwise a QA stage marked `reviewed` runs first.
- Back to Build or Plan starts that stage with the reason as a blocking finding.
- `POST /api/harness/runs/{id}/review-session` starts the person's embedded session in the worktree, on the run's branch, with intent `harness_review`. Only a run at Review accepts it.

[live] A request sent back at Review or plan approval reaches the next producer as a blocking finding, and every later checker as a request on top of the issue. Evidence: `internal/harness/approvals.go:332`.

[live] A passed run's worktree is removed at once. A cancelled or failed run keeps its worktree for `retentionDays`, and its branch after that. Evidence: `internal/harness/stages.go:271`.

## Engines And Briefs
<!-- chunkId: platform-harness-reference-engines -->
<!-- keywords: claude code, codex, opencode, process group, task-data -->

[live] Each engine runs in its own process group. Cancel sends SIGTERM, then SIGKILL after five seconds. The lease is the wall-clock limit. Evidence: `internal/harness/adapters/engine/process/process.go:47`.

[live] Claude Code runs as `claude -p --output-format stream-json`, with model, turn limit, permission mode, tool lists, session ID and spending limit as flags and the brief on stdin. Evidence: `internal/harness/adapters/engine/claudecode/claudecode.go:53`.

Codex runs `codex exec --json`, with its sandbox in place of the permission mode. OpenCode runs `opencode run --format json` with the brief attached as a file. Each engine reports the settings it cannot pass.

[live] A brief has six parts. Outside text sits inside task-data delimiters after a sentence saying it is data. The report is read from `.harness/report.json`, capped at 64 KB and validated per role. Evidence: `internal/harness/application/brief/compose.go:10`.

## Scope
<!-- chunkId: platform-harness-reference-scope -->
<!-- keywords: protected paths, plans folder, revert, advisory finding -->

[live] The planner may change only `plans/`. A builder and a tester may change the workspace's scope plus the plan folder. `.git/`, `.harness/`, `.claude/` and `.github/workflows/` are never in scope. Reverted paths become an advisory finding. Evidence: `internal/harness/build.go:18`.

## Board API
<!-- chunkId: platform-harness-reference-api -->
<!-- keywords: capability, event stream, last-event-id, transcript -->

[live] Board routes live under `/api/harness`. Reads need the read capability. Changes need the `harness` capability and, in Cloud mode, the CSRF header. Evidence: `internal/server/api/gin_routes.go:63`.

| Route                                               | Behavior                                                                                       |
|-----------------------------------------------------|------------------------------------------------------------------------------------------------|
| `POST /api/harness/tasks/{id}/remove`               | Stops any run, deletes the task with runs, stages, agent runs, approvals, branch sessions and worktrees. Records `task.removed`. Events stay |
| `GET /api/harness/runs/{id}/changes`                | Files the run changed, with lines added and removed                                            |
| `GET /api/harness/runs/{id}/diff?path=`             | One file's diff, at most 512 KB. An unchanged path is 404                                      |
| `GET /api/harness/runs/{id}/plan`                   | Changed files under `plans/`, at most 40 text files of 256 KB each                             |
| `GET /api/harness/events`                           | Replays events after `Last-Event-ID`, then streams, with a keepalive every 15 s. A slow subscriber gets `reset` |
| `GET /api/harness/agent-runs/{id}/transcript?limit=`| Last text turns of the CLI's own transcript. The harness stores only its path                  |
| `GET /api/workspaces/{id}/jira/issues/{key}`        | The task's Ticket tab. `?refresh=1` refetches. A task stores only status, priority, assignee and labels |

Changes, diff and plan read the worktree while it exists, else the `harness/*` branch, else the pushed `feature/*` branch, and say which. Evidence: `internal/harness/task_lifecycle.go:31`, `internal/harness/run_inspection.go:73`, `internal/server/api/harness_sse.go:24`, `internal/harness/reads.go:95`, `internal/jira/jira_service.go:109`.

The feature itself is [[platform-harness]].
