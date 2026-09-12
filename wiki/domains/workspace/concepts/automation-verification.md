---
slug: workspace-automation-verification
title: Automation Verification
pageType: CONCEPT
topics: workspace, automation, verification, browser, evidence
summary: Automation verification runs selected browser specifications through a workspace-owned runtime contract.
sourceRef: |
  PM-029 (plan retired) | Runtime automation
  PM-029 (plan retired) | User experience
lastVerified: 2026-09-12
---

## Purpose
<!-- chunkId: workspace-automation-verification-purpose -->
<!-- keywords: browser specification, runtime contract, selected evidence -->

[live] Automation verification lets a user choose discovered browser specifications and execute them through the workspace runtime without treating plan prose as executable input. Evidence: `internal/item/item_service.go:330`, `internal/verification/verification_service.go:500`.

The result keeps boot, health, test, teardown, logs, and retained artifacts visible as one bounded verification job. Exact fields and limits live in [[workspace-automation-verification-reference]].

## Safety Model
<!-- chunkId: workspace-automation-verification-safety-model -->
<!-- keywords: relative path, shell quoting, artifact boundary, isolated run -->

[live] Selected paths must remain relative to the configured automation repository, template values are validated and quoted, and artifact traversal outside configured roots is rejected. Evidence: `internal/verification/verification_service.go:541`, `internal/verification/verification_service.go:695`.

## Not supported
<!-- chunkId: workspace-automation-verification-not-supported -->
<!-- keywords: arbitrary markdown, durable job, production credential -->

[live] Arbitrary Markdown discovery, durable job history, and embedding credentials in plans or artifacts are not supported. Evidence: `internal/item/item_service.go:417`, `internal/verification/verification_service.go:152`.

## Known conflicts
<!-- chunkId: workspace-automation-verification-known-conflicts -->
<!-- keywords: runtime evidence, plan intent, browser execution -->

[open] Which browser journeys have been rerun against the current product build? Plan completion alone is not runtime evidence.

## Related pages

See [[workspace-automation-verification-reference]], [[e2e-testing-index]], and [[workspace-index]].
