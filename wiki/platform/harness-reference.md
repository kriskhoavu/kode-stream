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
<!-- keywords: internal/harness, removable, composition, adapters, ports -->

[live] The harness is the `internal/harness` tree. Only the composition root, the HTTP transport and the SQL storage import it, so removing the tree and those references removes the feature. Evidence: `internal/server/server.go:103`, `internal/storage/storage.go:701`.

Adapters depend on ports and the domain; the application packages hold the pure parts (ranking, briefs and reports, scouting, the event feed, webhook payloads); the stage logic lives in the `harness` package itself, because each step needs the repository, the feed and the settings in one transaction.

## Storage And The Queue
<!-- chunkId: platform-harness-reference-storage -->
<!-- keywords: migration 12, conditional update, claim, lease, skip locked -->

[live] Migration 12 adds the harness tables to SQLite and Postgres. Every change is one conditional update on id, state and revision; zero rows means another writer moved first, which the API answers as 409. Evidence: `internal/storage/storage.go:701`, `internal/storage/sql_harness_repository.go:101`.

[live] The work row's state is the queue: a role loop claims the oldest queued agent run of its roles inside a transaction, with `FOR UPDATE SKIP LOCKED` on Postgres, under a lease it extends every 30 seconds. An expired lease interrupts the agent run and re-queues it while it has attempts left; start-up interrupts every running agent run. Evidence: `internal/storage/sql_harness_repository.go:509`, `internal/harness/lifecycle.go:15`.

The harness runs as one replica. SQLite has one connection, so code inside a transaction uses only the transaction's repository.

## Ranking
<!-- chunkId: platform-harness-reference-ranking -->
<!-- keywords: score, priority, label boost, pin, re-ranker -->

[live] A task's score is its source priority, label boosts, half a point per waiting day up to five, a hand-set override, and 1000 when pinned; each term is a readable reason. Evidence: `internal/harness/application/queue/ranker.go:46`.

[live] An optional model re-ranker may reorder only the unpinned tasks among the rules' top N, and only by a valid permutation of them. Evidence: `internal/harness/application/queue/ranker.go:126`.

[live] A source is Jira, GitHub, Bitbucket or a local folder. A local folder holds one Markdown file per task: the file name is the key, the first `# ` heading the title, and the rest the body. A relative folder is read inside the workspace's checkout. A workspace with a local folder on hands off by pushing its branch to its origin and opens no pull request. `scripts/harness-sandbox.sh init` builds a sandbox for trying the pipeline this way: a tiny Go module whose origin is a local bare repository, and a task folder with one small task. `next` resets the sandbox and adds a new task, because a done task never reopens. Evidence: `internal/harness/adapters/scout/local/local.go:49`, `internal/harness/stages.go:204`.

[live] `GET /api/harness/settings` returns the defaults with `workspaces`, one entry per workspace holding its own choices: `enabled`, `autoStart`, `requireReview`, `askQuestions`, `verificationCommands`, `scope`, `models` and `procedure`, each absent or empty switch following the defaults. `models` holds every role once `workspaceModels` is true; `modelPolicy` is then only what a new workspace starts from, and a save leaves it as it is. A save that omits `workspaces` keeps the stored ones. The Test step and the builder's gate run a workspace's test command in the worktree; a workspace with none runs no test there, and Test passes. Evidence: `internal/harness/dashboard.go:604`.

[live] `GET /api/workspaces/{id}/settings-file` reads the workspace's `.kode-stream/settings.yaml`: whether it exists, whether it is valid, the parts it holds, and what an export would write now. `POST …/settings-file/export` and `POST …/settings-file/import` take `{parts}`, any of `provider`, `pipeline`, `tests`, `scope`, `models`, `procedure` and `sources`. An export keeps the parts it does not name and refuses to overwrite an invalid file; an import refuses a part the file does not hold. Evidence: `internal/server/api/settings_file_controller.go:81`, `internal/server/api/settings_file_controller.go:112`.

