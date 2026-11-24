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
<!-- keywords: selected specs, plan metadata, environment, suggestions -->

[live] An item stores the selected specification paths, environment, display mode and update time. The server finds candidate paths in plan metadata and ignores empty ones. Evidence: `internal/common/models/workstream.go:49`, `internal/item/item_verification.go:146`.

`automation-test[].path` values in a plan are suggestions. `verificationTests` records what the person chose to run.

## Runtime Contract
<!-- chunkId: workspace-automation-verification-reference-runtime-contract -->
<!-- keywords: runner, command template, artifact paths, teardown -->

[live] The workspace runtime configuration sets: automation on or off, the automation repository path, runner, default environment, command template and retained artifact paths. Evidence: `internal/common/models/workspace.go:87`.

The runner starts the application repository, waits for health, then runs the rendered command in the automation repository. Template values are shell-quoted. A failed start, health check or test triggers teardown and keeps bounded logs.

## Runtime Profiles And Checkpoints
<!-- chunkId: workspace-automation-verification-reference-profiles -->
<!-- keywords: smoke, critical, full, fingerprint -->

[live] Runtime verification has `smoke`, `critical` and `full` profiles. A checkpoint records verification finished elsewhere against a workspace and commit. Evidence: `internal/runtime/runtime_service.go:20`.

The fingerprint covers runtime configuration, mode, profile, environment, display mode and checkout state. A rerun creates a new bounded job.

## Job And Artifact Boundaries
<!-- chunkId: workspace-automation-verification-reference-job-boundaries -->
<!-- keywords: in-memory registry, artifact retention, size limits -->

[live] Jobs live in the verification service's bounded in-memory registry, so a restart loses them. Artifacts are limited to configured paths and sizes. Evidence: `internal/verification/retention.go:26`, `internal/verification/artifacts.go:101`.

## Not supported
<!-- chunkId: workspace-automation-verification-reference-not-supported -->
<!-- keywords: durable history, changed-service selection, arbitrary path -->

[live] Durable job history, automatic selection of changed services, and artifact collection from arbitrary paths. Evidence: `internal/verification/artifacts.go:118`.

## Known conflicts
<!-- chunkId: workspace-automation-verification-reference-known-conflicts -->
<!-- keywords: prose scanning, discovery semantics, runnable specs -->

- "Kode Stream finds runnable specs in plan prose." It reads explicit metadata only. Evidence: `internal/item/item_verification.go:146`.

## Related pages

See [[workspace-automation-verification]] and [[workspace-terminal-canvas-reference]].
