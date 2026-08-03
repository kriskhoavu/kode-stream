---
slug: e2e-testing-index
title: Browser Journey Index
pageType: REFERENCE
roles: TESTER, DEVELOPER
topics: e2e, browser, journeys, coverage
summary: Canonical provider-neutral browser journeys maintained by implemented feature plans.
sourceRef: wiki/e2e-testing/README.md
sourceCount: 0
---

## Execution Policy
<!-- chunkId: e2e-testing-index-policy -->
<!-- keywords: playwright, browser, runtime, evidence -->

Run journeys with Playwright MCP in a fresh context using runtime-supplied non-production fixtures. Keep credentials and
environment-specific secrets out of durable documentation.

## Pages
<!-- chunkId: e2e-testing-index-pages -->
<!-- keywords: pages, journey, canvas, branch review -->

| Journey                                     | Type   | Roles             | Sources | Summary                                                                  |
|---------------------------------------------|--------|-------------------|---------|--------------------------------------------------------------------------|
| [[platform-branch-review-checkout-journey]] | HOW_TO | TESTER, DEVELOPER | 1       | Review, import, switch checkout, and verify operational synchronization. |
| [[platform-terminal-canvas-journey]]        | HOW_TO | TESTER, DEVELOPER | 1       | Arrange Canvas nodes, launch safely, and observe verification staleness. |
