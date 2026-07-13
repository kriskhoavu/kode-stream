# Pipeline Design: PM-031

## Overview

PM-031 uses a gated migration pipeline. Each route family must pass focused parity tests and update the inventory before its legacy registration is removed. Final cutover removes fallback only after all route families pass.

## Pipeline Stages

| Stage                   | Gate                                                                  |
|-------------------------|-----------------------------------------------------------------------|
| Baseline route snapshot | Inventory and existing tests identify current behavior.               |
| Family parity tests     | Route family has focused status, body, error, and side-effect checks. |
| Gin registration        | Routes are registered on Gin and use existing services.               |
| Legacy family cleanup   | Matching `mux.HandleFunc` registrations are removed.                  |
| Route inventory update  | Family status changes from fallback to Gin-owned.                     |
| Full verification       | `rtk go test ./...` passes.                                           |
| Frontend verification   | `rtk npm run typecheck` passes for contract-sensitive families.       |
| Final cutover           | Fallback removed and missing route behavior is explicit.              |

## Migration Gates

| Gate             | Required Evidence                                                                  |
|------------------|------------------------------------------------------------------------------------|
| Contract gate    | Focused tests for route shape and error behavior.                                  |
| Side-effect gate | Writes preserve scan, index, audit, and refresh behavior.                          |
| Boundary gate    | Gin imports remain inside the HTTP transport boundary under `internal/server/api`. |
| Inventory gate   | Route inventory matches migrated and remaining route state.                        |
| Streaming gate   | WebSocket or stream lifecycle tests cover disconnect cleanup.                      |
| Cutover gate     | No legacy API route registrations remain.                                          |

## Options Considered

| Option                     | Pros                                         | Cons                                                       |
|----------------------------|----------------------------------------------|------------------------------------------------------------|
| One-shot Gin rewrite       | Fast apparent completion                     | High regression risk across writes, Git, files, streaming. |
| Family-by-family migration | Small commits, clear rollback, focused tests | More phases and temporary dual-stack complexity.           |
| Keep fallback permanently  | Lowest immediate risk                        | Leaves duplicate routing and unfinished migration.         |

## Selected Approach

Family-by-family migration with explicit cutover. This matches PM-030 and avoids a broad rewrite while still ending in a Gin-only API.

## Final Cutover Requirements

| Requirement           | Expected Final State                                  |
|-----------------------|-------------------------------------------------------|
| Legacy fallback       | Removed from API transport.                           |
| `ServeMux` API routes | Removed from `API.Routes()`.                          |
| Gin route inventory   | Covers every `/api/` route.                           |
| SPA serving           | Still served by `internal/server` outside Gin.        |
| Governance tests      | Fail if route inventory or import boundaries regress. |
