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

## A Repository's Own Procedure
<!-- chunkId: platform-harness-repository-procedure -->
<!-- keywords: harness.yaml, skill, track, feature-planning -->

[live] A repository can name the skill each role follows in `.kode-stream/harness.yaml`: `version: 1` and, under `roles`, a role (`planner`, `plan_reviewer`, `coder`, `code_reviewer`, `qa_fixer`) with a `skill` and an optional `track`. The harness reads the file from the run's worktree for every agent run, and the role's brief then says to follow that skill and track, or to read `.claude/skills/<skill>/SKILL.md` where the CLI cannot load skills. The brief's scope, constraints, report and verification still win over the skill, and a question the skill would put to the user goes into the report's `open`. A role the file does not map keeps the built-in instructions; so does every role when the file is missing, malformed, at another version, or names an unknown role, or a skill or track outside the allowed characters. Agents cannot change the file, because `.kode-stream/` is outside every scope. Evidence: `internal/harness/application/procedure/procedure.go:55`, `internal/harness/build.go:243`, `internal/harness/application/brief/brief.go:211`.

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
<!-- keywords: workstream board, card, needs you, worked by, task queue, agents tab, settings -->

[live] The harness has no page of its own. A ticket is one card on the Workstream board whether a person or the harness works on it: the board reads the harness beside its items and joins a task to a card by the Jira key, which is the item's identifier. A card the harness holds shows who works on it, its stage and its progress, and rings amber when it waits on a person; what waits on a person sits in the "Needs you" strip above the board; a task the Finder found that no plan holds yet is a dashed card in Unsorted; "Worked by" is a facet. Evidence: `web/src/features/workstream/harnessJoin.ts:54`, `web/src/pages/WorkstreamPage.tsx:152`.

[live] The board shows Tasks or Plans, never both, chosen with Tasks · Plans at the left of the title row, Tasks first; each is shown as a Board or a List, chosen beside it, and each remembers its own. Tasks holds every task the harness has in the workspace except the dismissed and stale ones, on the plan card that holds it or on a card of its own, and the harness sets its column: To do before it starts, Draft from its start until its plan is approved, In Progress while it is built and tested, Review while its pull request waits, Done once it is done; a stopped task stays in the column its run reached, and a person never moves a task card. Start moves the card to Draft at once, where it reads Starting, with a turning loader and the working ring, until a Planner takes it; every run and gate event carries its task, so the card follows the run without a reload. Plans holds the plans on the branch, which only a person moves, by drag or by the status menu, as cards or as rows grouped by status. The Tasks list is the Task Queue: the tasks in the order the harness takes them, each saying what it waits for; the card's Agents tab holds the harness run beside the hand-driven sessions, and `?view=run` on the item page shows the run whole, full width. Run with the harness on a card creates its task through `POST /api/harness/tasks`. The plan a run writes is readable on the card at any time before the merge: "Read the plan" on the card's run, and the Plan tab of a ticket card, read the run's plan folder live from its own copy while it has one, uncommitted work included, then from its branch, local or as last pushed. Every card opens with its kind and key: a plan card reads "Plan · key" beside a solid stripe, and a card the harness holds that no plan on the shown branch matches reads "Jira ticket · key" beside a hatched stripe. Stop, behind a confirmation, stops a run from its Task Queue row, its task panel or its full run page. A stopped task, whose run a person cancelled, is not finished: it stays on the board and in the Task Queue as Stopped, with Start again, which begins a new run beside the stopped one. The Task Queue shows the active tasks by default and the finished ones under Finished or All, where Remove deletes one with its runs and their history after a warning, keeping its branch and pull request. If its issue is still open and the Finder's search still matches it, the Finder adds it back as a new task. Reset, behind a confirmation, on the task panel and the full run page of any task that has had a run, starts the task over: it stops a run in flight, deletes every run with its stages, conversations, working copy and local `harness/` branch, and puts the task back in the Task Queue to wait for Start, keeping its pin, priority and questions setting; a pushed branch and a draft pull request stay at the remote (`POST /api/harness/tasks/{id}/reset`). Refresh on the Tasks board asks the workspace's sources at once (`POST /api/harness/scout/poll` with the workspace) and says what Jira returned, besides re-indexing the branch; on Plans it only re-indexes. The harness's settings live under Settings. `/harness/runs/{id}` opens the run whole, full width: on the item page of the plan card that holds it, or, for a Jira ticket no plan card holds, on a page of its own; the task panel's Open details goes there. Every other `/harness` path opens the board. A card an agent works on now wears a pulsing ring. Evidence: `web/src/features/workstream/harnessJoin.ts:179`, `web/src/features/workstream/PlanListView.tsx:17`, `web/src/features/workstream/QueueView.tsx:16`, `internal/harness/queue.go:281`, `web/src/features/harness/ResetTaskButton.tsx:23`, `web/src/features/workstream/AgentsPanel.tsx:78`, `web/src/app/router.ts:46`.
