---
slug: workspace-e2e-quality-reference
title: E2E Quality Coverage Reference
pageType: REFERENCE
topics: runbook discovery, source reference, result parsing, evidence, AI capability
summary: Reference for E2E runbook discovery, canonical matching, result safety, and constrained AI launch.
sourceRef: |
  PM-036 (plan retired) | Contracts and safety
  PM-036 (plan retired) | Quality surface and states
  PM-036 (plan retired) | Delivery and verification
lastVerified: 2026-09-12
---

## Runbook Discovery
<!-- chunkId: workspace-e2e-quality-reference-discovery -->
<!-- keywords: plan metadata, scenario markdown, deterministic order, diagnostic -->

[live] `E2ERunbooks` accepts only working-tree plans whose metadata enables `e2e-runbook`, discovers Markdown scenario files under `automation/`, and returns a diagnostic when coverage is absent. Evidence: `internal/item/e2e_runbooks.go:23`, `internal/item/e2e_runbooks.go:39`, `internal/item/e2e_runbooks.go:43`.

[live] Canonical journeys are selected from indexed E2E pages by exact normalized `sourceRef` matches and sorted by title. Evidence: `internal/knowledge/knowledge_service.go:437`, `internal/knowledge/knowledge_service.go:451`, `internal/knowledge/knowledge_service.go:457`.

## Result Boundary
<!-- chunkId: workspace-e2e-quality-reference-result-boundary -->
<!-- keywords: versioned payload, fingerprint, byte limit, path traversal -->

[live] Latest results are bounded before parsing, must use an accepted status and a matching runbook fingerprint, and may expose only safe evidence paths. Invalid, stale, oversized, or legacy content does not become passing evidence. Evidence: `internal/e2eresult/result.go:20`, `internal/e2eresult/result.go:56`, `internal/e2eresult/result.go:80`, `internal/e2eresult/result.go:93`.

The read model returns a result destination even before the first run. Missing results appear as not run, while unsafe paths and unreadable content produce diagnostics. Evidence: `internal/item/e2e_runbooks.go:90`, `internal/item/e2e_runbooks.go:108`.

## Quality Surface And Launch
<!-- chunkId: workspace-e2e-quality-reference-surface-launch -->
<!-- keywords: plan group, wiki group, guarded preview, skill descriptor -->

[live] `E2EQualityPanel` groups plan runbooks before wiki journeys, filters cards, displays result metadata, opens evidence through the guarded workspace file API, and launches the selected runbook through `AISessionLaunchDialog`. Evidence: `web/src/features/e2e-testing/E2EQualityPanel.tsx:29`, `web/src/features/e2e-testing/E2EQualityPanel.tsx:36`, `web/src/features/e2e-testing/E2EQualityPanel.tsx:47`, `web/src/features/e2e-testing/E2EQualityPanel.tsx:103`.

[live] E2E launch fixes the prompt's runbook and result path, requires a discovered `e2e-testing` capability, and sends the resolved capability identifier. Evidence: `web/src/features/ai-session/AISessionLaunchDialog.tsx:52`, `web/src/features/ai-session/AISessionLaunchDialog.tsx:64`, `web/src/features/ai-session/AISessionLaunchDialog.tsx:170`.

Knowledge renders the Quality side panel only while reading a page under the E2E bucket. Evidence: `web/src/pages/KnowledgePage.tsx:33`, `web/src/features/e2e-testing/KnowledgeE2ESidePanel.tsx:7`.

## Not supported
<!-- chunkId: workspace-e2e-quality-reference-not-supported -->
<!-- keywords: snapshot item, background polling, local runner, credential persistence -->

[live] Snapshot-reviewed plans cannot supply operational runbooks, result refresh is explicit rather than polled, and this feature does not create a local verification job or credential store. Evidence: `internal/item/e2e_runbooks.go:29`, `web/src/features/e2e-testing/E2EQualityPanel.tsx:108`.

## Known conflicts
<!-- chunkId: workspace-e2e-quality-reference-known-conflicts -->
<!-- keywords: historical report, markdown fields, unknown evidence, current schema -->

[legacy] PM-036's retired result records colon-delimited Markdown fields, while the current parser requires a versioned structured result and matching runbook fingerprint. The historical pass is useful migration context but is not current runtime evidence. Evidence: `internal/e2eresult/result.go:56`, `internal/e2eresult/result_test.go:19`.

## Related pages

See [[workspace-e2e-quality]], [[workspace-automation-verification-reference]], and [[e2e-testing-quality-coverage]].
