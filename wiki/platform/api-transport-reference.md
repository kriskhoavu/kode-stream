---
slug: platform-api-transport-reference
title: API Transport Reference
pageType: REFERENCE
topics: api, gin, middleware, authorization, performance
summary: Reference for the single Gin API transport, route ownership, middleware, and migration measurements.
sourceRef: |
  PM-030 (plan retired) | Gin migration
  PM-030 (plan retired) | Transport proposal
  PM-030 (plan retired) | Migration inventory
  PM-030 (plan retired) | Baseline measurements
  PM-031 (plan retired) | Gin-only transport
  PM-031 (plan retired) | Final route inventory
  PM-031 (plan retired) | Final measurements
lastVerified: 2026-09-12
---

## Single Transport
<!-- chunkId: platform-api-transport-reference-single-transport -->
<!-- keywords: gin engine, route manifest, handlers, spa handler -->

[live] One Gin engine registers the API route manifest. Registration and policy tests share that manifest. Evidence: `internal/server/api/gin_transport.go:20`, `internal/server/api/gin_routes.go:82`.

Gin parameters are adapted to the existing HTTP handlers. A separate SPA handler serves the frontend assets.

## Middleware And Policy
<!-- chunkId: platform-api-transport-reference-middleware-policy -->
<!-- keywords: recovery, request id, timeout, cors, capability -->

[live] The API stack applies recovery, Local extension CORS, request-ID propagation and a 30-second request context. Every protected route owner must declare an authorization policy. Evidence: `internal/server/api/gin_transport.go:26`, `internal/server/api/gin_routes.go:31`.

- The request-ID middleware passes on a supplied ID. It does not generate one.
- The timeout cancels the request context. It does not guarantee that a handler stops within 30 seconds.

## Error Envelope
<!-- chunkId: platform-api-transport-reference-error-envelope -->
<!-- keywords: domain error code, http status, recovery hint -->

Domain errors carry stable codes that the API maps to HTTP statuses: `validation` 400, `unauthorized` 401, `forbidden` 403, `not_found` 404, `conflict` 409, `infra` 500, `unavailable` 503. The JSON body holds `error` and optional `code` and `recoveryHint`.

## Not supported
<!-- chunkId: platform-api-transport-reference-not-supported -->
<!-- keywords: second router, implicit policy, unprotected owner -->

[live] A second API router, and a new protected owner without an explicit policy. Evidence: `internal/server/api/gin_routes.go:71`.

## Known conflicts
<!-- chunkId: platform-api-transport-reference-known-conflicts -->
<!-- keywords: warm cache, percentile measurement, benchmark -->

[open] What are the warm-cache p50 and p95 values for typical routes? Old benchmark scorecards are not current performance.

## Related pages

See [[platform-application-reference]] and [[platform-cloud-runtime-reference]].
