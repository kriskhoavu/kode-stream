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
<!-- keywords: plan metadata, scenario markdown, sourceRef match, diagnostic -->

[live] `E2ERunbooks` accepts only working-tree plans whose metadata enables `e2e-runbook`. It finds Markdown scenarios under `automation/` and returns a diagnostic when there is no coverage. Evidence: `internal/item/e2e_runbooks.go:23`.

[live] Canonical journeys are indexed E2E pages whose normalized `sourceRef` matches exactly, sorted by title. Evidence: `internal/knowledge/knowledge_e2e.go:14`.

## Result Boundary
<!-- chunkId: workspace-e2e-quality-reference-result-boundary -->
<!-- keywords: versioned payload, fingerprint, byte limit, path traversal -->

[live] A result is size-checked before parsing. It needs an accepted status, a matching runbook fingerprint and safe evidence paths. Invalid, stale, oversized or legacy content never counts as a pass. Evidence: `internal/e2eresult/result.go:18`, `internal/e2eresult/result.go:52`.

The read model returns the result path before the first run. A missing result shows as not run. Unsafe paths and unreadable content produce diagnostics. Evidence: `internal/item/e2e_runbooks.go:90`.

## Quality Surface And Launch
<!-- chunkId: workspace-e2e-quality-reference-surface-launch -->
<!-- keywords: plan group, wiki group, guarded preview, skill descriptor -->

[live] `E2EQualityPanel` lists plan runbooks before wiki journeys, filters cards, shows result metadata, opens evidence through the guarded file API and launches through `AISessionLaunchDialog`. Evidence: `web/src/features/e2e-testing/E2EQualityPanel.tsx:16`.

[live] An E2E launch fixes the runbook and result path in the prompt, requires a discovered `e2e-testing` capability and sends its resolved identifier. Evidence: `web/src/features/ai-session/AISessionLaunchDialog.tsx:66`.

Knowledge shows the Quality side panel only on pages under the E2E bucket. Evidence: `web/src/pages/KnowledgePage.tsx:32`.

## Not supported
<!-- chunkId: workspace-e2e-quality-reference-not-supported -->
<!-- keywords: snapshot item, background polling, local runner, credential persistence -->

[live] Reviewed snapshots supply no runbooks. Result refresh is explicit, not polled. The feature creates no verification job and no credential store. Evidence: `internal/item/e2e_runbooks.go:29`.

## Known conflicts
<!-- chunkId: workspace-e2e-quality-reference-known-conflicts -->
<!-- keywords: colon fields, markdown result, current schema -->

- "Colon-delimited Markdown fields record a result." The parser requires a versioned structured result with a matching fingerprint. Evidence: `internal/e2eresult/result_test.go:19`.

## Related pages

See [[workspace-e2e-quality]] and [[e2e-testing-quality-coverage]].
