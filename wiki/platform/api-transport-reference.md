---
slug: platform-api-transport-reference
title: API Transport Reference
pageType: REFERENCE
topics: api, gin, middleware, authorization, performance
summary: Reference for the single Gin API transport, route ownership, middleware, and migration measurements.
sourceRef: |
  plans/platform/PM-030/README.md | Gin migration
  plans/platform/PM-030/proposal.md | Transport proposal
  plans/platform/PM-030/route-inventory.md | Migration inventory
  plans/platform/PM-030/performance-scorecard.md | Baseline measurements
  plans/platform/PM-031/README.md | Gin-only transport
  plans/platform/PM-031/route-inventory.md | Final route inventory
  plans/platform/PM-031/performance-scorecard.md | Final measurements
lastVerified: 2026-09-12
---

## Single Transport
<!-- chunkId: platform-api-transport-reference-single-transport -->
<!-- keywords: gin engine, api routes, route manifest, handlers -->

[live] One Gin engine registers the API route manifest, and the manifest is shared by registration and policy tests. Evidence: `internal/server/api/gin_transport.go:20`, `internal/server/api/gin_routes.go:80`.

The earlier dual-router migration state is historical. Gin parameters are adapted to the existing HTTP handlers, while the separate SPA handler continues to own frontend assets.

## Middleware And Policy
<!-- chunkId: platform-api-transport-reference-middleware-policy -->
<!-- keywords: recovery, request id, timeout, cors, capability -->

[live] The API stack applies recovery, Local extension CORS, request-ID propagation, and a 30-second request context; every protected route owner must declare an authorization policy. Evidence: `internal/server/api/gin_transport.go:18`, `internal/server/api/gin_routes.go:31`.

The request-ID middleware propagates a supplied ID; it does not promise to generate one. The timeout cancels the request context and is not a guarantee that every handler has already stopped work or returned within 30 seconds.

## Historical Measurements
<!-- chunkId: platform-api-transport-reference-historical-measurements -->
<!-- keywords: benchmark scorecard, allocation count, audit fixture, historical result -->

[legacy] The PM-030 and PM-031 scorecards are development snapshots from Apple Silicon, useful for regression context but not current service-level objectives. Source: `PM-030 (plan retired) | Performance scorecard`, `PM-031 (plan retired) | Performance scorecard`.

The last recorded PM-031 samples reported health at 1,383 ns/op and 2,066 B/op, search at 1,279 ns/op and 1,642 B/op, and item detail at 5,133 ns/op and 3,064 B/op. The audit sample remained fixture and file-I/O dominated, so it does not establish a warm-cache improvement.

## Not supported
<!-- chunkId: platform-api-transport-reference-not-supported -->
<!-- keywords: unsupported transport, dual router, latency objective -->

[live] A second API router and implicit policy for a new protected owner are not supported. Evidence: `internal/server/api/gin_routes.go:31`, `internal/server/api/gin_routes.go:74`.

## Known conflicts
<!-- chunkId: platform-api-transport-reference-known-conflicts -->
<!-- keywords: stale route list, warm cache, percentile measurement -->

[open] What are the current warm-cache p50 and p95 values for representative routes? The retired scorecards did not measure these percentiles and must not be presented as current performance.

## Related pages

See [[platform-cloud-runtime-reference]], [[platform-storage-reference]], and [[workspace-index]].
