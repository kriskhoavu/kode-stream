# Backend Design: Gin Transport And Core Boundaries

## Overview

PM-030 adds Gin as the backend API transport without changing business behavior. The current `net/http` routing stays available until route groups pass parity tests. Core services and repositories receive standard inputs, `context.Context`, and existing model types rather than Gin-specific types.

## Current Backend Shape

| Package                                                    | Current Responsibility                                        | Migration Note                                              |
|------------------------------------------------------------|---------------------------------------------------------------|-------------------------------------------------------------|
| `internal/server`                                          | Process wiring, embedded frontend, API mount, signal handling | Keep SPA handling and server shutdown stable.               |
| `internal/server/api`                                      | Main API route registration and handlers                      | Primary migration target.                                   |
| `internal/common/httpx`                                    | Shared JSON and error envelope helpers                        | Extend for domain error mapping and Gin adapter.            |
| `internal/common`                                          | Shared errors and cross-package model support                 | Add typed domain errors here or in a subpackage.            |
| `internal/workspace`                                       | Workspace services, file service, health controller           | Keep framework-agnostic.                                    |
| `internal/item`                                            | Item service and writer                                       | Keep framework-agnostic.                                    |
| `internal/git`                                             | Git adapter and service                                       | Keep command timeout behavior; pass context where feasible. |
| `internal/runtime`                                         | Runtime config and command execution                          | Candidate for timeout and bounded job policy.               |
| `internal/verification`                                    | Verification jobs and artifacts                               | Candidate for concurrency policy.                           |
| `internal/navigation`, `internal/audit`, `internal/system` | Smaller HTTP controllers and repositories                     | Good first route-group candidates.                          |

## Target Package Boundaries

| Layer              | Responsibility                                                           | Rule                                                                                                                                                         |
|--------------------|--------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Transport          | Gin router, middleware, route groups, request decoding, response writing | May import Gin inside the HTTP transport boundary under `internal/server/api`, including handlers, middleware, routing, adapters, and transport-level tests. |
| Application        | Use-case orchestration and service methods                               | Must not import Gin.                                                                                                                                         |
| Domain             | Error codes, validation rules, item/workspace concepts                   | Must not import Gin or storage adapters.                                                                                                                     |
| Repository/Adapter | YAML persistence, filesystem, Git, Jira, runtime commands                | Must not import Gin.                                                                                                                                         |

Recommended package movement is incremental. Do not rename all packages before route migration proves the shape.

## API Route Groups

| Group           | Example Routes                                  | First Migration Risk | Notes                                                      |
|-----------------|-------------------------------------------------|----------------------|------------------------------------------------------------|
| Health          | `GET /api/health`                               | Low                  | No route parameters and simple JSON response.              |
| Audit           | `GET /api/audit-events`                         | Low                  | Query parsing and optional repository fallback.            |
| Navigation      | `/api/saved-filters`, `/api/recent-items`       | Medium               | Has validation and item not-found mapping.                 |
| State/Search    | `GET /api/state`, `GET /api/search`             | Medium               | Good parity coverage needed for filters and ranking.       |
| Workspace reads | `GET /api/workspaces`, tree, file reads         | Medium               | Path safety and source scope must remain unchanged.        |
| Item reads      | `GET /api/items`, detail, files, diff           | Medium               | JSON shape stability is critical for frontend.             |
| Writes and Git  | Workspace, item, file, metadata, Git operations | High                 | Preserve guards, audit, index refresh, and recovery hints. |
| Streaming       | WebSocket AI session channel                    | High                 | Defer until normal HTTP parity is stable.                  |

## Domain Error Model

| Code           | Default Status | Meaning                                                                   | Existing Examples                                 |
|----------------|----------------|---------------------------------------------------------------------------|---------------------------------------------------|
| `not_found`    | 404            | Requested workspace, item, session, file, or saved filter does not exist. | `ErrWorkspaceNotFound`, `ErrItemNotFound`         |
| `validation`   | 400            | Request body, query, path, branch, file, or source is invalid.            | invalid JSON, invalid route, invalid path         |
| `conflict`     | 409            | Write cannot proceed because state changed or operation conflicts.        | stale file, dirty Git state                       |
| `unauthorized` | 401            | Caller is not authenticated when auth exists.                             | Reserved                                          |
| `forbidden`    | 403            | Operation is outside allowed workspace/source/path scope.                 | path guard rejection                              |
| `unavailable`  | 503            | Optional service or dependency is not configured.                         | missing AI, navigation, audit, runtime dependency |
| `infra`        | 500            | Unexpected adapter, filesystem, Git, process, or persistence failure.     | command failure without a specific user error     |

