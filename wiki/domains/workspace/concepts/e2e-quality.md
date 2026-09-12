---
slug: workspace-e2e-quality
title: E2E Quality Coverage
pageType: CONCEPT
topics: workspace, quality, browser journey, evidence, AI session
summary: E2E Quality connects plan-local browser runbooks, reusable wiki journeys, recorded results, and an agent handoff.
sourceRef: |
  plans/platform/PM-036/README.md | Overview and design decisions
  plans/platform/PM-036/scenario/scenario-00-overview.md | User flows
lastVerified: 2026-09-12
---

## Coverage Model
<!-- chunkId: workspace-e2e-quality-coverage-model -->
<!-- keywords: local playbook, canonical journey, provenance, ordering -->

[live] A plan that declares reusable browser coverage shows its ticket-local runbooks before canonical wiki journeys linked to the same source. A canonical journey can also be opened from Knowledge with the same result and launch controls. Evidence: [[workspace-e2e-quality-reference]].

Local runbooks are working material for one plan. Canonical journeys under `wiki/e2e-testing/` are the durable, reusable browser procedures. See [[e2e-testing-quality-coverage]].

## Result And Agent Handoff
<!-- chunkId: workspace-e2e-quality-result-handoff -->
<!-- keywords: not run, browser evidence, immutable context, provider capability -->

[live] Each card reports a recorded result or an explicit not-run state, opens safe evidence through the shared preview, and hands execution to an AI session with the chosen runbook and result destination fixed. Evidence: [[workspace-e2e-quality-reference]].

Runtime inputs such as the base URL, authentication reference, and isolated test data are requested when the journey runs. They are not stored in the plan or wiki page.

## Not supported
<!-- chunkId: workspace-e2e-quality-not-supported -->
<!-- keywords: embedded executor, durable history, secret storage, inferred pass -->

[live] Kode Stream does not execute the browser journey itself, persist a run history, store runtime credentials, or infer a passing result when no valid result exists. Evidence: [[workspace-e2e-quality-reference]].

## Known conflicts
<!-- chunkId: workspace-e2e-quality-known-conflicts -->
<!-- keywords: repository automation, verification job, markdown result, freshness -->

[legacy] PM-036's recorded result used an older Markdown summary. The current result reader accepts a bounded, versioned result whose fingerprint matches the runbook; other content is treated as unknown rather than proof of a pass. Evidence: [[workspace-e2e-quality-reference]].

## Related pages

See [[workspace-e2e-quality-reference]], [[workspace-automation-verification]], and [[e2e-testing-index]].
