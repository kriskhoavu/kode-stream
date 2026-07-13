# Infrastructure Design: PM-030

## Overview

PM-030 is a local Go backend refactor. Infrastructure work focuses on dependency management, verification commands, static boundary checks, and benchmark artifacts. It does not require Docker, Kubernetes, external services, or frontend build changes for the first phases.

## Dependency Changes

| Dependency                 | Purpose                                                 | Constraint                                                                 |
|----------------------------|---------------------------------------------------------|----------------------------------------------------------------------------|
| `github.com/gin-gonic/gin` | HTTP router and middleware stack                        | Add only when first Gin route group is implemented.                        |
| Gin middleware packages    | Optional request ID, logging, timeout, recovery helpers | Prefer small local middleware unless external packages are clearly needed. |

## Verification Commands

| Command                                                   | Purpose                                                 | When                                |
|-----------------------------------------------------------|---------------------------------------------------------|-------------------------------------|
| `rtk go test ./...`                                       | Full backend regression suite                           | Every backend phase                 |
| `rtk go test ./internal/server/... ./internal/common/...` | Focused transport and mapper tests                      | Error and route migration phases    |
| `rtk go test -run TestRoute`                              | Route inventory and parity checks                       | During migration phases             |
| `rtk go test -bench . ./internal/server/...`              | Transport benchmark baseline and comparison             | Baseline and hardening phases       |
| `rtk npm run typecheck`                                   | Verify unchanged frontend API contract at compile level | Before removing old transport paths |

## Boundary Checks

| Check                 | Rule                                                   | Failure Example                                    |
|-----------------------|--------------------------------------------------------|----------------------------------------------------|
| Gin import boundary   | Only transport packages may import Gin                 | `internal/item` imports `github.com/gin-gonic/gin` |
| Route inventory       | Registered method/path set must match approved changes | Missing `GET /api/items/{id}`                      |
| Error envelope        | Error payload must include current `error` field       | New mapper omits `error`                           |
| Response content type | JSON routes must return `application/json`             | Gin default differs from current helper            |

## Benchmark Artifacts

| Artifact              | Content                                                      | Owner   |
|-----------------------|--------------------------------------------------------------|---------|
| Route inventory       | Method, path, handler owner, risk level, migration status    | Backend |
| Baseline report       | p50, p95, allocations, route group tested, test data size    | Backend |
| Performance scorecard | Before and after comparison for migrated route groups        | Backend |
| Cache report          | hit rate, miss rate, invalidation count, latency delta       | Backend |
| Concurrency report    | queue depth, worker utilization, cancel and timeout behavior | Backend |

## Design Decisions

| Decision                               | Rationale                                                                             |
|----------------------------------------|---------------------------------------------------------------------------------------|
| Keep verification local                | Kode Stream is a local app and current CI assumptions are lightweight.                |
| Add checks as tests first              | Tests are portable and fit the existing `go test ./...` workflow.                     |
| Avoid deployment changes               | PM-030 changes transport internals, not hosting or packaging behavior.                |
| Benchmark with representative fixtures | Transport speed without realistic handler work is not useful for migration decisions. |
