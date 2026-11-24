---
slug: workspace-session-focus-reference
title: Session Focus Reference
pageType: REFERENCE
topics: sessions, api, changes, authorization, polling
summary: Reference for Agent Space's session listing, change availability, file diffs, and access controls.
sourceRef: |
  PM-042 (plan retired) | Session APIs and Git computation
  PM-042 (plan retired) | Client state
  PM-042 (plan retired) | Browser journey
  PM-042 (plan retired) | Visible actions and assertions
  PM-042 (plan retired) | Historical result
lastVerified: 2026-11-24
---

## Global Session List
<!-- chunkId: workspace-session-focus-reference-global-list -->
<!-- keywords: active scope, authorization, row cap, change counts -->

[live] `GET /api/sessions` defaults to `active` scope and accepts `active` or `all`. It omits workspaces the caller cannot read, sorts newest first and returns at most 200 rows. Evidence: `internal/server/api/sessions.go:34`, `internal/ai/session_global.go:13`.

Only active rows get best-effort change counts, after sorting and truncation. A missing title, branch or count does not fail the list.

## Change Availability
<!-- chunkId: workspace-session-focus-reference-change-availability -->
<!-- keywords: launch commit, reason code, unsupported, unavailable -->

[live] Changes compare the session's launch commit (`ObservedCommit`) with the checkout, including staged, unstaged and untracked files. Each response carries an `availability` and, when not `available`, a `reason`. Evidence: `internal/ai/session_changes.go:15`.

| Availability  | Reason                         | Meaning                                                    |
|---------------|--------------------------------|------------------------------------------------------------|
| `unsupported` | `content_provider_unsupported` | The workspace is not a Local checkout                      |
| `unavailable` | `workspace_missing`            | The workspace is not registered, or its path is gone       |
| `unavailable` | `launch_commit_unknown`        | The record has no `ObservedCommit`                         |
| `unavailable` | `launch_commit_missing`        | The launch commit left history, for example after a rebase |

Any other Git failure is a server error, never `launch_commit_missing`. The summary reports file status, rename origin, additions, deletions, HEAD movement and truncation.

## Diff And Authorization
<!-- chunkId: workspace-session-focus-reference-diff-authorization -->
<!-- keywords: one mebibyte, path guard, hidden denial, 404 -->

[live] `GET /api/sessions/{id}/diff` returns one file diff on demand, capped at 1 MiB. A missing session and a denied workspace both return the same 404. Evidence: `internal/server/api/sessions.go:12`, `internal/server/api/sessions.go:80`.

An unavailable or unsupported diff returns 409 with its reason. Git never runs for an empty path or a path that starts with `:` or `-`.

## Session Clients
<!-- chunkId: workspace-session-focus-reference-client-refresh -->
<!-- keywords: global route, checkout filter, plan grouping, projection refresh -->

[live] Agent Space's Sessions view is the one session list, and `/sessions` opens it. It loads the current workspace's records for the checked-out branch, groups them by plan and lists harness sessions beside them (see [[workspace-agent-sessions-reference]]). Stopping an interactive session reloads the list and the Canvas projection. **Open on board** selects the existing Canvas node. Evidence: `web/src/app/router.ts:62`, `web/src/features/canvas/CanvasSessionsView.tsx:11`.

## Not supported
<!-- chunkId: workspace-session-focus-reference-not-supported -->
<!-- keywords: working tree fallback, durable diff, cloud provider, unlimited list -->

[live] A missing launch commit has no working-tree-only fallback. Diffs are never stored. Non-Local providers cannot compute changes. The global list is capped. Evidence: `internal/ai/session_changes.go:105`, `internal/ai/session_global.go:13`.

## Known conflicts
<!-- chunkId: workspace-session-focus-reference-known-conflicts -->
<!-- keywords: attribution, shared checkout, authorship -->

- "Changes show what this one session did." They show everything in the checkout since launch, by any session or person.

## Related pages

See [[workspace-session-focus]], [[workspace-agent-sessions-reference]] and [[workspace-terminal-canvas-reference]].
