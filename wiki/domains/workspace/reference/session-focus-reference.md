---
slug: workspace-session-focus-reference
title: Session Focus Reference
pageType: REFERENCE
topics: sessions, api, changes, authorization, polling
summary: Reference for global and Workbench session listing, change availability, file diffs, and access controls.
sourceRef: |
  PM-042 (plan retired) | Session APIs and Git computation
  PM-042 (plan retired) | Client state
  PM-042 (plan retired) | Browser journey
  PM-042 (plan retired) | Visible actions and assertions
  PM-042 (plan retired) | Historical result
lastVerified: 2026-09-12
---

## Global Session List
<!-- chunkId: workspace-session-focus-reference-global-list -->
<!-- keywords: global sessions, active scope, authorization, row cap -->

[live] `GET /api/sessions` defaults to active scope, accepts `active` or `all`, omits workspaces the caller cannot read, sorts newest first, and returns at most 200 rows. Evidence: `internal/server/api/sessions.go:43`, `internal/ai/session_global.go:14`.

Only active rows receive best-effort change counts, and only after sorting and truncation. Missing titles, branches, or change counts do not fail the full list.

## Change Availability
<!-- chunkId: workspace-session-focus-reference-change-availability -->
<!-- keywords: available, unavailable, unsupported, launch commit -->

[live] Changes are `unsupported` for non-Local content providers and `unavailable` for a missing workspace path, unknown launch commit, or launch commit removed from history. Evidence: `internal/ai/session_changes.go:93`, `internal/ai/session_changes.go:126`.

Other Git failures remain server errors so corruption or missing infrastructure is not mislabeled as a routine rewritten-history case. The summary reports file status, rename origin, additions, deletions, HEAD movement, and truncation.

## Diff And Authorization
<!-- chunkId: workspace-session-focus-reference-diff-authorization -->
<!-- keywords: session diff, one mibibyte, path guard, hidden denial -->

[live] `GET /api/sessions/{id}/diff` returns a single on-demand file diff capped at 1 MiB, and session authorization returns the same 404 for a missing session and a denied workspace. Evidence: `internal/server/api/sessions.go:12`, `internal/server/api/sessions.go:89`.

An unavailable or unsupported diff returns conflict with its reason. Empty paths and paths beginning with `:` or `-` are rejected before Git is invoked.

## Session Clients
<!-- chunkId: workspace-session-focus-reference-client-refresh -->
<!-- keywords: global route, checkout filter, plan grouping, projection refresh -->

[live] The top-level Sessions route remains in primary navigation and renders records grouped by workspace and plan. Evidence: `web/src/App.tsx:127`, `web/src/App.tsx:298`, `web/src/features/sessions/SessionsPage.tsx:44`.

[live] The workspace-node Sessions tab loads records for the current workspace and checkout and groups them by plan. Stopping a session reloads the list and Canvas projection; **Open on board** selects the existing Canvas node. Evidence: `web/src/features/canvas/CanvasWorkbench.tsx:193`, `web/src/features/canvas/CanvasWorkbench.tsx:300`, `web/src/features/canvas/CanvasWorkbench.tsx:319`, `web/src/pages/CanvasPage.tsx:75`.

## Not supported
<!-- chunkId: workspace-session-focus-reference-not-supported -->
<!-- keywords: working tree fallback, durable diff, cloud provider, unlimited list -->

[live] A missing launch commit has no working-tree-only fallback, diffs are not persisted, non-Local providers cannot compute changes, and the global list is not unbounded. Evidence: `internal/ai/session_changes.go:133`, `internal/ai/session_global.go:90`.

## Known conflicts
<!-- chunkId: workspace-session-focus-reference-known-conflicts -->
<!-- keywords: historical validation, two workspace, native keyboard -->

[open] Have both the two-workspace global journey and checkout-scoped Workbench tab, with deterministic real edits and native terminal keyboard interaction, been rerun? The retired result used one throwaway workspace and did not complete that coverage.

[live] The global Sessions client and checkout-scoped Workbench tab coexist. Treating either one as the replacement for the other contradicts the committed navigation and Canvas implementation. Evidence: `web/src/App.tsx:127`, `web/src/features/canvas/CanvasWorkbench.tsx:190`, `web/src/features/canvas/CanvasWorkbench.tsx:193`.

## Related pages

See [[workspace-session-focus]], [[workspace-terminal-canvas-reference]], and [[e2e-testing-workspace-terminal-canvas]].
