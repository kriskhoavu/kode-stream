---
slug: e2e-testing-quality-coverage
title: Inspect And Launch E2E Quality Coverage
pageType: HOW_TO
topics: quality, runbook, knowledge, evidence, AI session
summary: Browser journey for finding local and canonical E2E coverage, inspecting results, and opening a constrained agent handoff.
sourceRef: |
  PM-036 (plan retired) | Reusable E2E coverage
  PM-036 (plan retired) | Browser journey
  PM-036 (plan retired) | Historical result
lastVerified: never
---

## Goal And Preconditions
<!-- chunkId: e2e-testing-quality-coverage-goal -->
<!-- keywords: quality panel, disposable workspace, provider descriptor, test fixtures -->

[open] Can the current build complete this journey with an isolated Kode Stream workspace and a second workspace containing linked canonical coverage?

Use Playwright MCP in a fresh context. Supply the base URL, authentication reference when required, a disposable workspace containing an E2E-enabled plan, a second workspace with a matching canonical journey, and an enabled AI provider. Keep credentials and production data out of the runbook and evidence.

## Inspect Plan Coverage
<!-- chunkId: e2e-testing-quality-coverage-plan -->
<!-- keywords: ticket local, canonical ordering, not run, load diagnostic -->

[open] Has local-before-canonical ordering and the empty-result state been rerun against the current build?

1. Open an E2E-enabled plan and select **Quality**; wait for runtime, repository automation, and **E2E Test** sections to finish loading.
2. Assert the local scenario appears under **Plan runbooks** before any **Canonical wiki journeys** linked through source references.
3. Inspect a runbook without a valid latest result and assert it reports **Not run** or an explicit diagnostic rather than pass or failure.
4. Apply a text filter and assert only matching runbook cards remain without hiding an independent coverage diagnostic.

These steps are read-only.

## Inspect Knowledge Coverage
<!-- chunkId: e2e-testing-quality-coverage-knowledge -->
<!-- keywords: canonical page, side panel, ordinary page, result metadata -->

[open] Has Knowledge visibility and result presentation been rerun against the current build?

1. Open the linked canonical page under `wiki/e2e-testing/`; wait for the page and Quality side panel to load.
2. Assert the canonical journey card shows its current status, freshness, provider, environment, failed step, and safe evidence references when those fields exist.
3. Open a Knowledge page outside the E2E bucket and assert no E2E Quality side panel appears.
4. If safe evidence is listed, open one reference and assert the guarded shared preview renders it or reports an inline read error without removing coverage.

These steps are read-only.

## Open The Agent Handoff
<!-- chunkId: e2e-testing-quality-coverage-handoff -->
<!-- keywords: immutable prompt, result destination, required skill, blocked provider -->

[open] Has the constrained launch state been rerun for both capable and incapable providers?

1. Choose **Run E2E test** on a runbook and wait for the launch dialog and provider capabilities.
2. Assert the runbook context and result destination are fixed and the prompt requires the `e2e-testing` skill.
3. With a capable provider, assert the required skill is ready and **Open session** is enabled. Do not launch during a read-only validation run.
4. With a provider that lacks the capability, assert configuration guidance appears and launch remains disabled.

Opening the dialog is read-only. Launching an agent is a safe write only when the supplied fixture and cleanup authorize it.

## Cleanup And Evidence
<!-- chunkId: e2e-testing-quality-coverage-cleanup -->
<!-- keywords: screenshot, console health, reversible state, result record -->

[open] Has a current result been recorded with the versioned result contract?

Close dialogs and previews, return to the original workspace, and stop only sessions created for the run. Capture the two coverage groups, the not-run or diagnostic state, the Knowledge side panel, guarded evidence preview, and capable or blocked launch state. Record only safe identifiers and evidence paths.

## Coverage Deltas And Limits
<!-- chunkId: e2e-testing-quality-coverage-limits -->
<!-- keywords: agent execution, browser runtime, historical pass, framework neutrality -->

[legacy] PM-036's historical Playwright result covered ordering, launch readiness, Knowledge visibility, evidence preview, themes, and browser console health, but its Markdown result predates the current versioned evidence contract and therefore does not verify this page today.

Kode Stream presents coverage and prepares the agent handoff; it does not execute browser steps itself. See [[workspace-e2e-quality]], [[workspace-e2e-quality-reference]], and [[e2e-testing-index]].
