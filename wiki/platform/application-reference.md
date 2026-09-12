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
<!-- keywords: Go service, web frontend, embedded assets, process -->

[live] PM-003 and PM-023 Kode Stream is a Go application whose domain services are composed by the server and exposed through one Gin transport package; the built web client is embedded into the same distributable. Evidence: `internal/server/server.go:27`, `internal/server/api/gin_routes.go:1`, `internal/server/api/boundary_test.go:37`.

Domain packages own workspace, item, workstream, search, navigation, Jira, AI, Knowledge, runtime, verification, Git,
audit, and storage behavior. `internal/server/api` owns HTTP adaptation and route registration, not domain policy.

## Frontend Boundary
<!-- chunkId: platform-application-reference-frontend -->
<!-- keywords: React, feature package, shared API, route ownership -->

[live] PM-015 and PM-023 the client is organized around feature modules and page composition, with application routing and the shared API client acting as integration boundaries. Evidence: `web/src/app/router.ts:1`, `web/src/shared/api/index.ts:1`, `web/src/features/content-viewer/ContentViewer.tsx:15`.

Large surfaces may split rendering into lazy chunks, but package layout is not itself a promise that every component is
loaded independently. The production build currently reports a large main chunk, so historical performance review
targets are not treated as current service-level objectives.

## Release Artifacts
<!-- chunkId: platform-application-reference-release -->
<!-- keywords: GitHub release, darwin, linux, windows, checksum -->

[live] PM-017 tagged releases build packaged artifacts for macOS arm64 and amd64, Linux amd64, and Windows amd64, then publish checksums with the release. Evidence: `.github/workflows/release.yml:142`, `.github/workflows/release.yml:193`, `.github/workflows/release.yml:216`.

The packaged binary includes the compiled frontend. Release verification checks the lockfile's registry origin, exercises
local and cloud database paths, and uses the same repository verification lane before packaging.

## Installation And Diagnosis
<!-- chunkId: platform-application-reference-installation -->
<!-- keywords: Homebrew, formula, doctor, environment check -->

[live] PM-017 the documented local installation path uses the `kriskhoavu/homebrew-tap`, and `kode-stream doctor` checks environment and repository setup. Evidence: `README.md:155`, `README.md:165`, `cmd/kode-stream/main.go:41`.

The formula updater moves every supported release URL and checksum together, preventing a mixed-version formula. The
release helper does not replace the tagged CI release path.

## Not supported
<!-- chunkId: platform-application-reference-not-supported -->
<!-- keywords: plan CLI, automatic publish, package topology guarantee -->

[live] PM-017 the current CLI does not expose the proposed `init`, `plan create`, or `plan publish` commands. Evidence: `cmd/kode-stream/main.go:41`, `cmd/kode-stream/main.go:204`.

## Known conflicts
<!-- chunkId: platform-application-reference-known-conflicts -->
<!-- keywords: architecture refactor, performance review, flat package, local only -->

[legacy] PM-003 and PM-023 describe intermediate package topologies that were replaced by the Gin transport work and later storage composition. PM-015 is a review snapshot, not a current performance guarantee. PM-017's local-only premise predates Cloud and Agent modes.

## Related pages

See [[platform-api-transport-reference]], [[platform-storage-reference]], [[platform-cloud-runtime-reference]], and [[workspace-lifecycle-reference]].
