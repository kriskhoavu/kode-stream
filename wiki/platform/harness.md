---
slug: platform-harness
title: Engineering Harness
pageType: CONCEPT
topics: harness, agents, orchestration, approvals, queue, coding cli
summary: The harness finds issues, ranks them, and runs coding CLIs in worktrees from plan to draft pull request, with a person approving each gate.
sourceRef: |
  PM-044 | Overview
  PM-044 | Solution 01 backend
  PM-044 | Solution 03 bot contract
lastVerified: 2026-11-07
---

## What It Does
<!-- chunkId: platform-harness-what-it-does -->
<!-- keywords: harness, planner, builder, reviewer, draft pull request -->

[live] The harness turns issues into draft pull requests: scouts fill a ranked queue, a person starts a task, and an orchestrator moves its run through plan, plan approval, build, test and draft pull request. Evidence: `internal/harness/domain/transitions.go:44`, `internal/harness/work.go:79`.

Each role is a coding CLI run headless in the run's own git worktree: the planner and plan checker, the builder and code checker, and the tester. Claude Code is the default; Codex and OpenCode run the same roles through the same port. The harness has no model runtime of its own.

## Who Decides
<!-- chunkId: platform-harness-who-decides -->
<!-- keywords: approval, gate, start, resume, webex -->

[live] Work starts only when a person presses Start, unless `autoStart` is on, and a plan reaches the builder only after a person approves it; a blocked run resumes only on a person's approval. Evidence: `internal/harness/queue.go:211`, `internal/harness/approvals.go:42`.

A gate can be decided on the Workstream board or from Webex through chat-bot; the decision records its channel and who made it, and the same idempotency key always returns the same decision. See [[platform-harness-bot-contract]].

## What It Will Not Do
<!-- chunkId: platform-harness-boundaries -->
<!-- keywords: never commit, baseline, transcript, budget, datadir -->

[live] Agents never commit, push or switch branches; the harness commits on `harness/*` branches only, after an approved plan, an approved build, and passing tests. Evidence: `internal/harness/domain/settings.go:149`, `internal/harness/adapters/worktree/gitworktree/gitworktree.go:70`.

[live] No prompt, brief or transcript is stored; an agent run keeps its usage, cost, bounded report and the path of the CLI's transcript. Evidence: `internal/harness/work.go:262`.

[live] The harness runs in Local mode on SQLite or Postgres storage only. Under `datadir` it reports itself unsupported, and in Cloud mode its changes answer 503. Evidence: `internal/harness/harness.go:161`, `internal/server/api/cloud_command.go:98`.

[open] Budgets are not enforced. Cost is recorded per agent run and summed per day, run, role and model; the only stop is each CLI's own spending limit per agent run.

Details: [[platform-harness-reference]].

## Where It Shows
<!-- chunkId: platform-harness-where-it-shows -->
<!-- keywords: workstream board, card, needs you, worked by, waves, queue, agents tab, settings -->

[live] The harness has no page of its own. A ticket is one card on the Workstream board whether a person or the harness works on it: the board reads the harness beside its items and joins a task to a card by the Jira key, which is the item's identifier. A card the harness holds shows who works on it, its stage and its progress, and rings amber when it waits on a person; what waits on a person sits in the "Needs you" strip above the board; a task the Finder found that no plan holds yet is a dashed card in Unsorted; "Worked by" is a facet. Evidence: `web/src/features/workstream/harnessJoin.ts:54`, `web/src/pages/WorkstreamPage.tsx:152`.

[live] The queue and the waves are views of the board, chosen with Status · Waves · Queue; the card's Agents tab holds the harness run beside the hand-driven sessions, and `?view=run` on the item page shows the run whole, full width. Run with the harness on a card creates its task through `POST /api/harness/tasks`. The harness's settings live under Settings. An old `/harness/runs/{id}` link opens the card that holds the run; every other `/harness` path opens the board. Evidence: `web/src/features/workstream/QueueView.tsx:16`, `web/src/features/workstream/AgentsPanel.tsx:78`, `web/src/app/router.ts:46`.
