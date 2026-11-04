# Scenarios: PM-044 Engineering Harness

## Scenario List

| #  | Title                          | Expected Result                                                                       |
|----|--------------------------------|---------------------------------------------------------------------------------------|
| 1  | Fill the queue from a source   | A poll finds open issues, dedupes them by source key, ranks them, and lists them      |
| 2  | Pin, re-prioritise and dismiss | The queue order changes at once and an event reaches every open dashboard             |
| 3  | Start a task                   | A person presses Start; a run, a worktree and its plan stage appear                   |
| 4  | Approve a plan                 | The plan round passes review, the plan gate waits, approval starts the build stage    |
| 5  | Ask for changes                | A rejection with a reason blocks the run; a resume approval sends it back to work     |
| 6  | Build and check                | The coder works, the harness re-runs the gate, the reviewer approves, harness commits |
| 7  | Test and fix                   | Verification runs twice; a failure starts a fix round with the log tail               |
| 8  | Open a draft pull request      | The harness pushes `harness/*` and opens a draft PR; the task is done                 |
| 9  | Cancel a run                   | The CLI's process group stops, the stage is cancelled, the worktree is kept           |
| 10 | Restart mid-run                | Running agent runs become interrupted and are re-queued once                          |
| 11 | Approve from Webex             | The bot decides with an idempotency key; the dashboard shows "via Webex by X"         |
| 12 | Read the cost                  | Cost per day, run, role and model sums to the run ledger's total                      |

## Flow 1: Task Through The Pipeline

```text
Scout polls a source (Jira JQL, GitHub labels, Bitbucket issues)
  -> task discovered (dedupe on source kind + key, never downgraded)
  -> sorter ranks (rules, optional bounded re-ranking)
  -> person presses Start (start approval, channel dashboard)
  -> dispatcher: fetch, worktree add on harness/<key>-<short>, run + plan stage
  -> plan stage: planner -> plan reviewer, up to 3 rounds
  -> plan approval gate (dashboard or Webex)
  -> implement stage: coder -> harness gate -> code reviewer -> harness commit, up to 3 rounds
  -> QA stage: verification x2, qa fixer on failure, up to 2 fix rounds
  -> optional PR approval gate
  -> draft PR stage: push, draft pull request, task done, worktree removed
```

## Flow 2: Approval From Webex

```text
Harness appends approval.requested
  -> outbox delivers a signed webhook to chat-bot (at least once)
  -> person answers /harness approve <runId> in Webex
  -> bot draft -> confirm card -> POST /api/harness/bot/approvals/{id}/decide with idempotencyKey
  -> harness records channel webex, appends approval.decided
  -> dashboard event stream shows "Approved via Webex by <name>"
```

## Out Of Scope

- Budgets that pause new work, a budget gate and a spend meter. Cost is recorded and summed only.
- Jira comments and transitions, GitHub labels and comments, merging a pull request.
- The chat-bot change itself.
