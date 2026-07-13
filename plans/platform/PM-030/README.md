# PM-030: Gin Transport And Core Boundary Hardening

PM-030 modernizes the Go backend HTTP boundary by introducing Gin in a behavior-preserving way, while tightening service, domain, repository, error, cache, and concurrency boundaries that already exist in Kode Stream. The migration keeps existing API routes and JSON contracts stable, avoids Gin types outside transport code, and adds parity checks before moving route groups.

## Proposal Review

| Finding                                                                                                          | Improvement For Plan                                                                | Reason                                                                                                           |
|------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------|
| The proposal combines framework migration, architecture refactor, caching, and concurrency in one broad program. | Split work into gated phases with route parity before performance work.             | Reduces regression risk and keeps each commit reviewable.                                                        |
| "Repository abstractions" is broad against the current package layout.                                           | Start with capability ports around route groups that are migrated first.            | Existing services already wrap registry, item index, scanner, file access, Git, Jira, runtime, and verification. |
| Error mapping is listed before route migration but not tied to current helpers.                                  | Extend `internal/common/httpx` and shared errors before replacing handlers.         | Current handlers use inline status mapping plus `httpx.WriteError`.                                              |
| Caching candidates are named, but cache invalidation rules are not.                                              | Require a cache policy matrix before adding decorators.                             | Existing writes refresh item index and app state; stale cache would break the UI.                                |
| Concurrency policy is useful but too large for the first Gin slice.                                              | Limit first concurrency work to context propagation and one bounded heavy workflow. | Current hotspots include Git commands, runtime health checks, knowledge enrichment, and embedded AI sessions.    |
| Dual-stack switch is mentioned without a retirement rule.                                                        | Add explicit stop criteria for removing net/http route paths.                       | Prevents long-lived duplicated transport code.                                                                   |
| Success criteria mention p95 latency but no baseline source.                                                     | Add benchmark and route inventory as Phase B1 deliverables.                         | Current implementation has many route-level tests but no transport baseline report.                              |

## Related Plans

| Item                          | Relationship          | Key Context                                                                                    |
|-------------------------------|-----------------------|------------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Product baseline      | Established workspace registry, item index cache, scanner, HTTP API, and local app constraints |
| [PM-003](../PM-003/README.md) | Architecture baseline | Established behavior-preserving refactors, service extraction, and package boundary direction  |
| [PM-004](../PM-004/README.md) | Reliability baseline  | Added audit, health, safety checks, recovery hints, and regression tests                       |
| [PM-029](../PM-029/README.md) | Heavy workflow input  | Added runtime and automation verification jobs that need clear timeout and cancellation policy |

## Current Implementation Summary

| Area              | Current State                                                                                                                            | PM-030 Impact                                                        |
|-------------------|------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------|
| Server bootstrap  | `internal/server/server.go` wires services and mounts `/api/` on `http.NewServeMux`.                                                     | Add Gin as a replaceable API transport without changing SPA serving. |
| API routing       | `internal/server/api/api.go` registers most routes using Go 1.22 `ServeMux` patterns.                                                    | Introduce route-group migration and route inventory.                 |
| Split controllers | `audit`, `navigation`, `system`, and `workspace` health already register routes separately.                                              | Use these smaller controllers as low-risk Gin migration candidates.  |
| Responses         | `internal/common/httpx` writes JSON and error envelopes.                                                                                 | Preserve envelope shape while adding domain error mapping.           |
| Domain errors     | `internal/common/common_errors.go` has only shared not-found errors.                                                                     | Add typed domain error codes before transport conversion.            |
| Repositories      | Registry, item index, file access, Git, Jira, navigation, audit, knowledge, runtime, and verification are concrete package dependencies. | Add ports only where route migration or tests need seams.            |
| Tests             | Backend has broad package and API tests through `go test ./...`.                                                                         | Add contract/parity tests around migrated route groups.              |

## Glossary

| Term            | Meaning                                                                                       | Code                                                                                               |
|-----------------|-----------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|
| Gin Transport   | Gin router, middleware, request decoding, response encoding, and route groups                 | `internal/server/api` or future `internal/httpapi`                                                 |
| Core Layer      | Application services, domain rules, repositories, adapters, and models below HTTP             | `internal/workspace`, `internal/item`, `internal/git`, `internal/runtime`, `internal/verification` |
| Domain Error    | Typed application error with stable code and default HTTP mapping                             | `internal/common`                                                                                  |
| Error Mapper    | Transport helper that converts domain errors into the existing JSON envelope                  | `internal/common/httpx` plus Gin adapter                                                           |
| Route Parity    | Same method, path, status, headers where relevant, and JSON shape under old and new transport | API tests                                                                                          |
| Cache Decorator | Optional wrapper around a read-heavy service or repository with TTL and invalidation rules    | read-side adapters                                                                                 |
| Bounded Worker  | Limited queue and worker count for long-running jobs with cancellation and shutdown behavior  | runtime, verification, knowledge, or AI workflows                                                  |

## Scope

In scope:

- Add Gin dependency and transport adapter.
- Preserve all existing public API routes and response payloads during migration.
- Add route inventory and parity tests.
- Add domain error codes and centralized mapping for both `net/http` and Gin handlers.
- Migrate low-risk read route groups first.
- Add cache and concurrency policies with one measured implementation each.
- Add CI or test checks that keep Gin imports out of core packages.

Out of scope:

- Frontend API contract changes.
- Replacing the embedded SPA handler.
- Rewriting all services or repositories at once.
- Switching data storage away from YAML or local files.
- Moving WebSocket terminal streaming until normal HTTP route parity is stable.

## Data Flow

Request -> Gin middleware -> route handler -> decoder -> application service -> repository or adapter -> domain error/result -> mapper -> existing JSON envelope.

Core packages must not import Gin. Gin context is translated to standard request data, `context.Context`, and existing model types before calling services.

## Design Decisions

| Decision                                | Alternatives Considered                            | Rationale                                                                                       |
|-----------------------------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------------|
| Keep Gin at the transport boundary only | Pass `gin.Context` into services                   | Keeps service tests simple and avoids framework lock-in.                                        |
| Migrate route groups incrementally      | Replace `ServeMux` in one large change             | Route count is high and several handlers include file, Git, runtime, and WebSocket behavior.    |
| Start with read-only routes             | Start with write or streaming routes               | Read routes are safer for parity and do not trigger index refresh or Git side effects.          |
| Preserve `httpx` envelope semantics     | Introduce a new error response shape               | Frontend behavior depends on current `error` and optional `recoveryHint` fields.                |
| Add ports only where useful             | Create interfaces for every repository immediately | Current concrete dependencies are acceptable until tests or migration seams require interfaces. |
| Gate cache work behind policy           | Add ad hoc local caches                            | Kode Stream already depends on index freshness and app state version changes.                   |
| Add import boundary checks              | Rely on code review                                | Automated checks prevent Gin leakage into core packages.                                        |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Infrastructure Design](design/design-02-infrastructure.md)
- [Pipeline Design](design/design-03-pipeline.md)
- [Route Inventory](route-inventory.md)
- [Implementation Plan](implementation-plan.md)
