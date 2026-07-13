# Backend Design: Complete Gin API Migration

## Overview

PM-031 converts the remaining API route families to Gin and removes the legacy `ServeMux` fallback after coverage reaches zero. The design keeps Gin at the transport boundary and preserves current domain services, repositories, local file storage, frontend contracts, and SPA serving.

## Current Transport State

| Area                | Current State                                        |
|---------------------|------------------------------------------------------|
| Gin router          | Owns `/api/health` and `/api/audit-events`.          |
| Legacy fallback     | Handles remaining API routes via Go `ServeMux`.      |
| Middleware          | Recovery, request ID propagation, timeout context.   |
| Response helpers    | `ginJSON`, `ginAppError`, `httpx` mapper.            |
| Boundary governance | Tests restrict Gin imports to `internal/server/api`. |
| Route inventory     | PM-030 inventory covers current `ServeMux` routes.   |

## Route Family Plan

| Family           | Example Routes                                       | Migration Requirements                                         |
|------------------|------------------------------------------------------|----------------------------------------------------------------|
| Navigation       | saved filters, recent items                          | Decode validation, not-found mapping, repository nil behavior. |
| System           | config paths, picker/open-path routes                | Native dialog error mapping, platform behavior preserved.      |
| State/Search/AI  | state, search, AI capabilities/settings              | Query defaults, unavailable behavior, settings persistence.    |
| Workspace reads  | list, runtime, health, tree, file reads, path search | Path guards, content limits, query params, cache interactions. |
| Item reads       | list, detail, files, content search, diff, Jira      | Item not-found mapping, file ID behavior, attachment guards.   |
| Workspace writes | create, import, update, delete, scan, runtime saves  | Registry/index side effects, scan result contracts, audit.     |
| Item writes      | save file, revert, metadata, status, create          | Stale hash, refresh, metadata rules, audit and recovery hints. |
| Knowledge        | wiki pages, graph, rescan, sync, enrich              | Long-running actions, not-found mapping, graph contract.       |
| Verification     | jobs, checkpoints, artifacts, rerun                  | Bounded policy, job status contracts, artifact path behavior.  |
| Git              | status, activity, branches, fetch/pull/push/commit   | Dirty-state guards, recovery hints, command result contracts.  |
| Streaming        | workspace stream-create, AI session channel          | Flush/upgrade behavior, cancel/disconnect/shutdown cleanup.    |

## Handler Pattern

| Concern      | Rule                                                                             |
|--------------|----------------------------------------------------------------------------------|
| Request data | Read params, query, headers, and body in Gin handlers only.                      |
| Services     | Call existing services with `context.Context`, models, and primitives.           |
| Errors       | Prefer typed application errors for new migrated code where behavior matches.    |
| Responses    | Use existing JSON shapes and preserve `error`, `code`, and `recoveryHint`.       |
| Side effects | Preserve scans, index refreshes, audit appends, Git operations, and file writes. |
| Cleanup      | Remove matching `mux.HandleFunc` registration only after parity tests pass.      |

## Parity Matrix

| Route Type       | Required Checks                                                                   |
|------------------|-----------------------------------------------------------------------------------|
| Read JSON        | status, content type, JSON shape, query defaults, not-found behavior.             |
| Write JSON       | decode errors, validation errors, success body, side effects, audit behavior.     |
| File response    | path guards, size limits, binary/text classification, nosniff where applicable.   |
| Git command      | dirty-state guard, recovery hints, command status, audit and refresh behavior.    |
| Verification job | queued/running/final status, artifacts, queue-full, cancellation, rerun behavior. |
| Streaming        | upgrade/flush, message shape, disconnect, cancel, shutdown cleanup.               |

## Cutover Criteria

| Criterion              | Required State                                                         |
|------------------------|------------------------------------------------------------------------|
| Route inventory        | No `/api/` routes remain registered only on legacy `ServeMux`.         |
| Boundary checks        | Gin imports exist only in `internal/server/api`.                       |
| Test coverage          | Each route family has focused parity or contract tests.                |
| Frontend compatibility | `rtk npm run typecheck` passes after final route family migration.     |
| Performance            | Scorecard records baseline and final results for representative reads. |
| Fallback removal       | `newTransport` no longer needs `NoRoute` legacy mux delegation.        |

## Design Decisions

| Decision                     | Rationale                                                                       |
|------------------------------|---------------------------------------------------------------------------------|
| Keep route family ownership  | Route families map to current code and reduce review risk.                      |
| Remove fallback gradually    | Prevents accidental missing routes while migration is still incomplete.         |
| Migrate streaming last       | Streaming and WebSocket behavior is harder to validate than normal JSON routes. |
| No frontend contract changes | Goal is transport migration, not product API redesign.                          |
| Keep SPA outside Gin         | Static asset serving is stable and independent from `/api/` routing.            |
