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
<!-- keywords: playwright, fresh context, fixtures, secrets -->

[open] Have these journeys passed against a supplied disposable runtime? None has been run since it was written.

Run each journey with Playwright MCP in a fresh browser context, against non-production fixtures supplied at run time. Keep credentials and environment secrets out of the wiki and the evidence. Chrome DevTools may diagnose a failed step but never runs the journey.

## Pages
<!-- chunkId: e2e-testing-index-pages -->
<!-- keywords: journey list, canvas, branch review, quality -->

- [[e2e-testing-workspace-branch-review-checkout]]: review, import and checkout.
- [[e2e-testing-workspace-terminal-canvas]]: Canvas, launch safety, verification freshness and Focus.
- [[e2e-testing-quality-coverage]]: E2E coverage, results, evidence and agent handoff.

Behavior and reference pages: [[workspace-index]].

## Not written yet
<!-- chunkId: e2e-testing-index-not-written -->
<!-- keywords: card agents, harness board, cloud coverage -->

[open] Which features still need a reusable browser journey, such as card agents or the harness board?

## Open questions
<!-- chunkId: e2e-testing-index-open-questions -->
<!-- keywords: runtime fixtures, execution evidence, validation date -->

[open] Who supplies the disposable runtime and fixtures for a scheduled run?
