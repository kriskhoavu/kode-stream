---
slug: workspace-terminal-canvas-reference
title: Terminal Canvas Reference
pageType: REFERENCE
topics: canvas, api, storage, sessions, verification
summary: Reference for Canvas ownership, branch-aware APIs, safe session records, and verification freshness.
sourceRef: |
  PM-037 (plan retired) | Backend contracts
  PM-037 (plan retired) | Frontend contracts
  PM-038 (plan retired) | Checkout contracts
lastVerified: 2026-11-24
---

## Ownership
<!-- chunkId: workspace-terminal-canvas-reference-ownership -->
<!-- keywords: placements, authority, projection, process manager -->

| Data                                                             | Authority                 | Canvas storage  |
|------------------------------------------------------------------|---------------------------|-----------------|
| Position, collapsed state, viewport                              | Canvas repository         | Yes             |
| Plan content and repository relationships                        | Item index and repository | Reference only  |
| Branch, HEAD, dirty state, changed files                         | Git                       | Never           |
| Verification result and fingerprint                              | Verification service      | Projection only |
| Safe lifecycle metadata                                          | Session-record repository | Reference only  |
| PTY, grants, prompts, arguments, environment, bytes, credentials | Process manager           | Never           |

## APIs And Conflicts
<!-- chunkId: workspace-terminal-canvas-reference-api -->
<!-- keywords: resolve, layouts, viewport, optimistic revision -->

[open] Do the endpoint rules below still hold? They come from the retired plan.

- `POST /api/canvas/default` resolves the checkout through the Workstream loader, then creates or projects the layout. A request branch that differs from the checkout returns `canvas_branch_mismatch`.
- `GET /api/canvas/layouts/{layoutId}` refreshes the projection.
- Placement and viewport PATCH calls use separate optimistic revisions. A placement conflict affects only the reported nodes.
- References cannot cross workspaces or silently rebind a stale plan.

## Session Safety
<!-- chunkId: workspace-terminal-canvas-reference-session -->
<!-- keywords: idempotency key, branch mismatch, interrupted, binding -->

[live] A launch carries workspace, plan, expected branch, observed commit and one idempotency key. Just before start, the server rereads the checkout and returns `terminal_branch_mismatch` on a mismatch. One key starts at most one process. Evidence: `internal/ai/session_record_service.go:247`, `internal/server/api/ai_controller.go:86`.

- A record holds workspace and plan references, provider, bounded intent, branch, commit, timestamps, state and exit code. It never holds prompts, arguments, environment, grants, input, output, credentials or buffers.
- Live bindings, channel grants and xterm instances stay in memory. Startup marks a record without a binding `interrupted`.
- Terminal input, selection and scroll never start a node drag or Canvas pan.
- New entities are placed silently on load or refresh. Removed placements stay hidden.

## Verification Freshness
<!-- chunkId: workspace-terminal-canvas-reference-verification -->
<!-- keywords: fingerprint, fresh, stale, inconclusive, in-memory jobs -->

Each run takes a fingerprint at start and at finish. It covers branch, HEAD, index, relevant tracked and untracked content, and the verification configuration.

| State          | Meaning                                                      |
|----------------|--------------------------------------------------------------|
| `fresh`        | Start, finish and current fingerprints match                 |
| `stale`        | Inputs changed after the run finished                        |
| `inconclusive` | Inputs changed during the run, or fingerprinting failed      |

Canvas recomputes the current fingerprint on explicit refresh, after verification polling, on window focus and on document visibility. The Workbench shows short verified and current revisions.

[live] Verification jobs live in memory. A restart loses their history. Evidence: `internal/verification/job_store.go:23`.

## Limits
<!-- chunkId: workspace-terminal-canvas-reference-limits -->
<!-- keywords: placements, coordinates, zoom, batch -->

[live] A layout holds at most 300 placements. A patch holds at most 50. Coordinates stay within ±1,000,000 and zoom within 0.1–2.0. Evidence: `internal/canvas/repository.go:15`.

## Not supported
<!-- chunkId: workspace-terminal-canvas-reference-not-supported -->
<!-- keywords: cloud agent, snapshot canvas, custom edges, verification history -->

Cloud Agent execution, a snapshot-backed Canvas, server-stored sections, custom edges and durable verification history. Non-checkout review is [[workspace-branch-review-workflows]], not Canvas.

## Known conflicts
<!-- chunkId: workspace-terminal-canvas-reference-known-conflicts -->
<!-- keywords: postgres canvas, deferred persistence, record cap -->

- "Canvas has no Postgres persistence." It has. SQLite and Postgres both use the SQL Canvas and session-record repositories. Evidence: `internal/storage/storage.go:249`, `internal/storage/storage.go:271`.
- "Safe records are capped at 500 per workspace." No such cap exists in the code.

## Related pages

See [[workspace-terminal-canvas]], [[platform-storage-reference]] and [[workspace-branch-context-reference]].