[live] A task a source stops listing goes stale after three successful polls; a done or dismissed task never reopens. A cancelled task, whose run a person stopped, stays open: the Finder still refreshes it and makes it stale when its source stops listing it, and `POST /api/harness/tasks/{id}/start` puts it back in the queue for a new run, beside the runs it already had. Evidence: `internal/harness/application/scout/poller.go:23`, `internal/harness/queue.go:307`.

[live] `POST /api/harness/queue/clear` deletes every task no run has touched, discovered, ranked, stale or dismissed, in one workspace or all, and records `task.removed` for each; the next poll adds back what a source still returns. It is the way to drop what an old search query found instead of waiting for three polls. Evidence: `internal/harness/queue.go:212`.

## Stages
<!-- chunkId: platform-harness-reference-stages -->
<!-- keywords: review rounds, gate, qa twice, review, hand off, draft pr, resume -->

[live] A producer's report that is `blocked` or lists `open` questions stops its round at a `question` gate when the question gate is on: the stage waits in its round, the card and the "Needs you" strip on the board show what the agent asks, and the person's answer sends the same role out again with the questions and the answer in its brief; a rejection cancels the run. With the gate off, a blocked report blocks the run and open questions ride along onto the plan gate. The gate is on unless the settings, the workspace or the task says otherwise, the task's word winning. Evidence: `internal/harness/pipeline.go:208`, `internal/harness/domain/settings.go:132`.

[live] Plan and build each run up to three producer and reviewer rounds; a round with changes requested carries the findings into the next brief, and the last one blocks the run with a resume approval. A failed agent run retries within three attempts; a missing report gets one reminder. Evidence: `internal/harness/pipeline.go:298`, `internal/harness/pipeline.go:239`, `internal/harness/pipeline.go:142`.

[live] After each builder and tester run the harness reverts paths outside the scope, then runs the verification commands itself; a failing gate sends the builder back with the end of the log. Evidence: `internal/harness/build.go:56`, `internal/harness/build.go:89`.

[live] QA runs the verification and the runtime's smoke check twice; both must pass, with at most two tester rounds. The draft PR stage pushes the local `harness/<key>-<run>` branch as `feature/<key>-<run>`, the name branch hooks such as `feature/[A-Z0-9]+-\d+-.+` accept, and opens a draft pull request from it on GitHub or Bitbucket, retrying a minute apart. Evidence: `internal/harness/stages.go:82`, `internal/harness/stages.go:170`, `internal/harness/adapters/worktree/gitworktree/gitworktree.go:22`.

[live] Review follows QA when `requireReview` is on, the default. Its `review` gate takes `approve` (hand off), `reject` (back to Build) or `replan` (back to Plan); the last two need a reason. Any decision first commits what is uncommitted in the worktree as `Apply the review of …`. A hand-off goes straight to the draft PR when the run's commit is the one QA last verified (QA records it as `lastGate.head`), and otherwise through a QA stage marked `reviewed`, whose pass goes on to the draft PR. Back to Build or Plan starts a new stage of that kind carrying the reason as a blocking finding, and the run comes back to Review. `POST /api/harness/runs/{id}/review-session` starts a person's embedded session in the worktree of a run at Review, on its branch, with intent `harness_review`; any other run is refused. Evidence: `internal/harness/approvals.go:510`, `internal/harness/approvals.go:544`, `internal/harness/stages.go:106`, `internal/server/api/harness_controller.go:447`.

[live] What a person asks for when sending work back, at Review or plan approval, reaches the next producer as a blocking finding and every checker after it as a request on top of the issue, which the checker judges the work against and does not ask to undo. Evidence: `internal/harness/work.go:490`, `internal/harness/application/brief/brief.go:126`.

[live] A passed run's worktree is removed at once; a cancelled or failed run keeps its worktree for `retentionDays` and its branch after that. Evidence: `internal/harness/stages.go:283`.

## Engines And Briefs
<!-- chunkId: platform-harness-reference-engines -->
<!-- keywords: claude code, codex, opencode, process group, task-data -->

[live] Every engine runs in its own process group under one runner: cancel sends SIGTERM, then SIGKILL after five seconds, and the lease is the wall-clock limit. Evidence: `internal/harness/adapters/engine/process/stream.go:47`.

