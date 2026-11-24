---
slug: platform-application-reference
title: Application Architecture And Distribution Reference
pageType: REFERENCE
topics: application architecture, domain packages, frontend features, release, doctor
summary: Reference for Kode Stream's current domain-service architecture, centralized API transport, embedded frontend, and packaged release boundary.
sourceRef: |
  PM-003 (plan retired) | Architecture refactor
  PM-015 (plan retired) | Performance and render review
  PM-017 (plan retired) | Rollout and distribution
  PM-023 (plan retired) | Domain-oriented packaging
lastVerified: 2026-09-12
---

# Application Architecture And Distribution Reference

## Current Application Boundary
<!-- chunkId: platform-application-reference-boundary -->
<!-- keywords: Go service, embedded assets, domain packages -->

[live] Kode Stream is one Go binary. The server composes the domain services and exposes them through one Gin transport package. The built web client is embedded in the same binary. Evidence: `internal/server/server.go:66`, `internal/server/api/boundary_test.go:37`.

Domain packages own behavior. `internal/server/api` owns HTTP adaptation and routes, not domain policy. The package map is in `docs/architecture/overview.md`.

## Frontend Boundary
<!-- chunkId: platform-application-reference-frontend -->
<!-- keywords: React, feature module, shared API, lazy chunks -->

[live] The client is built from feature modules composed by pages. The router and the shared API client are its integration points. Evidence: `web/src/app/router.ts:9`, `web/src/shared/api/index.ts:17`.

Large surfaces may load lazily, but the production build still reports a large main chunk.

## Release Artifacts
<!-- chunkId: platform-application-reference-release -->
<!-- keywords: GitHub release, darwin, linux, windows, checksum -->

[live] A tagged release builds macOS arm64 and amd64, Linux amd64 and Windows amd64 artifacts and publishes checksums. Evidence: `.github/workflows/release.yml:142`.

Each binary includes the frontend. Release CI checks the lockfile's registry origin and exercises the local and cloud database paths before it packages.

## Installation And Diagnosis
<!-- chunkId: platform-application-reference-installation -->
<!-- keywords: Homebrew, formula, doctor -->

[live] Local installation uses the `kriskhoavu/homebrew-tap` formula. `kode-stream doctor` checks the environment and repository setup. Evidence: `cmd/kode-stream/main.go:41`.

The formula updater moves every release URL and checksum together, so a formula never mixes versions.

## Not supported
<!-- chunkId: platform-application-reference-not-supported -->
<!-- keywords: plan CLI, init command, publish command -->

[live] The CLI has no `init`, `plan create` or `plan publish` command. Evidence: `cmd/kode-stream/main.go:27`.

## Known conflicts
<!-- chunkId: platform-application-reference-known-conflicts -->
<!-- keywords: local only premise, package topology, performance review -->

- "Kode Stream is local-only." It also runs as a Cloud control plane. See [[platform-cloud-runtime-reference]].

## Related pages

See [[platform-api-transport-reference]] and [[platform-storage-reference]].
