# PM-042: Session Focus

## Overview

PM-042 makes the agent sessions you already run easier to see and act on, without inventing a new kind of session.
Three views share one list: a **Focus mode** on the Canvas that shows only plans with an active session, a
**Sessions tab** in the Workbench that lists the branch's sessions with their changes, and a **Sessions page** that
lists every session across every registered workspace. Each session gains a **Changes** view: the files the
checkout has changed since the session was launched, with a per-file diff.

This is the second shape of PM-042. The first, "Agent Lanes", modelled isolation as a separate lane entity with its
own node kind, Canvas section, and launch button. It was implemented on `feature/PM-042-agent-lanes` and set aside:
it duplicated the session concept, and its derived sections made the Canvas slow. An agent can simply be told to
work in its own worktree; what the tool owes the user is visibility of what each session is doing and a way to read
the result. That is what this plan delivers, on top of the existing session record, terminal manager, and Canvas.

## Glossary

| Term            | Meaning                                                                              | Maps To (code)                    |
|-----------------|--------------------------------------------------------------------------------------|-----------------------------------|
| Session         | One embedded or external AI or terminal process with a durable safe record.          | `ai.SessionRecord`                |
| Active session  | A session whose record is `starting` or `running` and whose process binding is live. | `ai.SessionRecordView.Live`       |
| Launch commit   | The checkout HEAD observed when the session started.                                 | `ai.SessionRecord.ObservedCommit` |
| Session changes | Files changed in the workspace since the launch commit: commits plus working tree.   | `ai.SessionChanges`               |
| Focus mode      | Canvas filter showing plans with an active session, their sessions, and edges only.  | `CanvasBoard` `focusActive` state |
| Sessions tab    | Workbench tab on the workspace node listing this branch's sessions.                  | `CanvasWorkbench` `sessions` tab  |
| Changes tab     | Workbench tab on a session node showing its session changes.                         | `CanvasWorkbench` `changes` tab   |
| Sessions page   | Left-nav page listing sessions across all registered workspaces.                     | `SessionsPage`                    |
| Session scope   | Query for the global list: `active` (default) or `all`.                              | `GET /api/sessions?scope=`        |

## Components

| Layer      | Component                                   | Purpose                                                                      |
|------------|---------------------------------------------|------------------------------------------------------------------------------|
| Domain     | `internal/git` since-commit readers         | Name-status, short stat, and per-file diff of the checkout against a commit. |
| Service    | `ai.Service.SessionChanges`                 | Resolves a session's changes from its workspace path and launch commit.      |
| Service    | `ai.Service.SessionRecordsAcrossWorkspaces` | Lists records for every workspace the caller may read, with live flags.      |
| Controller | `internal/server/api/sessions.go`           | `GET /api/sessions`, `GET /api/sessions/{id}/changes`, `.../diff`.           |
| Frontend   | `features/canvas` Focus mode                | Toolbar toggle, dimming, persisted per browser.                              |
| Frontend   | `features/canvas/CanvasWorkbench`           | Sessions tab on the workspace node; Changes tab on a session node.           |
| Frontend   | `features/sessions/SessionsPage`            | Cross-workspace list with stop, open, and changes actions.                   |
| Frontend   | `App.tsx`                                   | New `sessions` route and left-nav entry.                                     |

## Data Flow

```text
Canvas Focus mode
  projection.nodes -> plans with a session node in state starting|running and live -> visible set
  everything else dimmed (not hidden) so layout does not shift; edges to hidden nodes dropped

Workbench Sessions tab (workspace node selected)
  GET /api/ai/session-records?workspaceId&branch  (existing) -> table with state, provider, plan, started, changes count

Session node -> Changes tab
  GET /api/sessions/{id}/changes
    -> ai.Service.SessionChanges(record)
         git diff --name-status <launchCommit>        (committed + staged + unstaged, in the workspace path)
         git diff --shortstat  <launchCommit>
         untracked files from status
    -> { files[], additions, deletions, launchCommit, headCommit, headMoved }
  GET /api/sessions/{id}/diff?path -> unified diff of one file against the launch commit

Sessions page
  GET /api/sessions?scope=active|all
    -> for each registered workspace the caller may read: session records + live flag + workspace name + checkout branch
  Stop   -> DELETE /api/ai/sessions/{id}          (existing cancel)
  Open   -> navigate to Workbench for that workspace with the session node selected and expanded
  Changes-> GET /api/sessions/{id}/changes
```

## Design Decisions

| Decision                                                                   | Alternatives Considered                | Rationale                                                                                                    |
|----------------------------------------------------------------------------|----------------------------------------|--------------------------------------------------------------------------------------------------------------|
| No new entity; everything hangs off the session record                     | Keep PM-042 lanes and add views on top | One concept the user already understands. Isolation is the agent's choice, not a tool-managed object.        |
| Changes are computed against the launch commit                             | Store a diff per session               | The record already holds `observedCommit`; Git computes the rest on demand and nothing new is persisted.     |
| Changes include the working tree, not only commits                         | Commits only                           | Agents often leave uncommitted work; the user wants to see it before deciding anything.                      |
| Focus mode dims instead of hiding                                          | Filter nodes out                       | Hiding reflows the board and loses spatial memory; dimming keeps positions and is cheap to render.           |
| Focus mode is a toggle, persisted per browser                              | A saved view                           | It is a momentary lens, like the existing node-type facet, not a layout.                                     |
| Global list scope defaults to `active`                                     | Always all                             | The page exists to answer "what is running right now"; history is one click away.                            |
| Sessions page reuses the workspace authorization used by the AI controller | A separate policy                      | Cloud mode must not leak another tenant's session list or diffs; the existing check is the only correct one. |
| Sessions page is a new left-nav page                                       | Another Workbench tab                  | Workbench is workspace-scoped; a cross-workspace list has no home there.                                     |
| Stop reuses the existing cancel endpoint                                   | New bulk stop                          | One process owner, one cancel path, as PM-020 and PM-037 require.                                            |

## Non-Goals

- No worktree, branch, adopt, compare, or discard management. If a user wants isolation, they ask the agent for it.
- No transcript, prompt, or output storage. Changes are derived from Git at request time.
- No new process owner. The terminal manager keeps the PTY, grants, and cancellation.
- No Cloud Agent execution. The list and changes work for Local checkouts; other content providers report
  `unsupported` for changes.

## Related Plans

| Ticket                        | Relationship             | Key Context                                                                         |
|-------------------------------|--------------------------|-------------------------------------------------------------------------------------|
| [PM-037](../PM-037/README.md) | Canvas and session nodes | Reuse session records, live flags, node selection, and the Workbench shell.         |
| [PM-020](../PM-020/README.md) | Embedded terminal        | Cancel and reattach stay with the terminal manager.                                 |
| [PM-038](../PM-038/README.md) | Checkout-first           | Changes are computed in the checkout; the branch is never switched.                 |
| Agent Lanes (unmerged branch) | Earlier shape            | `feature/PM-042-agent-lanes` holds the lane design; this plan does not build on it. |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](solution/solution-01-backend.md)
- [Frontend Design](solution/solution-02-frontend.md)
- [UI Automation](automation/README.md)
- [Implementation Plan](implementation-plan.md)
