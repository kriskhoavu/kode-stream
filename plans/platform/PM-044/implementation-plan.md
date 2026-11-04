# Implementation Plan: PM-044 - Engineering Harness

## Overview

Build the harness backend in `internal/harness/` in seven phases, each landing alone with its own verification
command, beside a dashboard built in `web/` by a separate session. Documentation lands last.

## Execution Flow

```text
P0 Plan ─> B1 Core ─> B2 Scout + sorter ─> B3 Claude Code + plan stage ─> B4 Implement ─> B5 QA + draft PR ─> B7 Engines + ranker ─> I1 Docs
                │                                  └──────────────────────> B6 Bot + webhooks ─────────────────────┘
                └─> F1 Inbox ─> F2 Stepper ─┐
                          └─> F3 Waves ─────┴─> F4 Drawer ─> F6 Polish
                          └─> F5 Settings (after B3) ───────┘
```

## Terminology Lock

- `task` for an issue in the queue, `run` for one attempt, `stage`, `round`, `agent run`; never "job".
- `approval` and `gate`: a gate is the kind (`start`, `plan`, `resume`, `draft_pr`), an approval is one decision.
- `engine` for a coding CLI adapter; `scout` for a source reader; `brief` for an agent's instruction.
- Role keys for pause and health: `scout`, `sorter`, `planner`, `reviewer`, `coder`, `qa_fixer`.

## Delegation

| Wave | Backend (this session)                 | Frontend (separate session)            |
|------|----------------------------------------|----------------------------------------|
| 0    | P0 Plan folder                         | —                                      |
| 1    | B1 Core                                | F1 Inbox + queue list                  |
| 2    | B2 Scout + sorter + queue operations   | F2 Stepper + ledger                    |
| 3    | B3 Claude Code engine + plan stage     | F3 Wave board + dependency curves      |
| 4    | B4 Implement stage                     | F4 Run drawer + live session           |
| 5    | B5 QA + draft PR                       | F5 Settings + model policy             |
| 6    | B6 Bot surface + webhooks              | F6 Polish + motion                     |
| 7    | B7 More engines + LLM ranker, I1 Docs  | —                                      |

## Phases

| Phase | Name                                         | Track    | Depends on | Status  |
|-------|----------------------------------------------|----------|------------|---------|
| P0    | Plan folder                                  | Docs     | —          | Done    |
| B1    | Core                                         | Backend  | P0         | Done    |
| B2    | Scout, sorter, queue operations              | Backend  | B1         | Done    |
| B3    | Claude Code engine, plan stage, plan gate    | Backend  | B2         | Done    |
| B4    | Implement stage                              | Backend  | B3         | Done    |
| B5    | QA and draft PR                              | Backend  | B4         | Done    |
| B6    | Bot surface and webhooks                     | Backend  | B3         | Done    |
| B7    | More engines, policy editing, LLM ranker     | Backend  | B5         | Done    |
| F1–F6 | Dashboard                                    | Frontend | B1 contract| Separate session |
| I1    | Documentation                                | Docs     | all        | Pending |

### P0 Plan folder

Deliverables: `plan.yaml`, `README.md`, `scenario/scenario-00-overview.md`, the three solution pages,
`automation/README.md`, this page.
Verification: `plan.yaml` indexes on the board (`go test ./internal/workspace/scanner/...`).
Commit: `PM-044: Plan the engineering harness`.

### B1 Core

Deliverables: `internal/harness/domain` (entities, transitions, policy, settings, repository interface),
`internal/harness/application/feed`, `internal/harness/harness.go`; migration v12 `harness_core` for SQLite and
Postgres and `internal/storage/sql_harness_repository.go`; `models.CapabilityHarness` in models, `cloud_auth.go` and
`internal/system`; owners `harness` and `harness-bot` in `policyForRoute` and the route inventory; the GET routes
for status, settings, tasks, queue, runs, approvals, agent runs; `PUT /harness/settings`; the event stream;
composition in `internal/server/server.go`.
Verification: `go test ./internal/harness/... ./internal/storage/... ./internal/server/...`.
Commit: `PM-044: Add the harness core, storage and read API`.

### B2 Scout, sorter, queue operations

Deliverables: `jira.Client.SearchIssues`; Jira, GitHub and Bitbucket scouts; `RuleRanker`; the orchestrator with
the scout, sorter, dispatcher-less recovery loops and `Start`/`Stop` in Local mode; pin, priority, dismiss, start and
scout poll routes.
Verification: `go test ./internal/harness/... ./internal/jira/... ./internal/server/api/...`.
Commit: `PM-044: Fill and rank the harness queue from issue sources`.

### B3 Claude Code engine, plan stage, plan gate

Deliverables: engine port; Claude Code adapter tested against a fake `claude` script; git worktree methods on the
git adapter and the `gitworktree` adapter; brief composer and report parser; pipeline with the plan and approval
stages; dispatcher and the planner and reviewer loops; decide, pause, resume, cost, agent-run, changes and cancel
routes; harness paths in `cloudHostedExecutionRoute`; exported `ai.ClaudeTranscriptPath`.
Verification: `go test ./internal/harness/... ./internal/git/... ./internal/ai/... ./internal/server/api/...`.
Commit: `PM-044: Run the planner through Claude Code up to the plan approval`.

### B4 Implement stage

Deliverables: coder and code reviewer rounds, the difficulty override, the harness gate re-run, scope enforcement
with revert, the harness commit on the run branch.
Verification: `go test ./internal/harness/...`.
Commit: `PM-044: Build and review each plan in the implement stage`.

### B5 QA and draft PR

Deliverables: QA stage (verification run twice, qa fixer with an 8 KB log tail, at most 2 fix rounds), PR approval,
draft PR stage, GitHub and Bitbucket PR adapters, worktree janitor, audit records.
Verification: `go test ./internal/harness/... ./internal/runtime/...`.
Commit: `PM-044: Test each run and open a draft pull request`.

### B6 Bot surface and webhooks

Deliverables: bot controller (503, 401, 200 replay, 409 tests), outbox, webhook notifier with HMAC signature,
the bot contract page.
Verification: `go test ./internal/harness/... ./internal/server/api/...`.
Commit: `PM-044: Serve the harness to chat-bot and deliver signed webhooks`.

### B7 More engines, policy editing, LLM ranker

Deliverables: Codex and OpenCode adapters with `Detect`; model policy validation against `max`; the sorter's
bounded re-ranking over an OpenAI-compatible endpoint, the local Qwen by default.
Verification: `go test ./internal/harness/...`.
Commit: `PM-044: Add the Codex and OpenCode engines and the model re-ranker`.

### I1 Documentation

Deliverables: `docs/architecture/README.md` entry, `wiki/platform/harness.md` and its reference page, the
regenerated `wiki/index.md`, the `AGENTS.md` ownership line, the atlas's harness nodes in the fen workspace.
Verification: `make verify` in kode-stream; `docs/atlas/build.sh --check` in fen.
Commit: `PM-044: Document the engineering harness`.

## Out Of Scope

- Budgets (an open point): per-run and per-day limits, a budget gate, a spend meter.
- The chat-bot `/harness` commands and webhook receiver: a separate plan in that repository.