The mapper must preserve the current envelope. The response should keep `error` as the user-facing message. It may add `code` once tests prove frontend compatibility.

## Gin Adapter Requirements

| Requirement       | Design                                                                                                   |
|-------------------|----------------------------------------------------------------------------------------------------------|
| Route parameters  | Convert Gin params to plain strings before calling services.                                             |
| Query parameters  | Keep current defaults and limits per route.                                                              |
| Request body      | Continue rejecting malformed JSON and unknown fields where current handlers do.                          |
| Response envelope | Use `httpx` semantics through a Gin-specific writer helper.                                              |
| Request context   | Pass `c.Request.Context()` to services that support context.                                             |
| Middleware        | Add request ID, structured logging, panic recovery, timeout, and CORS only where behavior is equivalent. |
| WebSocket         | Defer migration of session channel until non-streaming routes are stable.                                |

## Cache Policy Matrix

| Candidate                       | Cache Key                                 | TTL                 | Invalidation                                                | Metric                        |
|---------------------------------|-------------------------------------------|---------------------|-------------------------------------------------------------|-------------------------------|
| Workspace runtime config        | workspace ID and runtime settings path    | Short, configurable | Runtime settings save                                       | hit, miss, stale invalidation |
| Item detail/index read          | workspace ID, item ID, branch/source mode | Short, configurable | item metadata/file/status write, scan, branch snapshot load | hit, miss, invalidation count |
| Verification discovery metadata | item ID and automation repository config  | Short, configurable | verification test save, automation repo settings change     | hit, miss, discovery duration |

Cache decorators must be opt-in, tested with fake clocks, and removable without changing service contracts.

## Concurrency Policy

| Concern       | Policy                                                                                            |
|---------------|---------------------------------------------------------------------------------------------------|
| Deadlines     | Long-running handlers must have explicit context deadlines or inherit configured request timeout. |
| Cancellation  | Adapter commands should use context-aware execution where available.                              |
| Worker limits | Heavy jobs should use bounded worker count and queue size.                                        |
| Backpressure  | Full queues return a typed `unavailable` or `conflict` error with a clear message.                |
| Shutdown      | Server shutdown must close sessions and workers without leaking goroutines.                       |
| Metrics       | Track queued, running, completed, failed, canceled, and timed-out jobs.                           |

Implemented concurrency pilot:

| Workflow             | Default Limit  | Default Timeout | Queue-Full Behavior           | Shutdown Behavior                 |
|----------------------|----------------|-----------------|-------------------------------|-----------------------------------|
| Verification service | 2 running jobs | 10 minutes      | Rejects new job with an error | Cancels service context and jobs. |

## Testing Strategy

| Test Type            | Purpose                                                                        |
|----------------------|--------------------------------------------------------------------------------|
| Route inventory test | Detect accidental method/path changes during migration.                        |
| Contract tests       | Lock status and JSON response shape for selected routes.                       |
| Parity tests         | Run equivalent requests against old and Gin handlers during dual-stack period. |
| Error mapper tests   | Verify domain error code to status and payload mapping.                        |
| Import boundary test | Fail if Gin appears outside the HTTP transport boundary.                       |
| Cache tests          | Verify TTL, keying, invalidation, and fake-clock behavior.                     |
| Concurrency tests    | Verify deadline, cancellation, full queue, and shutdown behavior.              |

## Design Decisions

| Decision                                           | Rationale                                                                                      |
|----------------------------------------------------|------------------------------------------------------------------------------------------------|
| Keep old transport during early Gin migration      | Enables direct parity checks and safer rollback.                                               |
| Add typed errors before migrating complex handlers | Reduces duplicated status mapping in both transports.                                          |
| Migrate small controllers first                    | Existing controller boundaries make audit, health, system, and navigation easier to isolate.   |
| Defer WebSocket migration                          | It has distinct origin checks, subscription lifecycle, goroutines, and binary/base64 behavior. |
| Avoid full repository interface sweep              | Interfaces should match real route seams and tests, not speculative architecture.              |

## Implemented Seams

| Interface          | Owner                 | Concrete Implementation | Purpose                                  |
|--------------------|-----------------------|-------------------------|------------------------------------------|
| `auditEventReader` | `internal/server/api` | `audit.Store`           | Read recent audit events for Gin routes. |
