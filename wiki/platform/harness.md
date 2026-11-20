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

[live] The harness turns issues into draft pull requests: scouts fill a ranked queue, a person starts a task, and an orchestrator moves its run through plan, plan approval, build, test, review and draft pull request. Evidence: `internal/harness/domain/transitions.go:153`, `internal/harness/work.go:79`.

Each role is a coding CLI run headless in the run's own git worktree: the planner and plan checker, the builder and code checker, and the tester. Claude Code is the default; Codex and OpenCode run the same roles through the same port. The harness has no model runtime of its own.

## A Repository's Own Procedure
<!-- chunkId: platform-harness-repository-procedure -->
<!-- keywords: role skills, skill, track, feature-planning, workspace settings -->

[live] A workspace's settings can name the skill each role follows in its repository: a role (`planner`, `plan_reviewer`, `coder`, `code_reviewer`, `qa_fixer`) with a `skill` and an optional `track`, set under Role skills in the workspace's AI and harness tab under Workspaces. The role's brief then says to follow that skill and track, or to read `.claude/skills/<skill>/SKILL.md` where the CLI cannot load skills. The brief's scope, constraints, report and verification still win over the skill, and a question the skill would put to the user goes into the report's `open`. A role without a skill keeps the built-in instructions. A skill or track outside the allowed characters is refused when the settings are saved. A repository's `.kode-stream/harness.yaml`, which held role skills before, is moved into its workspace's settings once, on start. Evidence: `internal/harness/build.go:242`, `internal/harness/teamfile.go:109`, `internal/harness/application/brief/brief.go:211`.

## Defaults And A Workspace's Own Choices
<!-- chunkId: platform-harness-workspace-settings -->
<!-- keywords: defaults, workspace override, models per workspace, review per workspace, settings file -->

[live] The harness's switches under Settings are defaults every workspace starts from, and a workspace may set its own in its AI and harness tab: whether the harness runs there, whether its tasks start on their own, whether its runs wait at Review, whether an agent may stop and ask, its test command, the paths its Builder may change and its role skills; its sources sit in its Integrations tab, beside its Jira connection. Each workspace holds its own models, each role's engine, model, hard model and most turns, copied from Kode Stream's defaults when it is first seen on start; no setting decides them for every workspace. Role pauses, slots, intervals and every tool permission are set once, under Settings. A team shares a workspace's choices through `.kode-stream/settings.yaml`, which Kode Stream only exports into and imports from when a person asks. See [[workspace-settings]]. Evidence: `internal/harness/domain/settings.go:154`, `internal/harness/domain/settings.go:200`, `internal/harness/dashboard.go:604`.

## Who Decides
<!-- chunkId: platform-harness-who-decides -->
<!-- keywords: approval, gate, start, resume, webex -->

[live] Work starts only when a person presses Start, unless `autoStart` is on, and a plan reaches the builder only after a person approves it; a blocked run resumes only on a person's approval. Evidence: `internal/harness/queue.go:211`, `internal/harness/approvals.go:42`.

[live] A run whose tests passed waits at Review, unless the settings turn `requireReview` off; it is on by default. There the person opens a session of their own in the run's worktree, from the ticket's Review step in Agent Space, tries the change and edits it. Then they decide: **Hand off** commits what they changed and opens the draft pull request, after running the tests again if anything changed since they passed; **Back to Build** and **Back to Plan** send their words to the Builder or the Planner as a must-fix finding, and the run comes back to Review once its tests pass again. Evidence: `internal/harness/approvals.go:510`, `internal/harness/approvals.go:544`.

A gate can be decided on the Workstream board or from Webex through chat-bot; the decision records its channel and who made it, and the same idempotency key always returns the same decision. See [[platform-harness-bot-contract]].

## What It Will Not Do
<!-- chunkId: platform-harness-boundaries -->
<!-- keywords: never commit, baseline, transcript, budget, datadir -->

[live] Agents never commit, push or switch branches; the harness commits on `harness/*` branches only, after an approved plan, an approved build, passing tests, and a Review decision, which commits what the person changed in their session. Evidence: `internal/harness/domain/settings.go:149`, `internal/harness/adapters/worktree/gitworktree/gitworktree.go:70`.

[live] No prompt, brief or transcript is stored; an agent run keeps its usage, cost, bounded report and the path of the CLI's transcript. Evidence: `internal/harness/work.go:262`.

[live] The harness runs in Local mode on SQLite or Postgres storage only. Under `datadir` it reports itself unsupported, and in Cloud mode its changes answer 503. Evidence: `internal/harness/harness.go:161`, `internal/server/api/cloud_command.go:98`.

[open] Budgets are not enforced. Cost is recorded per agent run and summed per day, run, role and model; the only stop is each CLI's own spending limit per agent run.

Details: [[platform-harness-reference]].

## Where It Shows
<!-- chunkId: platform-harness-where-it-shows -->
<!-- keywords: workstream board, card, needs you, worked by, task queue, agents tab, settings -->

[live] The harness has no page of its own. A ticket is one card on the Workstream board whether a person or the harness works on it: the board reads the harness beside its items and joins a task to a card by the Jira key, which is the item's identifier. A card the harness holds shows who works on it, its stage and its progress, and rings amber when it waits on a person; what waits on a person sits in the "Needs you" strip above the board; a task the Finder found that no plan holds yet is a dashed card in Unsorted; "Worked by" is a facet. Evidence: `web/src/features/workstream/harnessJoin.ts:54`, `web/src/pages/WorkstreamPage.tsx:152`.

