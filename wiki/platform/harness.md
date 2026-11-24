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
<!-- keywords: planner, builder, reviewer, draft pull request -->

[live] The harness turns issues into draft pull requests. Scouts fill a ranked queue. A person starts a task. An orchestrator moves its run through plan, plan approval, build, test, review and draft pull request. Evidence: `internal/harness/domain/transitions.go:112`, `internal/harness/work.go:67`.

Each role is a coding CLI run headless in the run's own git worktree: planner, plan checker, builder, code checker and tester. Claude Code is the default. Codex and OpenCode run the same roles through the same port. The harness has no model runtime of its own.

## A Repository's Own Procedure
<!-- chunkId: platform-harness-repository-procedure -->
<!-- keywords: role skills, track, skill file, brief -->

[live] A workspace can name the skill, and an optional track, that each role follows: `planner`, `plan_reviewer`, `coder`, `code_reviewer` or `qa_fixer`. Set it under Role skills on the workspace's AI and harness tab. Evidence: `internal/harness/build.go:223`, `internal/harness/application/brief/brief.go:104`.

- The brief tells the role to follow that skill and track, or to read `.claude/skills/<skill>/SKILL.md` where the CLI cannot load skills.
- The brief's scope, constraints, report and verification still win over the skill.
- A question the skill would ask the user goes into the report's `open` list.
- A role without a skill keeps the built-in instructions.
- Saving refuses a skill or track with characters outside the allowed set.

## Defaults And A Workspace's Own Choices
<!-- chunkId: platform-harness-workspace-settings -->
<!-- keywords: workspace override, per-workspace models, readiness check -->

[live] The harness switches under Settings are defaults. A workspace may override them, and it owns its models. Role pauses, slots, intervals and tool permissions are set once, under Settings. [[workspace-settings]] has the full split. Evidence: `internal/harness/domain/settings.go:141`, `internal/harness/dashboard_settings.go:60`.

[live] With the readiness check on, a Jira ticket in a ready status starts on its own. The Planner first checks that the ticket says enough to plan from. If not, the task waits for the ticket's reporter: the card reads "Waiting for <reporter>", and the panel holds the problem, the questions and a comment to copy onto the ticket. Kode Stream posts nothing to Jira. A reply on the ticket sends the Planner on. **Continue anyway** or **Dismiss** decide without the reporter. Evidence: `internal/harness/clarify.go:12`, `web/src/features/harness/GateCard.tsx:213`.

## Who Decides
<!-- chunkId: platform-harness-who-decides -->
<!-- keywords: approval gate, start, resume, hand off -->

[live] Work starts only on **Start**, unless `autoStart` is on. A plan reaches the builder only after a person approves it. A blocked run resumes only on a person's approval. Evidence: `internal/harness/work.go:113`, `internal/harness/approvals.go:40`.

[live] A run whose tests pass waits at Review while `requireReview` is on, the default. The person opens a session in the run's worktree from the Review step in Agent Space, tries the change and edits it. Evidence: `internal/harness/pipeline.go:276`.

- **Hand off** commits the person's changes and opens the draft pull request. It reruns the tests first if anything changed since they passed.
- **Back to Build** and **Back to Plan** send the person's words to the Builder or Planner as a must-fix finding. The run returns to Review once its tests pass.

A gate can be decided on the Workstream board or from Webex through the bot. The decision records its channel and actor. See [[platform-harness-bot-contract]].

## What It Will Not Do
<!-- chunkId: platform-harness-boundaries -->
<!-- keywords: never commit, transcript, budget, datadir -->

[live] Agents never commit, push or switch branches. The harness commits on `harness/*` branches only, after an approved plan, an approved build, passing tests and a Review decision. Evidence: `internal/harness/adapters/worktree/gitworktree/gitworktree.go:66`.

[live] No prompt, brief or transcript is stored. An agent run keeps its usage, cost, bounded report and the path of the CLI's transcript. Evidence: `internal/harness/domain/agent_run.go:51`.

