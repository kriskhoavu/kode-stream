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

[open] Can the current build complete this journey with one workspace holding an E2E-enabled plan and a second holding a linked canonical journey?

Supply the base URL, an authentication reference if needed, both workspaces and an enabled AI provider. Follow the [[e2e-testing-index]] execution policy.

## Inspect Plan Coverage
<!-- chunkId: e2e-testing-quality-coverage-plan -->
<!-- keywords: ticket local, canonical ordering, not run, load diagnostic -->

Read-only steps:

1. Open the E2E-enabled plan and select **Quality**. Wait for the runtime, repository automation and **E2E Test** sections to load.
2. Assert **Plan runbooks** appear before **Canonical wiki journeys**.
3. Assert a runbook without a valid result shows **Not run** or a diagnostic, never pass or fail.
4. Apply a text filter. Assert only matching cards remain and coverage diagnostics stay visible.

## Inspect Knowledge Coverage
<!-- chunkId: e2e-testing-quality-coverage-knowledge -->
<!-- keywords: canonical page, side panel, ordinary page, result metadata -->

Read-only steps:

1. Open the linked page under `wiki/e2e-testing/`. Wait for the page and the Quality side panel.
2. Assert the card shows status, freshness, provider, environment, failed step and safe evidence, where present.
3. Open a page outside the E2E bucket. Assert no Quality side panel appears.
4. Open one evidence reference. Assert the guarded preview renders it or shows an inline read error.

## Open The Agent Handoff
<!-- chunkId: e2e-testing-quality-coverage-handoff -->
<!-- keywords: fixed prompt, result destination, required skill, blocked provider -->

1. Choose **Run E2E test** on a runbook. Wait for the launch dialog and provider capabilities.
2. Assert the runbook and result path are fixed and the prompt requires the `e2e-testing` skill.
3. With a capable provider, assert the skill is ready and **Open session** is enabled. Do not launch in a read-only run.
4. With an incapable provider, assert configuration guidance appears and launch stays disabled.

Launching an agent is a safe write only when the fixture and its cleanup allow it.

## Cleanup And Evidence
<!-- chunkId: e2e-testing-quality-coverage-cleanup -->
<!-- keywords: screenshot, console health, reversible state, result record -->

Close dialogs and previews, return to the original workspace and stop only sessions this run created. Capture both coverage groups, the not-run state, the Knowledge side panel, the evidence preview and both launch states. Record only safe identifiers and paths, in the versioned result format.

Kode Stream shows coverage and prepares the handoff. It never runs browser steps. See [[workspace-e2e-quality]].