[live] The board shows Tasks or Plans, never both, chosen with Tasks · Plans at the left of the title row, Tasks first; each is shown as a Board or a List, chosen beside it, and each remembers its own. Tasks holds every task the harness has in the workspace except the dismissed and stale ones, on the plan card that holds it or on a card of its own, and the harness sets its column: To do before it starts, Draft from its start until its plan is approved, In Progress while it is built and tested, Review while its pull request waits, Done once it is done; a stopped task stays in the column its run reached, and a person never moves a task card. Start moves the card to Draft at once, where it reads Starting, with a turning loader and the working ring, until a Planner takes it; every run and gate event carries its task, so the card follows the run without a reload. Plans holds the plans on the branch, which only a person moves, by drag or by the status menu, as cards or as rows grouped by status, each row drawn as a Task Queue row: the ID with the service, the title with its source, owner and date, the harness's status or By hand, and the status menu. The Tasks list is the Task Queue: the tasks in the order the harness takes them, each saying what it waits for; the card's Agents tab holds the harness run beside the hand-driven sessions, and `?view=run` on the item page shows the run whole, full width. Run with the harness on a card creates its task through `POST /api/harness/tasks`. The plan a run writes is readable on the card at any time before the merge: "Read the plan" on the card's run, and the Plan tab of a ticket card, read the run's plan folder live from its own copy while it has one, uncommitted work included, then from its branch, local or as last pushed. Every card opens with its kind and key: a plan card reads "Plan · key" beside a solid stripe, and a card the harness holds that no plan on the shown branch matches reads "Jira ticket · key" beside a hatched stripe. The key opens the details: a card's key, a Plans row's ID and a Task Queue row's key open the plan's page or the task's panel, and the title is plain text, except where the ID only repeats the title, which then opens them. Stop, behind a confirmation, stops a run from its Task Queue row, its task panel or its full run page. A stopped task, whose run a person cancelled, is not finished: it stays on the board and in the Task Queue as Stopped, with Start again, which begins a new run beside the stopped one. The Task Queue shows the active tasks by default and the finished ones under Finished or All. Remove, behind a warning, deletes a task in any state from its Task Queue row, its task panel or its full run page: it stops a run in flight and deletes the task with its runs, conversations, sessions, working copy and local `harness/` branch, keeping a pushed branch and a pull request (`POST /api/harness/tasks/{id}/remove`). If its issue is still open and the Finder's search still matches it, the Finder adds it back as a new task. Reset, behind a confirmation, on the task panel and the full run page of any task that has had a run, starts the task over: it stops a run in flight, deletes every run with its stages, conversations, working copy and local `harness/` branch, and puts the task back in the Task Queue to wait for Start, keeping its pin, priority and questions setting; a pushed branch and a draft pull request stay at the remote (`POST /api/harness/tasks/{id}/reset`). Refresh on the Tasks board asks the workspace's sources at once (`POST /api/harness/scout/poll` with the workspace) and says what Jira returned, besides re-indexing the branch; on Plans it only re-indexes. The harness's settings live under Settings. `/harness/runs/{id}` opens the run whole, full width: on the item page of the plan card that holds it, or, for a Jira ticket no plan card holds, on a page of its own; the task panel's Open details goes there. Under the title it names the ticket as a link, the branch and, once opened, the pull request; under the pipeline it lays the run out as the item page lays out a plan. The left panel switches between Plan (the plan folder as a tree), Changes (every changed code file as a list, its name above its folder, with lines added and removed) and Agents (every conversation, grouped by step, where All steps or a step's name reads a whole section at once). The middle shows what is chosen in tabs: a plan file as Preview, Source or Diff, a code file as its Diff, a conversation as it ran. The right panel holds the task: Info, Jira with the questions a person answered, Checker with every review, whose findings open their file, and Steps with each step's status, rounds, time and cost. Clicking a step in the pipeline opens what that step made. Evidence: `web/src/features/harness/RunWorkbench.tsx:38`. Every other `/harness` path opens the board. A card an agent works on now wears a pulsing ring. Evidence: `web/src/features/workstream/harnessJoin.ts:179`, `web/src/features/workstream/PlanListView.tsx:17`, `web/src/features/harness/QueueList.tsx:84`, `web/src/features/workstream/QueueView.tsx:16`, `internal/harness/queue.go:281`, `web/src/features/harness/ResetTaskButton.tsx:23`, `web/src/features/harness/RemoveTaskButton.tsx:17`, `web/src/features/workstream/AgentsPanel.tsx:78`, `web/src/app/router.ts:46`.

[live] Each agent the harness runs, one Planner, Checker or Builder round, is a **harness session**: headless, in the run's own worktree, steered through the run's questions and approvals, read as a conversation. A **harness run** is the pipeline for one task and holds its harness sessions. A coding agent a person starts in a terminal on the checkout is an **interactive session**. Agent Space's Sessions view lists both kinds in one list, grouped by ticket, with an Interactive or Harness tag on each card and All · Interactive · Harness to narrow it. A ticket's harness sessions share one card there; its detail shows the run's stepper, where a step and then a round open that agent's conversation, beside the run's changes, Open run page and Stop. [[workspace-agent-sessions]] sets out the difference in full. Evidence: `web/src/features/sessions/HarnessSessionDetail.tsx:32`, `web/src/features/canvas/CanvasSessionsView.tsx:67`.
