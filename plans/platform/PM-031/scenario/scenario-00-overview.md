# Scenarios: PM-031 Overview

## Goal

Migrate all remaining API routes to Gin without changing the browser-visible API contract or local workspace side effects. This scenario set is complete.

## Scenario List

| #   | Title                        | Description                                                                    |
|-----|------------------------------|--------------------------------------------------------------------------------|
| 0   | Current PM-031 state         | Gin owns every `/api/` route and the legacy fallback is removed.               |
| 1   | Read route family migration  | Read-only JSON route families move to Gin with parity tests.                   |
| 2   | Write route family migration | Mutating routes move after audit, scan, refresh, and error behavior is locked. |
| 3   | Streaming route migration    | WebSocket and stream routes move after normal HTTP routes are stable.          |
| 4   | Gin-only cutover             | Legacy fallback is removed and route inventory has zero unmigrated routes.     |

## Final State

| Area            | State                                                                             |
|-----------------|-----------------------------------------------------------------------------------|
| Gin routes      | Every `/api/` route                                                               |
| Legacy fallback | Removed                                                                           |
| Boundary checks | Gin imports restricted to the HTTP transport boundary under `internal/server/api` |
| Route inventory | PM-031 inventory has zero fallback-owned routes                                   |
| Error handling  | `WriteError` behavior is preserved; typed mapper remains available                |

## Flow 1: Read Route Family Migration

| Step | Action                                                                    |
|------|---------------------------------------------------------------------------|
| 1    | Select the next read route family from the migration order.               |
| 2    | Add or extend parity tests using current `ServeMux` behavior as baseline. |
| 3    | Register the route family on Gin and call existing services.              |
| 4    | Verify status, content type, query defaults, and JSON shape.              |
| 5    | Remove the legacy mux registration for that family after tests pass.      |

## Flow 2: Write Route Family Migration

| Step | Action                                                                        |
|------|-------------------------------------------------------------------------------|
| 1    | Add tests for request decoding, validation errors, audit events, and refresh. |
| 2    | Migrate Gin handlers using typed errors where compatible.                     |
| 3    | Preserve write side effects: registry updates, scan refresh, index updates.   |
| 4    | Run frontend typecheck and focused API tests.                                 |
| 5    | Remove legacy mux registration only for covered writes.                       |

## Flow 3: Streaming Migration

| Step | Action                                                                        |
|------|-------------------------------------------------------------------------------|
| 1    | Lock current streaming behavior with lifecycle tests.                         |
| 2    | Migrate normal verification and AI session metadata routes first.             |
| 3    | Migrate WebSocket or stream route with upgrade, cancel, and disconnect tests. |
| 4    | Confirm no goroutine or process leaks on shutdown.                            |

## Flow 4: Gin-only Cutover

| Step | Action                                                                    |
|------|---------------------------------------------------------------------------|
| 1    | Route inventory check reports no legacy API mux registrations.            |
| 2    | Remove `ServeMux` fallback from Gin transport.                            |
| 3    | Update architecture docs and PM-031 completion report.                    |
| 4    | Run full backend tests, frontend typecheck, and selected benchmark suite. |

## Edge Cases

| Case                        | Expected Handling                                                           |
|-----------------------------|-----------------------------------------------------------------------------|
| Missing route after cutover | Tests fail because there is no fallback to hide the missing Gin route.      |
| Gin path param mismatch     | Parity tests catch incorrect parameter names or route ordering.             |
| Decode behavior drift       | Validation/error tests catch bad request body or unknown field differences. |
| Streaming disconnect        | Lifecycle tests verify cleanup and no leaked goroutine or PTY process.      |
| Frontend contract drift     | `rtk npm run typecheck` and API tests catch shape or status regressions.    |
