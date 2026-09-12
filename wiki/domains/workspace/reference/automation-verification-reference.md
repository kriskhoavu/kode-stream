---
slug: workspace-automation-verification-reference
title: Automation Verification Reference
pageType: REFERENCE
topics: workspace, automation, verification, runtime, artifacts
summary: Reference for selecting, launching, and retaining browser automation verification jobs.
sourceRef: |
  PM-026 (plan retired) | Runtime, checkpoint, and frontend contracts
  PM-029 (plan retired) | Runtime automation
  PM-029 (plan retired) | Contracts and lifecycle
  PM-029 (plan retired) | User flow
  PM-029 (plan retired) | Delivery and verification
  PM-029 (plan retired) | Scenario
lastVerified: 2026-09-12
---

## Selection And Discovery
<!-- chunkId: workspace-automation-verification-reference-selection -->
<!-- keywords: selected specs, runbook discovery, plan metadata, environment -->

[live] An item stores the selected specification paths, environment, display mode, and update time; the server discovers candidate paths from plan metadata and ignores empty suggestions. Evidence: `internal/common/models/models.go:797`, `internal/item/item_service.go:417`.

The selection is distinct from the reusable browser journey itself. Plan-local `automation-test[].path` values are suggestions, while `verificationTests` records what the user chose to run. See [[e2e-testing-index]].

## Runtime Contract
<!-- chunkId: workspace-automation-verification-reference-runtime-contract -->
<!-- keywords: runtime configuration, runner, command template, artifact paths -->

[live] Workspace runtime configuration controls whether automation is enabled, the automation repository path, runner, default environment, command template, and retained artifact paths. Evidence: `internal/common/models/models.go:192`, `internal/verification/verification_service.go:500`.

The runner prepares and starts the application repository, waits for health, then runs the rendered command in the automation repository. Template values are shell-quoted before substitution. A failed start, health check, or test triggers teardown and records bounded runtime and automation logs.

## Runtime Profiles And Checkpoints
<!-- chunkId: workspace-automation-verification-reference-profiles -->
<!-- keywords: smoke, critical, full, checkpoint, freshness -->

[live] PM-026 runtime verification supports `smoke`, `critical`, and `full` profiles, while checkpoints record externally completed verification against a workspace and commit. Evidence: `internal/runtime/runtime_service.go:20`, `internal/server/api/gin_routes.go:180`.

A verification fingerprint includes runtime configuration, mode, profile, environment, display mode, and checkout state.
Canvas and other consumers can therefore distinguish a fresh result from one invalidated by configuration or source
changes. A rerun creates another bounded job through the existing verification service.

## Job And Artifact Boundaries
<!-- chunkId: workspace-automation-verification-reference-job-boundaries -->
<!-- keywords: verification job, artifact retention, log bounds, teardown -->

[live] Automation jobs use the verification service's bounded in-memory job registry, and collected artifacts are constrained by configured paths and size limits. Evidence: `internal/verification/verification_service.go:152`, `internal/verification/verification_service.go:682`.

The API exposes workspace runtime reads and updates, item selection reads and updates, and automation-mode verification job creation. Restarting the application loses job history.

## Not supported
<!-- chunkId: workspace-automation-verification-reference-not-supported -->
<!-- keywords: unsupported capability, durable history, arbitrary path -->

[live] PM-026 durable verification job history, automatic changed-service selection, and arbitrary filesystem artifact collection are not supported. Evidence: `internal/verification/verification_service.go:152`, `internal/verification/verification_service.go:695`.

## Known conflicts
<!-- chunkId: workspace-automation-verification-reference-known-conflicts -->
<!-- keywords: plan intent, runtime evidence, discovery semantics -->

[live] Candidate automation paths come from explicit metadata; the implementation does not scan arbitrary Markdown prose for runnable specs. Evidence: `internal/item/item_service.go:373`, `internal/item/item_service.go:417`.

## Related pages

See [[workspace-automation-verification]], [[workspace-terminal-canvas-reference]], and [[e2e-testing-index]].
