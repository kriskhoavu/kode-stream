# Session Focus

Session Focus makes the sessions a user already runs easier to see and act on: a Canvas filter that surfaces what is
active right now, a per-session view of what the checkout has changed, and a page that lists sessions across every
registered workspace. It adds no new kind of session, node, or launch path — everything hangs off the existing
`ai.SessionRecord` and the terminal manager that already owns its process.

## Session Changes

Session changes are what the checkout has done since a session's **launch commit** — the HEAD `ai.SessionRecord`
observed when the session started (`ObservedCommit`). They are computed from Git at request time and are never
persisted:

- **Committed plus working tree.** The comparison covers commits made since the launch commit *and* the working
  tree — staged, unstaged, and untracked files. Agents often leave uncommitted work, and the user needs to see it
  before deciding anything, so the view is not commits-only.
- **Shared-checkout caveat.** The comparison is against the checkout, not against one session's private clone.
  Because PM-042 does not introduce worktree isolation, changes since launch include work by any session — or any
  person — in that same checkout, not only the one session being viewed.
- **What they are not.** Session changes carry no transcript, prompt, or output. There is no new store: the durable
  session record still holds only workspace/plan references, provider, intent, branch, launch commit, lifecycle
  timestamps, state, and exit code — never prompts, arguments, environment, grants, buffers, or credentials. A file
  diff is fetched on demand and discarded once rendered.

## Availability Rules And Reason Codes

Computing changes requires a resolved workspace with a Local checkout at the launch commit. `ai.Service.SessionChanges`
and `SessionFileDiff` run the same guard chain and report one of three `availability` values:

| Availability  | Meaning                                                           |
|---------------|-------------------------------------------------------------------|
| `available`   | Every field was computed from Git.                                |
| `unavailable` | The workspace or launch commit could not support the computation. |
| `unsupported` | The workspace's content provider cannot compute changes at all.   |

When availability is not `available`, a `reason` explains why:

| Reason                         | Meaning                                                                                                                      |
|--------------------------------|------------------------------------------------------------------------------------------------------------------------------|
| `content_provider_unsupported` | The workspace is not a Local checkout (for example, Cloud Agent-Backed with a non-local axis, or Agentless Remote Snapshot). |
| `workspace_missing`            | The workspace is not registered, or its checkout path no longer exists on disk.                                              |
| `launch_commit_unknown`        | The session record has no `ObservedCommit` to compare against.                                                               |
| `launch_commit_missing`        | The launch commit no longer exists in the checkout's history (for example, a rebase or a squash rewrote it).                 |

An infrastructure failure — a corrupt repository, a missing Git binary, or any error other than "the commit does not
exist" — is never reported as `launch_commit_missing`; it surfaces as a request failure instead, so a real outage is
never mistaken for a routine unavailability.

## The Global List's Authorization Rule

The Sessions page lists sessions across every registered workspace, so it reuses the same workspace authorization
the AI controller already applies to every other session endpoint (`authorizeWorkspace`), rather than a separate
policy. A workspace the caller may not read is simply absent from the list — never a reported error — which keeps a
denial indistinguishable from "no sessions here" and avoids turning the list into an oracle for workspace existence.
The same rule extends to a session ID: `authorizeSession` resolves a session's owning workspace first and applies the
identical check, so a session that does not exist gets the same 404 a denied workspace would.

This matters most in Cloud mode, where the global list must never leak another tenant's session names, workspace
names, or changes across a workspace boundary the caller cannot otherwise see.

## Focus Mode's Dim-Not-Hide Rule

Focus mode is a Canvas toolbar toggle, persisted per browser, that narrows attention to plans with an active session
— one whose record is `starting` or `running` and whose process binding is live — their session nodes, and the edges
between them. Everything else on the board is **dimmed, not hidden**: nodes keep their saved positions and the layout
never reflows, because hiding a node would shift everything else and lose the user's spatial memory of the board.
Dimmed nodes are also removed from keyboard tab order and are not clickable while dimmed, so focus is not just visual
— an edge connecting a dimmed node to another dimmed node is dropped from the rendered set entirely, matching the
"active plans, their sessions, and edges only" rule. Turning Focus on with nothing active shows a "Focus · nothing
running" label and a one-click way to turn it back off.

## Non-Goals

- No worktree, branch, adopt, compare, or discard management; if a user wants isolation, they ask the agent for it in
  its own worktree.
- No transcript, prompt, or output storage — changes are derived from Git at request time.
- No new process owner — the terminal manager keeps the PTY, grants, and cancellation.
- No Cloud Agent execution for changes — the list and changes work for Local checkouts; other content providers
  report `unsupported`.
