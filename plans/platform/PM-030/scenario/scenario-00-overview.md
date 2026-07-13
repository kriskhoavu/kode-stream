# Scenarios: PM-030 Overview

## Scenario List

| #   | Title                                 | Description                                                                |
|-----|---------------------------------------|----------------------------------------------------------------------------|
| 0   | Current `net/http` transport          | Existing backend routes run through `http.NewServeMux`.                    |
| 1   | Read route migrated to Gin            | A low-risk read route returns the same response through Gin.               |
| 2   | Domain error mapped centrally         | A service returns a typed domain error and transport maps it consistently. |
| 3   | Cache policy applied to a read path   | A measured read path uses a decorator without stale write behavior.        |
| 4   | Heavy job respects concurrency policy | A bounded workflow honors context timeout, cancellation, and shutdown.     |

## Scenario 0: Current Transport Baseline

### Goal

Record the current route behavior before introducing Gin.

### Starting State

| #   | Title       | Summary                                                                 |
|-----|-------------|-------------------------------------------------------------------------|
| 1   | Routes      | Most API routes are registered in `internal/server/api/api.go`.         |
| 2   | Controllers | Audit, navigation, system, and health routes use package controllers.   |
| 3   | Responses   | JSON and error responses use `internal/common/httpx` or local wrappers. |
| 4   | Tests       | Package and API tests run with `go test ./...`.                         |

### Execution Flow

Tester calls existing route -> `ServeMux` matches pattern -> handler decodes request -> service runs -> handler writes JSON through `httpx` -> contract test records status and payload.

## Scenario 1: Read Route Migrated To Gin

### Goal

Migrate one route group without changing client-visible behavior.

### Starting State

| #   | Title           | Summary                                                               |
|-----|-----------------|-----------------------------------------------------------------------|
| 1   | Candidate group | Health, audit, navigation, search, or state read routes are selected. |
| 2   | Contract tests  | Existing response shape tests exist or are added first.               |
| 3   | Boundary rule   | Core packages do not import Gin.                                      |

### Execution Flow

Client calls existing endpoint -> Gin route group handles request -> adapter extracts parameters and context -> existing service runs -> mapper writes existing JSON envelope -> parity test compares old and new behavior.

### Edge Cases

- Missing optional service returns the same fallback response.
- Query limits and defaults remain unchanged.
- Unknown route behavior remains unchanged under `/api/`.

## Scenario 2: Domain Error Mapped Centrally

### Goal

Remove repeated inline status mapping from migrated handlers.

### Starting State

| #   | Title         | Summary                                                                 |
|-----|---------------|-------------------------------------------------------------------------|
| 1   | Domain error  | Service returns a typed error code such as `not_found` or `validation`. |
| 2   | Mapper        | Transport maps error code to HTTP status and JSON payload.              |
| 3   | Compatibility | Existing `error` field and recovery hint behavior remain supported.     |

### Execution Flow

Service returns typed error -> handler passes error to mapper -> mapper selects status -> response uses existing envelope -> test asserts status, `error`, and optional `code`.

## Scenario 3: Cache Policy Applied To A Read Path

### Goal

Improve one measured read-heavy path without stale data.

### Starting State

| #   | Title      | Summary                                                                               |
|-----|------------|---------------------------------------------------------------------------------------|
| 1   | Candidate  | Workspace runtime config, item detail/index read, or verification discovery metadata. |
| 2   | Policy     | TTL, key, invalidation trigger, and metric are documented.                            |
| 3   | Write path | Writes still refresh index or invalidate the cache.                                   |

### Execution Flow

Read request arrives -> cache decorator checks key -> miss calls underlying service -> result stored with TTL -> writes invalidate affected keys -> metrics expose hit and miss counts.

## Scenario 4: Heavy Job Respects Concurrency Policy

### Goal

Prevent unbounded long-running work and goroutine leaks.

### Starting State

| #   | Title     | Summary                                                                                               |
|-----|-----------|-------------------------------------------------------------------------------------------------------|
| 1   | Candidate | Runtime verification, knowledge enrichment, Git command execution, or embedded AI session management. |
| 2   | Policy    | Deadline, queue size, worker count, and cancellation behavior are defined.                            |
| 3   | Shutdown  | Server shutdown closes owned sessions and workers cleanly.                                            |

### Execution Flow

Job request arrives -> context deadline is set -> job enters bounded queue -> worker runs adapter command -> cancellation propagates -> status and logs are returned or persisted.