[live] Claude Code runs as `claude -p --output-format stream-json` with the model, turn limit, permission mode, tool lists, session id and spending limit as flags and the brief on stdin. Evidence: `internal/harness/adapters/engine/claudecode/claudecode.go:59`.

Codex runs `codex exec --json` with its sandbox standing in for the permission mode; OpenCode runs `opencode run --format json` with the brief attached as a file. Each engine reports the knobs it cannot pass.

[live] A brief has six parts, and every piece of outside text in it sits inside task-data delimiters after a sentence saying it is data. The agent's report is read from `.harness/report.json`, bounded to 64 KB and validated per role. Evidence: `internal/harness/application/brief/brief.go:52`, `internal/harness/application/brief/report.go:24`.

## Scope
<!-- chunkId: platform-harness-reference-scope -->
<!-- keywords: protected paths, plans folder, revert, advisory finding -->

[live] The planner may change only `plans/`; a builder and a tester may change the workspace's scope setting plus the plan folder. `.git/`, `.harness/`, `.claude/` and `.github/workflows/` are never in scope. Reverted paths become an advisory finding. Evidence: `internal/harness/build.go:19`.

## Board API
<!-- chunkId: platform-harness-reference-api -->
<!-- keywords: /api/harness, capability, event stream, last-event-id, reset -->

[live] The board's routes live under `/api/harness`; reads need the read capability, and every change needs the `harness` capability and, in Cloud mode, the CSRF header. Evidence: `internal/server/api/gin_routes.go:65`.

[live] `POST /api/harness/tasks/{id}/remove` deletes one finished or stopped task, done, cancelled, dismissed or stale, with its runs, stages, agent runs and approvals, and records `task.removed`; events stay as the log. It first removes a working copy a run still holds and keeps every branch. Any other task is refused with `invalid_request`. Evidence: `internal/harness/queue.go:227`.

[live] `GET /api/harness/runs/{id}/changes` lists the files a run changed with lines added and removed, and `GET /api/harness/runs/{id}/diff?path=` returns one changed file's diff, at most 512 KB; both read the run's worktree while it exists, else its `harness/*` branch or its `feature/*` branch as last pushed, and say which, so a finished run still shows its changes. A path the run did not change is 404. Evidence: `internal/harness/approvals.go:446`, `internal/harness/approvals.go:425`, `internal/harness/adapters/worktree/gitworktree/gitworktree.go:237`.

[live] `GET /api/harness/runs/{id}/plan` returns the files the run changed under `plans/`, at most 40 of 256 KB each and text only: from the run's worktree while it exists, else from its `harness/*` branch or its `feature/*` branch as last pushed, saying which, so the plan is readable before the merge and before the plan is approved and committed. Evidence: `internal/harness/approvals.go:419`, `internal/harness/adapters/worktree/gitworktree/gitworktree.go:167`.

[live] `GET /api/harness/events` replays stored events after `Last-Event-ID`, then streams live ones with a keepalive every 15 seconds; a subscriber that falls behind gets `reset` and refetches. Evidence: `internal/server/api/harness_sse.go:27`.

[live] A task's drawer opens on its Ticket tab, the Workstream page's Jira panel read by key in the task's workspace: `GET /api/workspaces/{id}/jira/issues/{key}` takes `?refresh=1`, and `…/attachments/{attachmentId}` serves an attachment of that issue. The harness stores none of it; a task carries only the tracker's status, priority, assignee and labels. Evidence: `internal/jira/jira_service.go:293`.

[live] The run drawer's Live tab, and the card's conversation pane, list the run's agent rounds and shows the chosen agent's conversation: `GET /api/harness/agent-runs/{id}/transcript?limit=` reads the last text turns of the coding tool's own transcript, whose path and session id the harness records when the CLI starts, and re-reads it every few seconds while the agent runs. The harness stores the path, never the turns, and the Workstream page's transcript list draws them. Evidence: `internal/harness/harness.go:377`.

The route list and every shape are in the PM-044 backend solution page. The feature itself is [[platform-harness]].