[live] The harness runs in Local mode on SQLite or Postgres only. Under `datadir` it reports itself unsupported. In Cloud mode its changes return 503. Evidence: `internal/harness/harness.go:143`, `internal/server/api/cloud_auth.go:62`.

[open] Should the harness enforce budgets? Cost is recorded per agent run and summed per day, run, role and model. The only stop is each CLI's own spending limit per agent run.

## Where It Shows
<!-- chunkId: platform-harness-where-it-shows -->
<!-- keywords: workstream board, needs you, task queue, run page -->

[live] The harness has no page of its own. A ticket is one card on the Workstream board, joined by Jira key to the item's identifier. Evidence: `web/src/features/workstream/harnessJoin.ts:44`, `web/src/pages/WorkstreamPage.tsx:75`.

- A harness card shows who works on it, its stage and progress. It rings amber when it waits on a person, and pulses while an agent works.
- **Needs you**, above the board, lists what waits on a person.
- A task the Finder found that no plan holds is a dashed card in Unsorted. "Worked by" is a facet.
- A card opens with its kind and key: "Plan · key" with a solid stripe, or "Jira ticket · key" with a hatched stripe. The key opens the plan's page or the task's panel.

**Tasks · Plans** picks what the board shows, and each shows as a Board or a List. Evidence: `web/src/features/workstream/PlanListView.tsx:18`, `web/src/features/workstream/QueueView.tsx:16`.

- **Tasks** holds every task except dismissed and stale ones. The harness sets the column: To do before start, Draft until the plan is approved, In Progress while built and tested, Review while the pull request waits, Done when done. A stopped task stays where its run stopped.
- The only manual move is dragging an unstarted task from To do to Draft, which starts it. The card reads Starting until a Planner takes it.
- The Tasks list is the Task Queue: tasks in the order the harness takes them, each saying what it waits for. Active tasks show by default. Finished ones show under Finished or All.
- **Plans** holds the branch's plans, which only a person moves, by drag or by the status menu.
- **Run** with the harness on a plan card creates its task (`POST /api/harness/tasks`).
- **Configure AI & harness** beside To do opens the workspace's harness settings in a dialog.
- **Refresh** on Tasks polls the workspace's sources at once (`POST /api/harness/scout/poll`) and re-indexes. On Plans it only re-indexes.

Task actions:

- **Stop**, after a confirmation, stops a run. The task stays as Stopped, with **Start again** for a new run.
- **Reset**, after a confirmation, deletes every run with its stages, conversations, worktree and local `harness/` branch, and puts the task back in the queue. It keeps the pin, priority and questions setting (`POST /api/harness/tasks/{id}/reset`). Evidence: `web/src/features/harness/ResetTaskButton.tsx:17`.
- **Remove**, after a warning, stops any run and deletes the task with its runs, conversations, sessions, worktree and local branch (`POST /api/harness/tasks/{id}/remove`). If its issue still matches the Finder's search, the Finder adds it back. Evidence: `web/src/features/harness/RemoveTaskButton.tsx:12`.
- Reset and Remove keep a pushed branch and a pull request at the remote.

The plan a run writes is readable before the merge. **Read the plan** and a ticket card's Plan tab read the run's plan folder from its worktree, uncommitted work included, then from its branch.

`/harness/runs/{id}` opens the run full width, on the plan's item page or on its own page for a ticket without a plan. Evidence: `web/src/features/harness/RunWorkbench.tsx:20`, `web/src/app/router.ts:44`.

- Under the title: the ticket link, the branch and the pull request.
- Left panel: Plan (the plan folder), Changes (changed files with lines added and removed) and Agents (conversations grouped by step).
- Middle: the chosen plan file (Preview, Source or Diff), code diff or conversation.
- Right panel: Info, Jira with answered questions, Checker with every review, and Steps with status, rounds, time and cost.
- Every other `/harness` path opens the board.

Each harness agent round is a **harness session**. [[workspace-agent-sessions]] explains how Agent Space lists them beside interactive sessions.

Details: [[platform-harness-reference]].
