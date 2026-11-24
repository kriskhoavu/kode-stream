---
slug: workspace-automation-verification
title: Automation Verification
pageType: CONCEPT
topics: workspace, automation, verification, browser, evidence
summary: Automation verification runs selected browser specifications through a workspace-owned runtime contract.
sourceRef: |
  PM-026 (plan retired) | Runtime-adaptive verification
  PM-029 (plan retired) | Runtime automation
  PM-029 (plan retired) | User experience
lastVerified: 2026-09-12
---

## Purpose
<!-- chunkId: workspace-automation-verification-purpose -->
<!-- keywords: browser specification, runtime contract, selected evidence -->

[live] A person picks discovered browser specifications and runs them through the workspace runtime. Plan prose is never executable input. Evidence: `internal/item/item_verification.go:53`, `internal/verification/automation.go:28`.

One bounded verification job shows boot, health, test, teardown, logs and retained artifacts. See [[workspace-automation-verification-reference]].

## Safety Model
<!-- chunkId: workspace-automation-verification-safety-model -->
<!-- keywords: relative path, shell quoting, artifact boundary -->

[live] Selected paths stay relative to the automation repository. Template values are validated and quoted. Artifact paths outside the configured roots are rejected. Evidence: `internal/verification/automation.go:69`.

## Not supported
<!-- chunkId: workspace-automation-verification-not-supported -->
<!-- keywords: arbitrary markdown, durable job, production credential -->

[live] Discovery from arbitrary Markdown, durable job history, and credentials in plans or artifacts. Evidence: `internal/item/item_verification.go:146`, `internal/verification/retention.go:26`.

## Known conflicts
<!-- chunkId: workspace-automation-verification-known-conflicts -->
<!-- keywords: runtime evidence, plan intent, browser execution -->

[open] Which browser journeys have been rerun against the current build? A finished plan is not runtime evidence.

## Related pages

See [[workspace-automation-verification-reference]] and [[e2e-testing-index]].
