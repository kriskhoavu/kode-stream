---
slug: platform-terminal-canvas-reference
title: Terminal Canvas Reference
pageType: REFERENCE
roles: DEVELOPER, TESTER, BA
topics: canvas, api, storage, sessions, verification
summary: Reference for Canvas ownership, branch-aware APIs, safe session records, and verification freshness.
sourceRef: plans/platform/PM-037/design/design-01-backend.md
sourceRef: plans/platform/PM-037/design/design-02-frontend.md
sourceRef: plans/platform/PM-038/design/design-01-backend.md
sourceCount: 2
lastTicket: PM-038
---

## Ownership
<!-- chunkId: platform-terminal-canvas-reference-ownership -->
<!-- keywords: ownership, placements, entities, git, process -->

| Data                                                             | Authority                 | Canvas storage  |
|------------------------------------------------------------------|---------------------------|-----------------|
| Position, collapsed state, viewport                              | Canvas repository         | Yes             |
| Plan content and repository relationships                        | Item index and repository | Reference only  |
| Branch, HEAD, dirty state, changed files                         | Git                       | Never           |
| Verification result and fingerprint                              | Verification service      | Projection only |
| Safe lifecycle metadata                                          | Session-record repository | Reference only  |
| PTY, grants, prompts, arguments, environment, bytes, credentials | Process manager           | Never           |

## APIs And Conflicts
<!-- chunkId: platform-terminal-canvas-reference-api -->
<!-- keywords: api, resolve, placements, viewport, conflict -->

`POST /api/canvas/default` resolves the current checkout through the Workstream loader before it creates or projects the
default workspace/branch layout. A legacy request branch is accepted only when it matches the checkout; otherwise it
returns `canvas_branch_mismatch`. `GET /api/canvas/layouts/{layoutId}` refreshes the projection. Placement and viewport
PATCH operations use independent optimistic revisions; a placement conflict affects only reported nodes. References
cannot cross workspaces or silently rebind stale plans. See [[platform-branch-context-reference]].

## Session Safety
<!-- chunkId: platform-terminal-canvas-reference-session -->
<!-- keywords: session, metadata, process, branch, idempotency -->

A launch includes expected workspace, branch, observed commit, and idempotency key. Repeated requests with one key start
at most one process. Durable records retain bounded identity and lifecycle fields only. Live bindings, channel grants,
and xterm instances remain in memory; startup reconciliation marks records without bindings interrupted. Selection and
terminal disclosure are independent: a top-right action persists the placement's collapsed state, while terminal input,
selection, and scrolling are excluded from node drag and Canvas pan handling. Removed placements remain hidden and are
not returned as new-node candidates. See
[[platform-terminal-canvas-workflows]].

## Verification Freshness
<!-- chunkId: platform-terminal-canvas-reference-verification -->
<!-- keywords: verification, fingerprint, fresh, stale, inconclusive -->

The fingerprint covers branch, HEAD, index, relevant tracked and untracked worktree content, and verification
configuration. `fresh` means start, finish, and current fingerprints match; `stale` means current inputs changed;
`inconclusive` means inputs changed during execution or fingerprinting failed. Verification jobs are in-memory in PM-037.

## Limits
<!-- chunkId: platform-terminal-canvas-reference-limits -->
<!-- keywords: limits, placements, coordinates, sessions, deferred -->

The MVP bounds a layout to 300 placements, a patch to 50 placements, coordinate magnitude to 1,000,000, viewport zoom
to 0.1–2.0, and safe records to 500 per workspace. Postgres Canvas persistence, Cloud Agent execution, snapshot-backed
Canvas UX, groups, custom edges, and durable verification history are deferred. Local non-checkout review is provided
by [[platform-branch-review-workflows]], not by Canvas.
