# PM-044: Engineering Harness

## Overview

PM-044 automates the planner, coder and reviewer loop that people run today with the `feature-planning` skill. Small
single-role agents do the work and one orchestrator inside kode-stream moves each task through its stages. A person
approves the start of a task, its plan, and optionally its draft pull request, from the dashboard or from Webex.

The harness runs no language model of its own. Each coding role runs a coding CLI headless in a git worktree of its
own: Claude Code first, then Codex and OpenCode. The orchestrator re-runs every gate itself, and only the harness
commits and pushes. The work row's state column is the queue: a loop claims a row by a conditional transition, and a
lease that expires is recovered on the next tick.

## Glossary

| Term      | Meaning                                                                                        | Maps To (code)    |
|-----------|------------------------------------------------------------------------------------------------|-------------------|
| Task      | One issue found by a scout, ranked in the queue, worked by at most one run at a time.          | `domain.Task`     |
| Run       | One attempt to take a task from plan to draft pull request, on branch `harness/<key>-<short>`. | `domain.Run`      |
| Stage     | One step of a run: plan, plan approval, implement, QA, PR approval, draft PR.                  | `domain.Stage`    |
| Round     | One producer and reviewer pass inside a stage.                                                 | `domain.Round`    |
| Agent run | One headless CLI process for one role, with its usage, cost and structured report.             | `domain.AgentRun` |
| Approval  | A gate a person decides: start, plan, resume, draft PR.                                        | `domain.Approval` |
| Event     | An append-only record with a sequence number, streamed to the dashboard and the bot.           | `domain.Event`    |
| Engine    | The spawn-and-parse adapter for one coding CLI.                                                | `ports.Engine`    |
| Scout     | A reader of one issue source: Jira, GitHub, Bitbucket.                                         | `ports.Scout`     |
| Brief     | The six-part instruction the orchestrator composes for one agent run. Never stored.            | `brief.Compose`   |
| Report    | The agent's structured result, `harness.report/v1`, read from `.harness/report.json`.          | `brief.Report`    |

## Components

| Layer       | Component                                    | Purpose                                                           |
|-------------|----------------------------------------------|-------------------------------------------------------------------|
| Façade      | `internal/harness/harness.go`                | Configuration, construction, start and stop, the service methods. |
| Domain      | `internal/harness/domain`                    | Entities, state machines, policy, settings, repository interface. |
| Application | `internal/harness/application/*`             | Ranker, scheduler loops, pipeline stages, brief, feed, outbox.    |
| Ports       | `internal/harness/ports`                     | Engine, scout, worktree, pull request, notifier.                  |
| Adapters    | `internal/harness/adapters/*`                | Claude Code, Codex, OpenCode, scouts, git worktree, PRs, webhook. |
| Storage     | `internal/storage/sql_harness_repository.go` | SQLite and Postgres implementation of `domain.Repository`.        |
| Transport   | `internal/server/api/harness_*.go`           | Dashboard routes, bot routes, the event stream.                   |
| Frontend    | `web/src/features/harness`, `HarnessPage`    | Inbox, Pipeline, Queue as waves, run drawer, settings.            |

## Boundaries

- Only `internal/server` (composition), `internal/server/api` (transport) and `internal/storage` (SQL repository)
  import `internal/harness`. Deleting the tree and those three references removes the feature.
- The harness runs in Local mode only. In Cloud mode its mutating routes answer 503.
- The `datadir` storage option does not support the harness. Its routes report `storageSupported: false` and its
  mutating routes answer 503 with the hint to switch Storage to SQLite.
- Agents never commit, push or switch branches. The harness commits on `harness/*` branches only and never touches
  the baseline branch.
- No prompt and no transcript is stored. An agent run keeps the path of its CLI transcript only.
- Task bodies, findings and logs are untrusted data. Every brief wraps them in `<task-data>` delimiters.
- Budgets are an open point. Cost is recorded per agent run and summed. The only cost stop is the CLI's own
  `--max-budget-usd` per agent run.
- agent-plane is not used. The sorter's optional re-ranking calls an OpenAI-compatible chat endpoint directly.
- The chat-bot `/harness` commands and webhook receiver are a separate plan in that repository.
