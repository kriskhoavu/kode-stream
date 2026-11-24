---
slug: workspace-e2e-quality
title: E2E Quality Coverage
pageType: CONCEPT
topics: workspace, quality, browser journey, evidence, AI session
summary: E2E Quality connects plan-local browser runbooks, reusable wiki journeys, recorded results, and an agent handoff.
sourceRef: |
  PM-036 (plan retired) | Overview and design decisions
  PM-036 (plan retired) | User flows
lastVerified: 2026-09-12
---

## Coverage Model
<!-- chunkId: workspace-e2e-quality-coverage-model -->
<!-- keywords: local playbook, canonical journey, provenance, ordering -->

[live] A plan that declares browser coverage lists its own runbooks first, then the canonical wiki journeys linked to it. Knowledge opens a canonical journey with the same result and launch controls. Evidence: [[workspace-e2e-quality-reference]].

Local runbooks are working material for one plan. Journeys under `wiki/e2e-testing/` are the reusable procedures. See [[e2e-testing-quality-coverage]].

## Result And Agent Handoff
<!-- chunkId: workspace-e2e-quality-result-handoff -->
<!-- keywords: not run, browser evidence, fixed context, base url -->

[live] Each card shows a recorded result or **not run**, opens evidence in the shared preview, and hands the run to an AI session with the runbook and result path fixed. Evidence: [[workspace-e2e-quality-reference]].

The session asks for the base URL, authentication reference and test data at run time. They are never stored in the plan or wiki.

## Not supported
<!-- chunkId: workspace-e2e-quality-not-supported -->
<!-- keywords: embedded executor, durable history, secret storage, inferred pass -->

[live] Kode Stream does not run the journey itself, keep run history, store credentials or infer a pass without a valid result. Evidence: [[workspace-e2e-quality-reference]].

## Known conflicts
<!-- chunkId: workspace-e2e-quality-known-conflicts -->
<!-- keywords: markdown result, versioned result, fingerprint -->

- "A Markdown summary records a pass." Only a versioned result whose fingerprint matches the runbook counts. Anything else is unknown. Evidence: [[workspace-e2e-quality-reference]].

## Related pages

See [[workspace-e2e-quality-reference]] and [[workspace-automation-verification]].
