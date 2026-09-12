---
slug: e2e-testing-index
title: Browser Journey Index
pageType: REFERENCE
topics: e2e, browser, journeys, coverage
summary: Canonical provider-neutral browser journeys maintained by implemented feature plans.
sourceRef: |
  wiki/e2e-testing/README.md
lastVerified: never
---

## Execution Policy
<!-- chunkId: e2e-testing-index-policy -->
<!-- keywords: playwright, browser, runtime, evidence -->

[open] Does the current implementation still satisfy the execution policy behavior described below? This section has not yet been reverified.

Run journeys with Playwright MCP in a fresh context using runtime-supplied non-production fixtures. Keep credentials and
environment-specific secrets out of durable documentation.

## Pages
<!-- chunkId: e2e-testing-index-pages -->
<!-- keywords: pages, journey, canvas, branch review -->

[open] Does the current implementation still satisfy the pages behavior described below? This section has not yet been reverified.

- [[e2e-testing-workspace-branch-review-checkout]] validates review and checkout.
- [[e2e-testing-workspace-terminal-canvas]] validates the spatial workbench.
- [[e2e-testing-quality-coverage]] validates E2E coverage discovery, results, evidence, and agent handoff.
- [[workspace-index]] holds the behavior and reference counterparts.

## Not written yet
<!-- chunkId: e2e-testing-index-not-written -->
<!-- keywords: session focus, navigator interaction, cloud coverage -->

[open] Which additional completed plans supply reusable browser journeys?

## Open questions
<!-- chunkId: e2e-testing-index-open-questions -->
<!-- keywords: runtime fixtures, execution evidence, validation date -->

[open] Have these journeys passed against a supplied disposable runtime? This migration does not execute browser tests.
