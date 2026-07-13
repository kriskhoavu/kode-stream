# PM-030 Proposal: Gin Adoption with Service/Domain/Repository Improvements

## Objective

Modernize backend delivery and maintainability by adopting Gin at the HTTP boundary while refactoring service/domain/repository layers for clarity, extensibility, and operational reliability.

Target outcomes:

- better API development ergonomics
- clearer architecture for Java/Spring-style onboarding
- safer concurrency model
- consistent error handling
- measurable caching and performance improvements

## Why This Approach

Gin alone improves transport-layer developer experience, but it does not automatically solve architecture quality, concurrency, or caching.

This proposal combines:

1. Gin at the transport layer
2. Core-layer refactoring (service/domain/repository)
3. Shared policies for error mapping, caching, and concurrency

This avoids framework lock-in in business logic and keeps the system easier to test and evolve.

## Scope

In scope:

- HTTP migration to Gin (incremental)
- clearer interfaces/ports across core modules
- package boundary cleanup
- domain error model and centralized HTTP mapper
- repository abstractions
- caching decorators
- concurrency policies (worker pools, deadlines, cancellation)

Out of scope (initial phases):

- full data-store migration
- complete module rewrites in one release
- replacing all infrastructure adapters at once

## Proposed Architecture

Transport uses Gin for routing, middleware, request decoding, and response writing. Application services own use cases. Domain packages own entities, validation, errors, and ports. Infrastructure packages implement repository, cache, Git, filesystem, runtime, Jira, and verification adapters.

Dependency rule:

- `transport -> application -> domain`
- `infrastructure` implements `domain/application` ports
- no Gin types outside transport

## Review Amendments

The current implementation already has several service and repository packages, a shared `httpx` response helper, broad API tests, and separate controllers for audit, navigation, system, and health routes. PM-030 should therefore avoid a broad rewrite and use these improvements:

- Treat Gin as a transport replacement, not an application architecture rewrite.
- Add a route inventory and baseline benchmark before the first migration.
- Add typed domain errors and a central mapper before complex routes move to Gin.
- Migrate low-risk read route groups first, such as health, audit, navigation reads, state, or search.
- Keep WebSocket and write/Git workflows on the old transport until parity tests cover normal HTTP routes.
- Add repository interfaces only where route migration or tests need seams.
- Require cache TTL, key, invalidation, and metrics before adding decorators.
- Pilot concurrency policy on one heavy workflow before standardizing worker pools.
- Add an automated boundary check that rejects Gin imports outside transport packages.
- Define old-transport removal criteria so dual-stack code does not become permanent.

## Delivery Plan

### Phase 0: Baseline and Safety

- inventory existing endpoints and handlers
- capture p50/p95 latency baseline for key APIs
- lock contract tests for critical endpoints

Deliverables:

- API inventory
- baseline benchmark report
- parity checklist

### Phase 1: Architecture Blueprint

- define package structure and dependency constraints
- establish coding conventions for handler/service/repo boundaries
- document standard request/response and error envelope

Deliverables:

- architecture ADR
- boundary rules document

### Phase 2: Domain Interfaces and Error Model

- introduce clear service ports/use-case interfaces
- define repository interfaces by capability
- add typed domain errors:
  - `not_found`
  - `validation`
  - `conflict`
  - `unauthorized`
  - `forbidden`
  - `infra`
- implement transport mapper: domain error -> HTTP status + response payload

Deliverables:

- shared domain error package
- centralized error mapper middleware/helper

### Phase 3: Repository Abstractions

- wrap filesystem/git/index operations behind interfaces
- keep current implementations, move behind ports
- add mocks/fakes for service testing

Deliverables:

- repository interfaces
- adapter implementations
- service tests with fakes

### Phase 4: Gin Transport Migration (Incremental)

- bootstrap Gin router and middleware stack
- migrate read endpoints first, then write, then complex workflows
- preserve parity via contract tests per migrated route

Recommended middleware:

- request ID
- structured logging
- panic recovery
- timeout
- CORS/auth (as required)

Deliverables:

- parity route map
- dual-stack switch (temporary)

### Phase 5: Caching Decorators

- define cache interface with TTL semantics
- implement local cache first; add Redis adapter if needed
- apply decorator pattern on read-heavy use-cases/repositories
- define explicit invalidation on writes

Initial candidates:

- workspace/runtime config reads
- item detail/index reads
- verification discovery metadata

Deliverables:

- cache abstraction
- cache policy matrix
- hit/miss metrics

### Phase 6: Concurrency Policies

- enforce context deadlines across layers
- replace ad-hoc goroutines with bounded worker pools for heavy jobs
- add cancellation, queue limits, and backpressure
- standardize graceful shutdown behavior

Deliverables:

- worker pool utility/pattern
- concurrency policy document
- queue depth and worker metrics

### Phase 7: Performance and Hardening

- compare before/after latency and throughput
- tune hotspots (serialization, command execution, file access)
- enforce rate limiting and payload guards where appropriate

Deliverables:

- performance scorecard
- operational runbook updates

### Phase 8: Cleanup and Governance

- remove old transport paths after full parity
- add CI checks for boundary rules
- publish extension templates for new modules

Deliverables:

- finalized architecture guidelines
- migration completion report

## Risk Management

Key risks:

- transport migration regressions
- accidental framework leakage into core logic
- cache staleness bugs
- goroutine leaks or unbounded work

Mitigations:

- endpoint-by-endpoint migration with contract tests
- boundary lint/check rules in CI
- conservative TTL + explicit invalidation
- worker limits + context deadlines + shutdown tests

## Success Criteria

- no Gin types in service/domain/repository packages
- standardized domain error mapping for all endpoints
- measurable cache hit ratio on selected read paths
- reduced p95 latency on targeted APIs
- stable concurrency metrics under load (bounded queues/workers)
- easier onboarding and feature extension flow

## Recommended Initial Backlog (First Sprint)

1. Create architecture ADR and dependency rules
2. Introduce shared domain error package and HTTP mapper
3. Bootstrap Gin router with logging/recovery/timeout middleware
4. Migrate 2-3 low-risk read endpoints with parity tests
5. Add cache interface and one decorator for a read-heavy endpoint
6. Add context timeout policy and one bounded worker pool for a heavy job
